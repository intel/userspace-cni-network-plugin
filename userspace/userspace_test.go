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

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/plugins/pkg/testutils"
	"github.com/intel/userspace-cni-network-plugin/cniovs"
	"github.com/intel/userspace-cni-network-plugin/pkg/types"
	"github.com/intel/userspace-cni-network-plugin/userspace/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const verString = "userspace-cni-network-plugin version:%s, commit:%s, date:%s"

func TestPrintVersionString(t *testing.T) {
	t.Run("verify version string", func(t *testing.T) {
		exp := fmt.Sprintf(verString, version, commit, date)
		out := printVersionString()
		assert.Equal(t, exp, out, "Version string mismatch")
	})
}

func TestLoadNetConf(t *testing.T) {
	testCases := []struct {
		name       string
		netConfStr string
		expNetConf *types.NetConf
		expErr     error
		expStdErr  string
	}{
		{
			name:       "fail to parse netConf 1",
			netConfStr: "{",
			expNetConf: nil,
			expErr:     errors.New("failed to load netconf:"),
		},
		{
			name:       "fail to parse netConf 2",
			netConfStr: `{"host"}`,
			expNetConf: nil,
			expErr:     errors.New("failed to load netconf:"),
		},
		{
			name:       "fail to parse netConf 3",
			netConfStr: `{"host":}`,
			expNetConf: nil,
			expErr:     errors.New("failed to load netconf:"),
		},
		{
			name:       "fail to parse netConf 4",
			netConfStr: `{"host",}`,
			expNetConf: nil,
			expErr:     errors.New("failed to load netconf:"),
		},
		{
			name:       "fail to parse netConf 5",
			netConfStr: `{"host",{"engine":"ovs-dpdk"}`,
			expNetConf: nil,
			expErr:     errors.New("failed to load netconf:"),
		},
		{
			name:       "fail to parse netConf 6",
			netConfStr: `{"host":{"engine":"ovs-dpdk","container":{"engine":"ovs-dpdk"}}`,
			expNetConf: nil,
			expErr:     errors.New("failed to load netconf:"),
		},
		{
			name:       "fail to set default logging level",
			netConfStr: `{"LogLevel": "nologsatall"}`,
			expNetConf: &types.NetConf{LogLevel: "nologsatall"},
			expStdErr:  "Userspace-CNI logging: cannot set logging level to nologsatall",
		},
		{
			name:       "fail to set log file",
			netConfStr: `{"LogFile": "/proc/cant_log_here.log"}`,
			expNetConf: &types.NetConf{LogFile: "/proc/cant_log_here.log"},
			expStdErr:  "Userspace-CNI logging: cannot open ",
		},
		{
			name:       "load correct netConf",
			netConfStr: `{"kubeconfig":"/etc/kube.conf","sharedDir":"/tmp/tmp_shareddir","host":{"engine":"ovs-dpdk","iftype":"vhostuser","netType":"bridge"}}`,
			expNetConf: &types.NetConf{KubeConfig: "/etc/kube.conf", SharedDir: "/tmp/tmp_shareddir", HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", NetType: "bridge"}},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var netConf *types.NetConf

			// capture stderror messages from loadNetConf
			stdR, stdW, stdErr := os.Pipe()
			if stdErr != nil {
				t.Fatal("Can't capture stderr")
			}
			origStdErr := os.Stderr
			os.Stderr = stdW
			netConf, err := loadNetConf([]byte(tc.netConfStr))
			os.Stderr = origStdErr
			stdW.Close()
			var buf bytes.Buffer
			io.Copy(&buf, stdR)
			stdError := buf.String()

			if tc.expStdErr == "" {
				assert.Equal(t, tc.expStdErr, stdError, "Unexpected error at stderr")
			} else {
				assert.Contains(t, stdError, tc.expStdErr, "Unexpected error at stderr")
			}
			if err == nil {
				assert.Equal(t, tc.expErr, err, "Error was expected")
			} else {
				require.Error(t, tc.expErr, "Unexpected error returned")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected error returned")
			}
			assert.Equal(t, tc.expNetConf, netConf, "Unexpected parsing output.")
		})
	}
}

