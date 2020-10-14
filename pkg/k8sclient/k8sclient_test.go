// Copyright 2020 Intel Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sclient

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"testing"

	"github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/intel/userspace-cni-network-plugin/userspace/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetK8sArgs(t *testing.T) {
	testCases := []struct {
		name    string
		args    *skel.CmdArgs
		expArgs *k8sArgs
		expErr  error
	}{
		{
			name:    "args set to empty string",
			args:    &skel.CmdArgs{Args: ""},
			expArgs: &k8sArgs{},
		},
		{
			name:    "args set correctly",
			args:    &skel.CmdArgs{Args: "IgnoreUnknown=true;IP=127.0.0.1;K8S_POD_NAME=testpod;K8S_POD_NAMESPACE=testspace;K8S_POD_INFRA_CONTAINER_ID=0"},
			expArgs: &k8sArgs{CommonArgs: cnitypes.CommonArgs{IgnoreUnknown: true}, IP: net.IPv4(127, 0, 0, 1), K8S_POD_NAME: "testpod", K8S_POD_NAMESPACE: "testspace", K8S_POD_INFRA_CONTAINER_ID: "0"},
		},
		{
			name:    "ingnore unknown arg",
			args:    &skel.CmdArgs{Args: "IgnoreUnknown=true;s0mEArG=anyValue"},
			expArgs: &k8sArgs{CommonArgs: cnitypes.CommonArgs{IgnoreUnknown: true}},
		},
		{
			name:   "fail with invalid IP",
			args:   &skel.CmdArgs{Args: "IP=512.0.0.1;K8S_POD_NAME=testpod"},
			expErr: errors.New("ARGS: error parsing"),
		},
		{
			name:   "fail with unknown arg",
			args:   &skel.CmdArgs{Args: "s0mEArG=anyValue"},
			expErr: errors.New("ARGS: unknown args"),
		},
		{
			name:   "fail with unknown arg 2",
			args:   &skel.CmdArgs{Args: "IgnoreUnknown=false;s0mEArG=anyValue"},
			expErr: errors.New("ARGS: unknown args"),
		},
		{
			name:   "fail with CmdArgs set to nil",
			args:   nil,
			expErr: errors.New("getK8sArgs: failed to get k8s"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args, err := getK8sArgs(tc.args)

			if tc.expErr != nil {
				require.Error(t, err, "Error was expected")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected error")
			} else {
				require.NoError(t, err, "Unexpected error")
				assert.Equal(t, tc.expArgs, args, "Unexpected result")
			}

		})
	}
}

func TestGetK8sClient(t *testing.T) {
	testCases := []struct {
		name      string
		testType  string
		expClient kubernetes.Interface
		expErr    error
	}{
		{
			name:     "kubeClient set",
			testType: "client_set",
		},
		{
			name:      "kubeConfig set to nil",
			testType:  "config_empty",
			expClient: nil,
		},
		{
			name:     "kubeConfig set to wrong file",
			testType: "config_invalid",
			expErr:   errors.New("getK8sClient: failed to get context for the kubeConfig"),
		},
		{
			name:     "kube env variables set",
			testType: "environment_set",
			expErr:   errors.New("createK8sClient: failed to get context for in-cluster"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var resClient, kubeClient kubernetes.Interface
			var kubeConfig string
			var err error

			switch tc.testType {
			case "client_set":
				kubeClient = fake.NewSimpleClientset()
				tc.expClient = kubeClient
			case "config_empty":
				kubeConfig = ""
			case "config_invalid":
				kubeConfig = "/proc/kubeconfig.yaml"
			case "environment_set":
				for _, param := range []string{"KUBERNETES_SERVICE_HOST", "KUBERNETES_SERVICE_PORT"} {
					value, found := os.LookupEnv(param)
					if found {
						defer os.Setenv(param, value)
					} else {
						defer os.Unsetenv(param)
					}
					os.Setenv(param, param+"-test-value")
				}
			}

			resClient, err = getK8sClient(kubeClient, kubeConfig)

			if tc.expErr != nil {
				require.Error(t, err, "Error was expected")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected error")
			} else {
				require.NoError(t, err, "Unexpected error")
				assert.Equal(t, tc.expClient, resClient, "Unexpected result")
			}

		})
	}
}

