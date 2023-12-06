package cnivpp

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/containernetworking/cni/pkg/skel"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/google/uuid"
	"github.com/intel/userspace-cni-network-plugin/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

func GetTestPod(sharedDir string) *v1.Pod {
	id, _ := uuid.NewUUID()
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			UID:       apitypes.UID(id.String()),
			Name:      fmt.Sprintf("pod-%v", id[:8]),
			Namespace: fmt.Sprintf("namespace-%v", id[:8]),
		},
	}
	if sharedDir != "" {
		pod.Spec.Volumes = append(pod.Spec.Volumes,
			v1.Volume{
				Name: "shared-dir",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: sharedDir,
					},
				},
			})
		pod.Spec.Containers = append(pod.Spec.Containers,
			v1.Container{
				Name:         "container",
				VolumeMounts: []v1.VolumeMount{{Name: "shared-dir", MountPath: sharedDir}},
			})
	}
	return pod
}

func GetTestArgs() *skel.CmdArgs {
	id, _ := uuid.NewUUID()
	return &skel.CmdArgs{
		ContainerID: id.String(),
		IfName:      fmt.Sprintf("eth%v", int(id[7])),
		StdinData:   []byte("{}"),
	}
}

func TestGetMemifSocketfileName(t *testing.T) {
	t.Run("get Memif Socker File Name", func(t *testing.T) {
		args := GetTestArgs()

		sharedDir, dirErr := os.MkdirTemp("/tmp", "test-cniovs-")
		require.NoError(t, dirErr, "Can't create temporary directory")
		defer os.RemoveAll(sharedDir)

		memifSockFileName := getMemifSocketfileName(&types.NetConf{}, sharedDir, args.ContainerID, args.IfName)
		assert.Equal(t, filepath.Join(sharedDir, fmt.Sprintf("memif-%s-%s.sock", args.ContainerID[:12], args.IfName)), memifSockFileName, "Unexpected error")

		conf := &types.NetConf{}
		conf.HostConf.MemifConf.Socketfile = "socketFile.sock"

		memifSockFileName = getMemifSocketfileName(conf, sharedDir, args.ContainerID, args.IfName)
		assert.Equal(t, filepath.Join(sharedDir, conf.HostConf.MemifConf.Socketfile), memifSockFileName, "Unexpected error")
	})
}

func TestAddOnContainer(t *testing.T) {
	t.Run("save container data to file", func(t *testing.T) {
		var result *current.Result
		args := GetTestArgs()
		cniVpp := CniVpp{}

		sharedDir, dirErr := os.MkdirTemp("/tmp", "test-cniovs-")
		require.NoError(t, dirErr, "Can't create temporary directory")
		defer os.RemoveAll(sharedDir)

		pod := GetTestPod(sharedDir)
		resPod, resErr := cniVpp.AddOnContainer(&types.NetConf{}, args, nil, sharedDir, pod, result)
		assert.NoError(t, resErr, "Unexpected error")
		assert.Equal(t, pod, resPod, "Unexpected change of pod data")
		fileName := fmt.Sprintf("configData-%s-%s.json", args.ContainerID[:12], args.IfName)
		assert.FileExists(t, path.Join(sharedDir, fileName), "Container data were not saved to file")
	})
}

func TestDelOnContainer(t *testing.T) {
	t.Run("remove container configuration", func(t *testing.T) {
		args := GetTestArgs()
		cniVpp := CniVpp{}

		sharedDir, dirErr := os.MkdirTemp("/tmp", "test-cniovs-")
		require.NoError(t, dirErr, "Can't create temporary directory")
		// just in case DelFromContainer fails
		defer os.RemoveAll(sharedDir)

		err := cniVpp.DelFromContainer(&types.NetConf{}, args, sharedDir, nil)
		assert.NoError(t, err, "Unexpected error")
		assert.NoDirExists(t, sharedDir, "Container data were not removed")
	})
}