func TestGetPodAndSharedDir(t *testing.T) {
	args := testdata.GetTestArgs()
	pod := testdata.GetTestPod("")
	podWithVolume := testdata.GetTestPod("/tmp/testdir/")

	testCases := []struct {
		name         string
		pod          *v1.Pod
		netConf      *types.NetConf
		expSharedDir string
	}{
		{
			name:         "default sharedDir",
			pod:          pod,
			netConf:      &types.NetConf{},
			expSharedDir: fmt.Sprintf("/var/lib/cni/usrspcni/%v/", args.ContainerID[:12]),
		},
		{
			name:         "default sharedDir when Pod not found",
			pod:          nil,
			netConf:      &types.NetConf{},
			expSharedDir: fmt.Sprintf("/var/lib/cni/usrspcni/%v/", args.ContainerID[:12]),
		},
		{
			name:         "default sharedDir for vpp",
			pod:          pod,
			netConf:      &types.NetConf{HostConf: types.UserSpaceConf{Engine: "vpp"}},
			expSharedDir: fmt.Sprintf("/var/run/vpp/%v/", args.ContainerID[:12]),
		},
		{
			name:         "default sharedDir for ovs-dpdk",
			pod:          pod,
			netConf:      &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk"}},
			expSharedDir: fmt.Sprintf("/usr/local/var/run/openvswitch/%v/", args.ContainerID[:12]),
		},
		{
			name:         "default sharedDir for ovs-dpdk with kubeconfig",
			pod:          pod,
			netConf:      &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk"}, KubeConfig: "/etc/kube.conf"},
			expSharedDir: fmt.Sprintf("/usr/local/var/run/openvswitch/%v/", args.ContainerID[:12]),
		},
		{
			name:         "configured sharedDir in netConf with trailing slash",
			pod:          pod,
			netConf:      &types.NetConf{SharedDir: "/tmp/netconfdir/"},
			expSharedDir: fmt.Sprintf("/tmp/netconfdir/%v/", args.ContainerID[:12]),
		},
		{
			name:         "configured sharedDir in netConf with NO trailing slash",
			pod:          pod,
			netConf:      &types.NetConf{SharedDir: "/tmp/netconfdir"},
			expSharedDir: fmt.Sprintf("/tmp/netconfdir/%v/", args.ContainerID[:12]),
		},
		{
			name:         "configured sharedDir in Pod.Spec.Volumes",
			pod:          podWithVolume,
			netConf:      &types.NetConf{},
			expSharedDir: "/tmp/testdir/",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var kubeClient *fake.Clientset

			if tc.pod == nil {
				kubeClient = fake.NewSimpleClientset()
			} else {
				kubeClient = fake.NewSimpleClientset(tc.pod)
				args.Args = fmt.Sprintf("K8S_POD_NAME=%s;K8S_POD_NAMESPACE=%s", tc.pod.Name, tc.pod.Namespace)
			}

			resClient, resPod, sharedDir, err := getPodAndSharedDir(tc.netConf, args, kubeClient)
			assert.NoError(t, err, "Unexpected error")
			assert.Equal(t, tc.expSharedDir, sharedDir, "Unexpected sharedDir returned")
			assert.Equal(t, tc.pod, resPod, "Unexpected pod returned")
			assert.Equal(t, kubeClient, resClient, "Unexpected kube client returned")
		})
	}
}