func TestGetPod(t *testing.T) {
	testCases := []struct {
		name     string
		testType string
		expErr   error
	}{
		{
			name: "get pod",
		},
		{
			name:     "fail to get pod with nil CmdArgs",
			testType: "args_nil",
			expErr:   errors.New("getK8sArgs: failed to get k8s args for CmdArgs set to <nil>"),
		},
		{
			name:     "fail to get pod with nil kubeClient",
			testType: "client_nil",
			expErr:   errors.New("GetPod: No kubeClient: <nil>"),
		},
		{
			name:     "fail to get pod with bad CmdArgs",
			testType: "args_invalid",
			expErr:   errors.New("ARGS: error parsing"),
		},
		{
			name:     "fail to get pod with bad kubeConfig",
			testType: "config_invalid",
			expErr:   errors.New("getK8sClient: failed to get context for the kubeConfig"),
		},
		{
			name:     "fail to get pod with empty kubeConfig",
			testType: "config_empty",
			expErr:   errors.New("GetPod: No kubeClient:"),
		},
		{
			name:     "fail to get pod from empty pod set",
			testType: "pods_empty",
			expErr:   errors.New("pods \"\" not found"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var kubeClient kubernetes.Interface
			var args *skel.CmdArgs
			var kubeConfig string
			var pod *v1.Pod

			switch tc.testType {
			case "args_invalid":
				args = &skel.CmdArgs{Args: "IP=512.0.0.1;K8S_POD_NAME=testpod"}
			case "args_nil":
				args = nil
			case "client_nil":
				args = testdata.GetTestArgs()
				kubeClient = nil
			case "config_invalid":
				args = testdata.GetTestArgs()
				kubeConfig = "/proc/kubeconfig.yaml"
			case "config_empty":
				args = testdata.GetTestArgs()
				kubeConfig = ""
			case "pods_empty":
				args = testdata.GetTestArgs()
				kubeClient = fake.NewSimpleClientset()
			default:
				sharedDir, dirErr := ioutil.TempDir("/tmp", "test-k8sclient-")
				require.NoError(t, dirErr, "Can't create temporary directory")
				defer os.RemoveAll(sharedDir)

				pod = testdata.GetTestPod(sharedDir)
				args = &skel.CmdArgs{Args: fmt.Sprintf("K8S_POD_NAME=%s;K8S_POD_NAMESPACE=%s", pod.Name, pod.Namespace)}
				kubeClient = fake.NewSimpleClientset(pod)
			}

			resPod, resClient, err := GetPod(args, kubeClient, kubeConfig)

			if tc.expErr != nil {
				require.Error(t, err, "Error was expected")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected error")
			} else {
				require.NoError(t, err, "Unexpected error")
				require.Equal(t, kubeClient, resClient, "Unexpected result")
				require.Equal(t, pod, resPod, "Unexpected result")
			}

		})
	}
}

func TestWritePodAnnotation(t *testing.T) {
	testCases := []struct {
		name     string
		testType string
		expErr   error
	}{
		{
			name: "write annotations",
		},
		{
			name:     "fail with pod set to nil",
			testType: "pod_nil",
			expErr:   errors.New("WritePodAnnotation: No pod:"),
		},
		{
			name:     "fail with kubeClient set to nil",
			testType: "client_nil",
			expErr:   errors.New("WritePodAnnotation: No kubeClient:"),
		},
		{
			name:     "fail to get pod",
			testType: "pod_not_found",
			expErr:   errors.New("status update failed for pod"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var kubeClient kubernetes.Interface
			var pod *v1.Pod

			sharedDir, dirErr := ioutil.TempDir("/tmp", "test-k8sclient-")
			require.NoError(t, dirErr, "Can't create temporary directory")
			defer os.RemoveAll(sharedDir)

			switch tc.testType {
			case "client_nil":
				kubeClient = nil
			case "pod_not_found":
				pod = testdata.GetTestPod(sharedDir)
				kubeClient = fake.NewSimpleClientset()
			case "pod_nil":
				pod = nil
				kubeClient = fake.NewSimpleClientset()
			default:
				pod = testdata.GetTestPod(sharedDir)
				kubeClient = fake.NewSimpleClientset(pod)
			}

			origPod := pod.DeepCopy()
			resPod, err := WritePodAnnotation(kubeClient, pod)

			if tc.expErr != nil {
				require.Error(t, err, "Error was expected")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected error")
			} else {
				require.NoError(t, err, "Unexpected error")
				require.Equal(t, origPod, resPod, "Unexpected result")
			}

		})
	}
}
