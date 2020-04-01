package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"testing"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/plugins/pkg/testutils"
	"github.com/intel/userspace-cni-network-plugin/cniovs"
	"github.com/intel/userspace-cni-network-plugin/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				require.NotNil(t, tc.expErr, "Unexpected error returned")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected error returned")
			}
			assert.Equal(t, tc.expNetConf, netConf, "Unexpected parsing output.")
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
			assert.Nil(t, err, "Unexpected error")
			assert.Equal(t, tc.expSharedDir, sharedDir, "Unexpected sharedDir returned")
		})
	}
}

func TestCmdAdd(t *testing.T) {
	testArgs := getTestArgs()
	testPod := getTestPod()
	testNetNS, nsErr := testutils.NewNS()
	require.Nil(t, nsErr, "Can't craete NewNS")

	testCases := []struct {
		name       string
		netConfStr string
		netNS      string
		expError   string
		expJSONKey string // a mandatory key in valid JSON output
		fakeExec   bool
	}{
		{
			name:       "fail to parse netConf",
			netConfStr: "{",
			netNS:      "",
			expError:   "failed to load netconf:",
		},
		{
			name:       "fail to open netns",
			netConfStr: `{"host":{"engine":"ovs-dpdk"}}`,
			netNS:      "badNS",
			expError:   "failed to open netns",
		},
		{
			name:       "fail to connect to vpp",
			netConfStr: `{"host":{"engine":"vpp"}}`,
			netNS:      testNetNS.Path(),
			expError:   "dial unix /run/vpp-api.sock: connect: no such file or directory",
		},
		{
			name:       "fail to connect to ovs-dpdk",
			netConfStr: `{"host":{"engine":"ovs-dpdk"}}`,
			netNS:      testNetNS.Path(),
			expError:   `exec: "ovs-vsctl":`,
		},
		{
			name:       "fail with unknown engine",
			netConfStr: `{"host":{"engine":"nonsence"}}`,
			netNS:      testNetNS.Path(),
			expError:   "ERROR: Unknown Host Engine:nonsence",
		},
		{
			name:       "host set and no IPAM",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser"}}`,
			netNS:      testNetNS.Path(),
			expJSONKey: "cniVersion",
			fakeExec:   true,
		},
		{
			// currently host and container engine can differ - does it make sense?
			name:       "container with vpp engine",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser"},"container":{"engine":"vpp","iftype":"vhostuser"}}`,
			netNS:      testNetNS.Path(),
			expJSONKey: "cniVersion",
			fakeExec:   true,
		},
		{
			name:       "fail container with unknown engine",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser"},"container":{"engine":"nonsence","iftype":"vhostuser"}}`,
			netNS:      testNetNS.Path(),
			fakeExec:   true,
			expError:   "ERROR: Unknown Container Engine:nonsence",
		},
		{
			name:       "container set and no IPAM",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser"},"container":{"engine":"ovs-dpdk","iftype":"vhostuser"}}`,
			netNS:      testNetNS.Path(),
			expJSONKey: "cniVersion",
			fakeExec:   true,
		},
		{
			name:       "fail when CNI command is not set",
			netConfStr: `{"ipam":{"type":"host-local"},"host":{"engine":"ovs-dpdk","iftype":"vhostuser"}}`,
			netNS:      testNetNS.Path(),
			fakeExec:   true,
			expError:   "CNI_COMMAND is not",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var kubeClient *fake.Clientset
			var exec invoke.Exec

			testArgs.Netns = tc.netNS
			kubeClient = fake.NewSimpleClientset(testPod)

			testArgs.StdinData = []byte(tc.netConfStr)

			if tc.fakeExec {
				cniovs.SetExecCommand(&cniovs.FakeExecCommand{})
				defer cniovs.SetDefaultExecCommand()
			}

			// capture JSON printed to stdout on cmdAdd() success
			stdR, stdW, stdErr := os.Pipe()
			if stdErr != nil {
				t.Fatal("Can't capture stderr")
			}
			origStdout := os.Stdout
			os.Stdout = stdW
			err := cmdAdd(testArgs, exec, kubeClient)
			os.Stdout = origStdout
			stdW.Close()
			var buf bytes.Buffer
			io.Copy(&buf, stdR)
			stdOut := buf.String()

			if tc.expError == "" {
				assert.Nil(t, err, "Unexpected error")
			} else {
				require.NotNil(t, err, "Unexpected error")
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
		})
	}
}