func TestCmdAdd(t *testing.T) {
	testCases := []struct {
		name       string
		netConfStr string
		netNS      string
		expError   string
		expJSONKey string // a mandatory key in valid JSON output
		fakeExec   bool
		fakeErr    error
	}{
		{
			name:       "fail to parse netConf",
			netConfStr: "{",
			netNS:      "",
			expError:   "failed to load netconf:",
		},
		{
			name:       "fail to open netns",
			netConfStr: `{"host":{"engine":"ovs-dpdk"},"sharedDir":"#sharedDir#"}`,
			netNS:      "badNS",
			expError:   "failed to open netns",
		},
		{
			name:       "fail to connect to vpp",
			netConfStr: `{"host":{"engine":"vpp"},"sharedDir":"#sharedDir#"}`,
			netNS:      "generate",
			expError:   "VPP API socket file /run/vpp/api.sock does not exist",
		},
		{
			name:       "fail to connect to ovs-dpdk",
			netConfStr: `{"host":{"engine":"ovs-dpdk"},"sharedDir":"#sharedDir#"}`,
			netNS:      "generate",
			expError:   "ovs exec error",
			fakeExec:   true,
			fakeErr:    errors.New("ovs exec error"),
		},
		{
			name:       "fail with unknown engine",
			netConfStr: `{"host":{"engine":"nonsense"},"sharedDir":"#sharedDir#"}`,
			netNS:      "generate",
			expError:   "ERROR: Unknown Host Engine:nonsense",
			fakeExec:   true,
		},
		{
			name:       "host set and no IPAM",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser","vhost":{"mode":"client"}},"sharedDir":"#sharedDir#"}`,
			netNS:      "generate",
			expJSONKey: "cniVersion",
			fakeExec:   true,
		},
		{
			// currently host and container engine can differ - does it make sense?
			name:       "container with vpp engine",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser","vhost":{"mode":"client"}},"container":{"engine":"vpp","iftype":"vhostuser"},"sharedDir":"#sharedDir#"}`,
			netNS:      "generate",
			expJSONKey: "cniVersion",
			fakeExec:   true,
		},
		{
			name:       "fail container with unknown engine",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser","vhost":{"mode":"client"}},"container":{"engine":"nonsense","iftype":"vhostuser"},"sharedDir":"#sharedDir#"}`,
			netNS:      "generate",
			fakeExec:   true,
			expError:   "ERROR: Unknown Container Engine:nonsense",
		},
		{
			name:       "container set and no IPAM",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser","vhost":{"mode":"client"}},"container":{"engine":"ovs-dpdk","iftype":"vhostuser"},"sharedDir":"#sharedDir#"}`,
			netNS:      "generate",
			expJSONKey: "cniVersion",
			fakeExec:   true,
		},
		{
			name:       "fail when CNI command is not set",
			netConfStr: `{"ipam":{"type":"host-local"},"host":{"engine":"ovs-dpdk","iftype":"vhostuser","vhost":{"mode":"client"}},"sharedDir":"#sharedDir#"}`,
			netNS:      "generate",
			fakeExec:   true,
			expError:   "no paths provided",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var kubeClient *fake.Clientset
			var exec invoke.Exec
			args := testdata.GetTestArgs()

			if tc.netNS == "generate" {
				netNS, nsErr := testutils.NewNS()
				require.NoError(t, nsErr, "Can't create NewNS")
				defer testutils.UnmountNS(netNS)
				args.Netns = netNS.Path()
			} else {
				args.Netns = tc.netNS
			}

			sharedDir, dirErr := ioutil.TempDir("/tmp", "test-userspace-")
			require.NoError(t, dirErr, "Can't create temporary directory")
			tc.netConfStr = strings.Replace(tc.netConfStr, "#sharedDir#", sharedDir, -1)
			defer os.RemoveAll(sharedDir)

			pod := testdata.GetTestPod(sharedDir)
			kubeClient = fake.NewSimpleClientset(pod)

			if pod != nil {
				args.Args = fmt.Sprintf("K8S_POD_NAME=%s;K8S_POD_NAMESPACE=%s", pod.Name, pod.Namespace)
			}

			args.StdinData = []byte(tc.netConfStr)

			if tc.fakeExec {
				cniovs.SetExecCommand(&cniovs.FakeExecCommand{Err: tc.fakeErr})
				defer cniovs.SetDefaultExecCommand()
			}

			// capture JSON printed to stdout on cmdAdd() success
			stdR, stdW, stdErr := os.Pipe()
			if stdErr != nil {
				t.Fatal("Can't capture stderr")
			}
			origStdout := os.Stdout
			os.Stdout = stdW
			err := cmdAdd(args, exec, kubeClient)
			os.Stdout = origStdout
			stdW.Close()
			var buf bytes.Buffer
			io.Copy(&buf, stdR)
			stdOut := buf.String()

			if tc.expError == "" {
				assert.NoError(t, err, "Unexpected error")
			} else {
				require.Error(t, err, "Unexpected error")
				assert.Contains(t, err.Error(), tc.expError, "Unexpected error")
			}

			// validate captured JSON output
			if tc.expJSONKey == "" {
				assert.Empty(t, stdOut, "Unexpected output")
			} else {
				var jsonOut interface{}
				require.NoError(t, json.Unmarshal([]byte(stdOut), &jsonOut), "Invalid JSON in output")
				assert.Contains(t, jsonOut, tc.expJSONKey)
			}

			// remove termporary files by reading saved data
			var data cniovs.OvsSavedData
			assert.NoError(t, cniovs.LoadConfig(&types.NetConf{}, args, &data))
		})
	}
}