func TestAddOnHost(t *testing.T) {
	cniVpp := CniVpp{}

	testCases := []struct {
		name    string
		netConf *types.NetConf
		fakeErr error
		expErr  error
	}{
		{
			name: "Happy path",
			netConf: &types.NetConf{
				HostConf: types.UserSpaceConf{Engine: "vpp", IfType: "memif", NetType: "interface",
					VhostConf: types.VhostConf{Mode: "client"},
					MemifConf: types.MemifConf{
						Role: "master", // Role of memif: master|slave
						Mode: "ip",     // Mode of memif: ip|ethernet|inject-punt
					}}},
			expErr: nil,
		},
		{
			name: "Invalid MEMIF Role",
			netConf: &types.NetConf{
				HostConf: types.UserSpaceConf{Engine: "vpp", IfType: "memif", NetType: "interface",
					VhostConf: types.VhostConf{Mode: "client"},
					MemifConf: types.MemifConf{
						Role: "",   // Role of memif: master|slave
						Mode: "ip", // Mode of memif: ip|ethernet|inject-punt
					}}},
			expErr: errors.New("ERROR: Invalid MEMIF Role"),
		},
		{
			name: "Unknown IfType",
			netConf: &types.NetConf{
				HostConf: types.UserSpaceConf{Engine: "vpp", IfType: "", NetType: "interface",
					VhostConf: types.VhostConf{Mode: "client"},
					MemifConf: types.MemifConf{
						Role: "",   // Role of memif: master|slave
						Mode: "ip", // Mode of memif: ip|ethernet|inject-punt
					}}},
			expErr: errors.New("Unknown HostConf.IfType"),
		},
		{
			name: "Unknown NetType",
			netConf: &types.NetConf{
				HostConf: types.UserSpaceConf{Engine: "vpp", IfType: "memif", NetType: "UnkownNetType",
					VhostConf: types.VhostConf{Mode: "client"},
					MemifConf: types.MemifConf{
						Role: "master", // Role of memif: master|slave
						Mode: "ip",     // Mode of memif: ip|ethernet|inject-punt
					}}},
			expErr: errors.New("Unknown HostConf.NetType"),
		},
		{
			name: "Bridge already exists",
			netConf: &types.NetConf{
				HostConf: types.UserSpaceConf{Engine: "vpp", IfType: "memif", NetType: "bridge",
					VhostConf: types.VhostConf{Mode: "client"},
					MemifConf: types.MemifConf{
						Role: "master", // Role of memif: master|slave
						Mode: "ip",     // Mode of memif: ip|ethernet|inject-punt
					}}},
			expErr: errors.New("Bridge domain already exists"),
		},
		{
			name: "Create 12345 Bridge",
			netConf: &types.NetConf{
				HostConf: types.UserSpaceConf{Engine: "vpp", IfType: "memif", NetType: "bridge",
					BridgeConf: types.BridgeConf{
						BridgeName: "12345",
						BridgeId:   12345,
					},
					VhostConf: types.VhostConf{Mode: "client"},
					MemifConf: types.MemifConf{
						Role: "master", // Role of memif: master|slave
						Mode: "ip",     // Mode of memif: ip|ethernet|inject-punt
					}}},
			expErr: nil,
		},
		{
			name: "NetType set to empty",
			netConf: &types.NetConf{
				HostConf: types.UserSpaceConf{Engine: "vpp", IfType: "memif", NetType: "",
					VhostConf: types.VhostConf{Mode: "client"},
					MemifConf: types.MemifConf{
						Role: "master", // Role of memif: master|slave
						Mode: "ip",     // Mode of memif: ip|ethernet|inject-punt
					}}},
			expErr: errors.New("ERROR: NetType must be provided"),
		},
		{
			name: "interface slave and ip mode",
			netConf: &types.NetConf{
				HostConf: types.UserSpaceConf{Engine: "vpp", IfType: "memif", NetType: "interface",
					VhostConf: types.VhostConf{Mode: "client"},
					MemifConf: types.MemifConf{
						Role: "slave", // Role of memif: master|slave
						Mode: "ip",    // Mode of memif: ip|ethernet|inject-punt
					}}},
			expErr: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result *current.Result
			args := GetTestArgs()

			sharedDir, dirErr := os.MkdirTemp("/tmp", "test-cnivpp-")
			require.NoError(t, dirErr, "Can't create temporary directory")
			defer os.RemoveAll(sharedDir)

			pod := GetTestPod(sharedDir)
			kubeClient := fake.NewSimpleClientset(pod)

			err := cniVpp.AddOnHost(tc.netConf, args, kubeClient, sharedDir, result)
			if tc.expErr == nil {
				assert.Equal(t, tc.expErr, err, "Unexpected result")
				// on success there shall be saved ovs data
				var data VppSavedData
				assert.NoError(t, LoadVppConfig(tc.netConf, args, &data))
				assert.NotEmpty(t, data.MemifSocketId)
			} else {
				require.Error(t, err, "Unexpected result")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected result")
			}
		})
	}
}

func TestDelFromHost(t *testing.T) {
	cniVpp := CniVpp{}

	testCases := []struct {
		name      string
		netConf   *types.NetConf
		savedData string
		fakeErr   error
		expErr    error
	}{
		{
			name: "Happy path",
			netConf: &types.NetConf{
				HostConf: types.UserSpaceConf{Engine: "vpp", IfType: "memif", NetType: "interface",
					VhostConf: types.VhostConf{Mode: "client"},
					MemifConf: types.MemifConf{
						Role: "master", // Role of memif: master|slave
						Mode: "ip",     // Mode of memif: ip|ethernet|inject-punt
					}}},
			expErr: nil,
		},
		{
			name: "Unknown HostConf Type",
			netConf: &types.NetConf{
				HostConf: types.UserSpaceConf{Engine: "vpp", IfType: "Unknown", NetType: "interface",
					VhostConf: types.VhostConf{Mode: "client"},
					MemifConf: types.MemifConf{
						Role: "master", // Role of memif: master|slave
						Mode: "ip",     // Mode of memif: ip|ethernet|inject-punt
					}}},
			expErr: fmt.Errorf("ERROR: Unknown HostConf.Type"),
		},
		{
			name: "Delete Bridge with IfType set to vhostUser",
			netConf: &types.NetConf{
				HostConf: types.UserSpaceConf{Engine: "vpp", IfType: "vhostuser", NetType: "bridge",
					VhostConf: types.VhostConf{Mode: "client"},
					BridgeConf: types.BridgeConf{
						BridgeName: "12345",
						BridgeId:   12345,
					},
					MemifConf: types.MemifConf{
						Role: "master", // Role of memif: master|slave
						Mode: "ip",     // Mode of memif: ip|ethernet|inject-punt
					}}},
			expErr: fmt.Errorf("GOOD: Found HostConf.Type:vhostuser"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := GetTestArgs()
			sharedDir, dirErr := os.MkdirTemp("/tmp", "test-cnivpp-")
			require.NoError(t, dirErr, "Can't create temporary directory")
			defer os.RemoveAll(sharedDir)

			var result *current.Result

			pod := GetTestPod(sharedDir)
			kubeClient := fake.NewSimpleClientset(pod)

			_ = cniVpp.AddOnHost(tc.netConf, args, kubeClient, sharedDir, result)

			err := cniVpp.DelFromHost(tc.netConf, args, sharedDir)
			if tc.expErr == nil {
				assert.Equal(t, tc.expErr, err, "Unexpected result")
			} else {
				require.Error(t, err, "Unexpected result")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected result")
			}
		})
	}
}