func TestCmdGet(t *testing.T) {
	t.Run("test placeholder until GetCmd will be implemented", func(t *testing.T) {
		var exec invoke.Exec
		testArgs := getTestArgs()
		kubeClient := fake.NewSimpleClientset()
		assert.NoError(t, cmdGet(testArgs, exec, kubeClient), "Unexpected error")
	})
}

func TestCmdDel(t *testing.T) {
	testArgs := getTestArgs()
	testPod := getTestPod()

	testNetNS, nsErr := testutils.NewNS()
	require.Nil(t, nsErr, "Can't craete NewNS")

	testCases := []struct {
		name       string
		netConfStr string
		netNS      string
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
			netConfStr: `{"host":{"engine":"vpp"}}`,
			expError:   "dial unix /run/vpp-api.sock: connect: no such file or directory",
		},
		{
			name:       "fail to connect to ovs-dpdk",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser"}}`,
			expError:   `exec: "ovs-vsctl":`,
		},
		{
			name:       "fail with unknown host engine",
			netConfStr: `{"host":{"engine":"nonsence"}}`,
			expError:   "ERROR: Unknown Host Engine:nonsence",
		},
		{
			name:       "container fail with unknown engine",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser"},"container":{"engine":"nonsence","iftype":"vhostuser"}}`,
			fakeExec:   true,
			expError:   "ERROR: Unknown Container Engine:nonsence",
		},
		{
			name:       "host set and no IPAM",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser"},"sharedDir":"/tmp/tmp_shareddir"}`,
			fakeExec:   true,
		},
		{
			name:       "host and netNS set and no IPAM",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser"},"sharedDir":"/tmp/tmp_shareddir"}`,
			netNS:      testNetNS.Path(),
			fakeExec:   true,
		},
		{
			name:       "container set and no IPAM",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser"},"container":{"engine":"ovs-dpdk","iftype":"vhostuser"}}`,
			fakeExec:   true,
		},
		{
			name:       "fail ipam call",
			netConfStr: `{"ipam":{"type":"host-local"},"host":{"engine":"ovs-dpdk","iftype":"vhostuser"},"container":{"engine":"ovs-dpdk","iftype":"vhostuser"}}`,
			expError:   "environment variable CNI_COMMAND must be specified",
			fakeExec:   true,
		},
		{
			// currently host and container engine can differ - does it make sense?
			name:       "container with vpp engine",
			netConfStr: `{"host":{"engine":"ovs-dpdk","iftype":"vhostuser"},"container":{"engine":"vpp","iftype":"vhostuser"}}`,
			fakeExec:   true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var kubeClient *fake.Clientset
			var exec invoke.Exec

			kubeClient = fake.NewSimpleClientset(testPod)
			testArgs.Netns = tc.netNS

			netConf, netErr := loadNetConf([]byte(tc.netConfStr))
			if netErr == nil {
				_, _, sharedDir, sharedDirErr := getPodAndSharedDir(netConf, testArgs, kubeClient)
				if sharedDirErr == nil && sharedDir != "" {
					dir := path.Join(sharedDir, testArgs.ContainerID[:12])
					require.Nil(t, os.MkdirAll(dir, os.ModePerm), "Can't create shared directory")
					defer os.RemoveAll(dir)
				}
			}
			testArgs.StdinData = []byte(tc.netConfStr)
			if tc.fakeExec {
				cniovs.SetExecCommand(&cniovs.FakeExecCommand{})
				defer cniovs.SetDefaultExecCommand()
			}
			err := cmdDel(testArgs, exec, kubeClient)

			if tc.expError == "" {
				assert.Nil(t, err, "Unexpected error")
			} else {
				require.NotNil(t, err, "Unexpected error")
				assert.Contains(t, err.Error(), tc.expError, "Unexpected error")
			}
		})
	}
}