func TestCmdGet(t *testing.T) {
	t.Run("test placeholder until GetCmd will be implemented", func(t *testing.T) {
		var exec invoke.Exec
		args := testdata.GetTestArgs()
		kubeClient := fake.NewSimpleClientset()
		assert.NoError(t, cmdGet(args, exec, kubeClient), "Unexpected error")
	})
}

func TestCmdDel(t *testing.T) {
	testCases := []struct {
		name       string
		netConfStr string
		expError   string
		fakeExec   bool
	}{
		{
			name:       "fail to parse netConf",
			netConfStr: `{"host":{"engine":"vpp"}`,
			expError:   "failed to load netconf:",
		},
		{
			name:       "fail to connect to vpp",
			netConfStr: `{"host":{"engine":"vpp"},"sharedDir":"#sharedDir#"}`,
			expError:   "VPP API socket file /run/vpp/api.sock does not exist",
		},
		{
			name:       "fail to connect to ovs-dpdk",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser"},"sharedDir":"#sharedDir#"}`,
			expError:   `exec: "ovs-vsctl":`,
		},
		{
			name:       "fail with unknown host engine",
			netConfStr: `{"host":{"engine":"nonsense"},"sharedDir":"#sharedDir#"}`,
			expError:   "ERROR: Unknown Host Engine:nonsense",
		},
		{
			name:       "container fail with unknown engine",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser"},"container":{"engine":"nonsense","iftype":"vhostuser"},"sharedDir":"#sharedDir#"}`,
			fakeExec:   true,
			expError:   "ERROR: Unknown Container Engine:nonsense",
		},
		{
			name:       "host set and no IPAM",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser"},"sharedDir":"#sharedDir#"}`,
			fakeExec:   true,
		},
		{
			name:       "host and netNS set and no IPAM",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser"},"sharedDir":"#sharedDir#"}`,
			fakeExec:   true,
		},
		{
			name:       "container set and no IPAM",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser"},"container":{"engine":"ovs-dpdk","iftype":"vhostuser"},"sharedDir":"#sharedDir#"}`,
			fakeExec:   true,
		},
		{
			name:       "fail ipam call",
			netConfStr: `{"ipam":{"type":"host-local"},"host":{"engine":"ovs-dpdk","iftype":"vhostuser"},"container":{"engine":"ovs-dpdk","iftype":"vhostuser"},"sharedDir":"#sharedDir#"}`,
			expError:   "no paths provided",
			fakeExec:   true,
		},
		{
			// currently host and container engine can differ - does it make sense?
			name:       "container with vpp engine",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser"},"container":{"engine":"vpp","iftype":"vhostuser"},"sharedDir":"#sharedDir#"}`,
			fakeExec:   true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var kubeClient *fake.Clientset
			var exec invoke.Exec

			args := testdata.GetTestArgs()

			netNS, nsErr := testutils.NewNS()
			require.NoError(t, nsErr, "Can't create NewNS")
			defer testutils.UnmountNS(netNS)

			sharedDir, dirErr := ioutil.TempDir("/tmp", "test-userspace-")
			require.NoError(t, dirErr, "Can't create temporary directory")
			testDir := filepath.Join(sharedDir, args.ContainerID[:12])
			require.NoError(t, os.MkdirAll(testDir, os.ModePerm), "Can't create shared directory")
			tc.netConfStr = strings.Replace(tc.netConfStr, "#sharedDir#", sharedDir, -1)
			defer os.RemoveAll(sharedDir)

			pod := testdata.GetTestPod(sharedDir)
			kubeClient = fake.NewSimpleClientset(pod)
			args.Netns = netNS.Path()
			args.StdinData = []byte(tc.netConfStr)
			if tc.fakeExec {
				cniovs.SetExecCommand(&cniovs.FakeExecCommand{})
				defer cniovs.SetDefaultExecCommand()
			}

			err := cmdDel(args, exec, kubeClient)

			if tc.expError == "" {
				assert.NoError(t, err, "Unexpected error")
			} else {
				require.Error(t, err, "Unexpected error")
				assert.Contains(t, err.Error(), tc.expError, "Unexpected error")
			}
		})
	}
}
