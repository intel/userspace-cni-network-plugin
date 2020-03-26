package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/plugins/pkg/testutils"
	"github.com/intel/userspace-cni-network-plugin/pkg/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const verString = "userspace-cni-network-plugin version:%s, commit:%s, date:%s"

func getTestPod() *v1.Pod {
	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "testName",
			Namespace: "testNamespace",
		},
	}
}

func getTestArgs() *skel.CmdArgs {
	return &skel.CmdArgs{
		ContainerID: "12345678901234567890",
		Netns:       "testNet",
		IfName:      "eth0",
		Args:        "",
	}
}

func TestPrintVersionString(t *testing.T) {
	t.Run("verify version string", func(t *testing.T) {
		exp := fmt.Sprintf(verString, version, commit, date)
		out := printVersionString()
		if out != exp {
			t.Error("Version string mismatch")
		}
	})
}

func TestLoadNetConf(t *testing.T) {
	testCases := []struct {
		name       string
		netConfStr string
		expNetConf *types.NetConf
		expError   string
	}{
		{
			name:       "fail to parse netConf",
			netConfStr: "{",
			expNetConf: nil,
			expError:   "failed to load netconf:",
		},
		{
			name:       "fail to set default logging level",
			netConfStr: `{"LogLevel": "nologsatall"}`,
			expNetConf: &types.NetConf{LogLevel: "nologsatall"},
			expError:   "Userspace-CNI logging: cannot set logging level to nologsatall",
		},
		{
			name:       "fail to set log file",
			netConfStr: `{"LogFile": "/proc/cant_log_here.log"}`,
			expNetConf: &types.NetConf{LogFile: "/proc/cant_log_here.log"},
			expError:   "Userspace-CNI logging: cannot open ",
		},
		{
			name:       "load correct netConf",
			netConfStr: `{"kubeconfig":"/etc/kube.conf","sharedDir":"/tmp/tmp_shareddir","host":{"engine":"ovs-dpdk","iftype":"vhostuser","netType":"bridge"}}`,
			expNetConf: &types.NetConf{KubeConfig: "/etc/kube.conf", SharedDir: "/tmp/tmp_shareddir", HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", NetType: "bridge"}},
			expError:   "",
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

			if tc.expError != "" {
				// one of err or stdError has to match to expError, while the 2nd one is set to nil
				if err == nil && stdError == "" {
					t.Errorf("Error was expected but not observed. Expected: '%v'", tc.expError)
				} else if err == nil && !strings.HasPrefix(stdError, tc.expError) {
					t.Errorf("Unexpected stderror. Expected prefix: '%v', observed: '%v'", tc.expError, stdError)
				} else if stdError == "" && !strings.HasPrefix(err.Error(), tc.expError) {
					t.Errorf("Unexpected error. Expected prefix: '%v', observed: '%v'", tc.expError, err)
				} else if err != nil && stdError != "" {
					t.Errorf("Unexpected errors. Expected prefix: '%v', observed: error: '%v' stderror: '%v'", tc.expError, err, stdError)
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			} else if stdError != "" {
				t.Errorf("Unexpected stderror: %v", stdError)
			}

			// compare retrieved NetConf struct with expected result
			if !reflect.DeepEqual(tc.expNetConf, netConf) {
				t.Errorf("Unexpected parsing output. Expected netConf: '%v', observed: '%v'", tc.expNetConf, netConf)
			}
		})
	}
}

func TestGetPodAndSharedDir(t *testing.T) {
	args := getTestArgs()
	testPod := getTestPod()
	testPodWithVolume := getTestPod()

	tmpVolume := v1.Volume{
		Name: "shared-dir",
		VolumeSource: v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: "/tmp/testdir/",
			},
		},
	}
	testPodWithVolume.Spec.Volumes = append(testPodWithVolume.Spec.Volumes, tmpVolume)

	testCases := []struct {
		name         string
		pod          *v1.Pod
		netConf      *types.NetConf
		expSharedDir string
	}{
		{
			name:         "default sharedDir",
			pod:          testPod,
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
			pod:          testPod,
			netConf:      &types.NetConf{HostConf: types.UserSpaceConf{Engine: "vpp"}},
			expSharedDir: fmt.Sprintf("/var/run/vpp/%v/", args.ContainerID[:12]),
		},
		{
			name:         "default sharedDir for ovs-dpdk",
			pod:          testPod,
			netConf:      &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk"}},
			expSharedDir: fmt.Sprintf("/usr/local/var/run/openvswitch/%v/", args.ContainerID[:12]),
		},
		{
			name:         "default sharedDir for ovs-dpdk with kubeconfig",
			pod:          testPod,
			netConf:      &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk"}, KubeConfig: "/etc/kube.conf"},
			expSharedDir: fmt.Sprintf("/usr/local/var/run/openvswitch/%v/", args.ContainerID[:12]),
		},
		{
			name:         "configured sharedDir in netConf with trailing slash",
			pod:          testPod,
			netConf:      &types.NetConf{SharedDir: "/tmp/netconfdir/"},
			expSharedDir: fmt.Sprintf("/tmp/netconfdir/%v/", args.ContainerID[:12]),
		},
		{
			name:         "configured sharedDir in netConf with NO trailing slash",
			pod:          testPod,
			netConf:      &types.NetConf{SharedDir: "/tmp/netconfdir"},
			expSharedDir: fmt.Sprintf("/tmp/netconfdir/%v/", args.ContainerID[:12]),
		},
		{
			name:         "configured sharedDir in Pod.Spec.Volumes",
			pod:          testPodWithVolume,
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
			}

			_, _, sharedDir, err := getPodAndSharedDir(tc.netConf, args, kubeClient)
			if err != nil {
				t.Errorf("getPodAndSharedDir error: %v", err)
			}
			if sharedDir != tc.expSharedDir {
				t.Errorf("sharedDir error; shareDir: %v, expSharedDir: %v", sharedDir, tc.expSharedDir)
			}
		})
	}
}

func TestCmdAdd(t *testing.T) {
	testArgs := getTestArgs()
	testPod := getTestPod()
	testNetNS, nsErr := testutils.NewNS()
	if nsErr != nil {
		t.Fatal("Can't create NewNS")
	}

	testCases := []struct {
		name       string
		pod        *v1.Pod
		netConfStr string
		netNS      string
		expError   string
	}{
		{
			name:       "fail to parse netConf",
			pod:        testPod,
			netConfStr: "{",
			netNS:      "",
			expError:   "failed to load netconf:",
		},
		{
			name:       "fail to open netns",
			pod:        testPod,
			netConfStr: `{"host":{"engine":"ovs-dpdk"}}`,
			netNS:      "badNS",
			expError:   "failed to open netns",
		},
		{
			name:       "fail to connect to vpp",
			pod:        testPod,
			netConfStr: `{"host":{"engine":"vpp"}}`,
			netNS:      testNetNS.Path(),
			expError:   "dial unix /run/vpp-api.sock: connect: no such file or directory",
		},
		{
			name:       "fail to connect to ovs-dpdk",
			pod:        testPod,
			netConfStr: `{"host":{"engine":"ovs-dpdk"}}`,
			netNS:      testNetNS.Path(),
			expError:   `exec: "ovs-vsctl":`,
		},
		{
			name:       "fail with unknown engine",
			pod:        testPod,
			netConfStr: `{"host":{"engine":"nonsence"}}`,
			netNS:      testNetNS.Path(),
			expError:   "ERROR: Unknown Host Engine:nonsence",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var kubeClient *fake.Clientset
			var exec invoke.Exec

			testArgs.Netns = tc.netNS
			kubeClient = fake.NewSimpleClientset(tc.pod)

			testArgs.StdinData = []byte(tc.netConfStr)
			err := cmdAdd(testArgs, exec, kubeClient)
			if tc.expError != "" {
				if err == nil {
					t.Errorf("Error was expected but not observed. Expected: '%v'", tc.expError)
				} else if !strings.HasPrefix(err.Error(), tc.expError) {
					t.Errorf("Unexpected error. Expected: '%v', observed: '%v'", tc.expError, err)
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestCmdGet(t *testing.T) {
	t.Run("Phony test placeholder until GetCmd will be implemented", func(t *testing.T) {
		var exec invoke.Exec
		testArgs := getTestArgs()
		kubeClient := fake.NewSimpleClientset()

		err := cmdGet(testArgs, exec, kubeClient)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})
}

func TestCmdDel(t *testing.T) {
	testArgs := getTestArgs()
	testPod := getTestPod()

	testCases := []struct {
		name       string
		pod        *v1.Pod
		netConfStr string
		expError   string
	}{
		{
			name:       "fail to parse netConf",
			pod:        testPod,
			netConfStr: "{",
			expError:   "failed to load netconf:",
		},
		{
			name:       "fail to connect to vpp",
			pod:        testPod,
			netConfStr: `{"host":{"engine":"vpp"}}`,
			expError:   "dial unix /run/vpp-api.sock: connect: no such file or directory",
		},
		{
			name:       "fail to connect to ovs-dpdk",
			pod:        testPod,
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser"}}`,
			expError:   `exec: "ovs-vsctl":`,
		},
		{
			name:       "fail with unknown engine",
			pod:        testPod,
			netConfStr: `{"host":{"engine":"nonsence"}}`,
			expError:   "ERROR: Unknown Host Engine:nonsence",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var kubeClient *fake.Clientset
			var exec invoke.Exec

			kubeClient = fake.NewSimpleClientset(tc.pod)

			testArgs.StdinData = []byte(tc.netConfStr)
			err := cmdDel(testArgs, exec, kubeClient)
			if tc.expError != "" {
				if err == nil {
					t.Errorf("Error was expected but not observed. Expected: '%v'", tc.expError)
				} else if !strings.HasPrefix(err.Error(), tc.expError) {
					t.Errorf("Unexpected error. Expected: '%v', observed: '%v'", tc.expError, err)
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
