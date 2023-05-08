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

package cniovs

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"regexp"
	"strings"
	"syscall"
	"testing"

	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/intel/userspace-cni-network-plugin/pkg/annotations"
	"github.com/intel/userspace-cni-network-plugin/pkg/types"
	"github.com/intel/userspace-cni-network-plugin/userspace/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes/fake"
)

func TestAddOnHost(t *testing.T) {
	ovs := CniOvs{}

	testCases := []struct {
		name    string
		netConf *types.NetConf
		fakeErr error
		expErr  error
	}{
		{
			name:    "fail to create bridge",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", NetType: "bridge", VhostConf: types.VhostConf{Mode: "client"}}},
			fakeErr: errors.New("Can't create bridge"),
			expErr:  errors.New("Can't create bridge"),
		},
		{
			name:    "fail due to missing IfType",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", NetType: "bridge"}},
			expErr:  errors.New("ERROR: Unknown HostConf.IfType:"),
		},
		{
			name:    "fail due to wrong IfType",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "badIfType", NetType: "bridge"}},
			expErr:  errors.New("ERROR: Unknown HostConf.IfType:"),
		},
		{
			name:    "fail due to NetType set to interface",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", NetType: "interface", VhostConf: types.VhostConf{Mode: "client"}}},
			expErr:  errors.New("ERROR: HostConf.NetType"),
		},
		{
			name:    "fail due to NetType set to wrong value",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", NetType: "badNetType", VhostConf: types.VhostConf{Mode: "client"}}},
			expErr:  errors.New("ERROR: Unknown HostConf.NetType:"),
		},
		{
			name:    "configure host bridge and store ovs data",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", NetType: "bridge", VhostConf: types.VhostConf{Mode: "client"}}},
			expErr:  nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result *current.Result
			args := testdata.GetTestArgs()

			sharedDir, dirErr := ioutil.TempDir("/tmp", "test-cniovs-")
			require.NoError(t, dirErr, "Can't create temporary directory")
			defer os.RemoveAll(sharedDir)

			pod := testdata.GetTestPod(sharedDir)
			kubeClient := fake.NewSimpleClientset(pod)

			SetExecCommand(&FakeExecCommand{Err: tc.fakeErr})
			err := ovs.AddOnHost(tc.netConf, args, kubeClient, sharedDir, result)
			SetDefaultExecCommand()
			if tc.expErr == nil {
				assert.Equal(t, tc.expErr, err, "Unexpected result")
				// on success there shall be saved ovs data
				var data OvsSavedData
				assert.NoError(t, LoadConfig(tc.netConf, args, &data))
				assert.NotEmpty(t, data.Vhostname)
			} else {
				require.Error(t, err, "Unexpected result")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected result")
			}
		})
	}
}

func TestAddOnContainer(t *testing.T) {
	t.Run("save container data to file", func(t *testing.T) {
		var result *current.Result
		args := testdata.GetTestArgs()
		ovs := CniOvs{}

		sharedDir, dirErr := ioutil.TempDir("/tmp", "test-cniovs-")
		require.NoError(t, dirErr, "Can't create temporary directory")
		defer os.RemoveAll(sharedDir)

		pod := testdata.GetTestPod(sharedDir)
		resPod, resErr := ovs.AddOnContainer(&types.NetConf{}, args, nil, sharedDir, pod, result)
		assert.NoError(t, resErr, "Unexpected error")
		assert.Equal(t, pod, resPod, "Unexpected change of pod data")
		fileName := fmt.Sprintf("configData-%s-%s.json", args.ContainerID[:12], args.IfName)
		assert.FileExists(t, path.Join(sharedDir, fileName), "Container data were not saved to file")
	})
}

func TestDelOnContainer(t *testing.T) {
	t.Run("remove container configuration", func(t *testing.T) {
		args := testdata.GetTestArgs()
		ovs := CniOvs{}

		sharedDir, dirErr := ioutil.TempDir("/tmp", "test-cniovs-")
		require.NoError(t, dirErr, "Can't create temporary directory")
		// just in case that DelFromContainer fails
		defer os.RemoveAll(sharedDir)

		err := ovs.DelFromContainer(&types.NetConf{}, args, sharedDir, nil)
		assert.NoError(t, err, "Unexpected error")
		assert.NoDirExists(t, sharedDir, "Container data were not removed")
	})
}

func TestDelFromHost(t *testing.T) {
	ovs := CniOvs{}

	testCases := []struct {
		name      string
		netConf   *types.NetConf
		savedData string
		fakeErr   error
		expErr    error
	}{
		{
			name:      "fail to load saved data",
			netConf:   &types.NetConf{},
			savedData: "{",
			expErr:    errors.New("ERROR: Failed to parse"),
		},
		{
			name:      "fail to delete interface with unknown type",
			netConf:   &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "unknownType"}},
			savedData: "{}",
			expErr:    errors.New("ERROR: Unknown HostConf.Type:"),
		},
		{
			name:      "fail to delete interface with vhostuser type",
			netConf:   &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser"}},
			savedData: "{}",
			fakeErr:   errors.New("exec error"),
			expErr:    errors.New("exec error"),
		},
		{
			name:      "delete interface",
			netConf:   &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser"}},
			savedData: "{}",
			expErr:    nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := testdata.GetTestArgs()
			execCommand := &FakeExecCommand{Err: tc.fakeErr}

			localDir := annotations.DefaultLocalCNIDir
			fileName := fmt.Sprintf("local-%s-%s.json", args.ContainerID[:12], args.IfName)
			if _, err := os.Stat(localDir); err != nil {
				require.NoError(t, os.MkdirAll(localDir, 0700), "Can't create data dir")
				defer os.RemoveAll(localDir)
			}
			path := path.Join(localDir, fileName)

			require.NoError(t, ioutil.WriteFile(path, []byte(tc.savedData), 0644), "Can't create test file")
			defer os.Remove(path)

			sharedDir, dirErr := ioutil.TempDir("/tmp", "test-cniovs-")
			require.NoError(t, dirErr, "Can't create temporary directory")
			defer os.RemoveAll(sharedDir)

			SetExecCommand(execCommand)
			err := ovs.DelFromHost(tc.netConf, args, sharedDir)
			SetDefaultExecCommand()

			if tc.expErr == nil {
				assert.Equal(t, tc.expErr, err, "Unexpected result")
				assert.NoDirExists(t, sharedDir, "Shared directory was not removed")
			} else {
				require.Error(t, err, "Unexpected result")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected result")
			}
		})
	}
}

func TestGenerateRandomMacAddress(t *testing.T) {
	nr := 10
	t.Run(fmt.Sprintf("generate %v random MAC addresses", nr), func(t *testing.T) {
		macs := make(map[string]int)
		for i := 0; i < nr; i++ {
			mac := generateRandomMacAddress()
			ok, err := regexp.Match("^[0-9a-fA-F]{2}(:([0-9a-fA-F]){2}){5}$", []byte(mac))
			require.NoError(t, err, fmt.Sprintf("MAC is incorrect. MAC: %q", mac))
			require.True(t, ok, fmt.Sprintf("MAC is incorrect. MAC: %q", mac))
			macs[mac]++
		}
		assert.Equal(t, nr, len(macs), fmt.Sprintf("MACs are not unique. MACs: %v", macs))
	})
}

func TestGetShortSharedDir(t *testing.T) {
	testCases := []struct {
		name      string
		sharedDir string
		expDir    string
	}{
		{
			name:      "return shared dir",
			sharedDir: "shared-dir",
			expDir:    "shared-dir",
		},
		{
			name:      "return shared dir with path",
			sharedDir: "/tmp/var/log/shared-dir",
			expDir:    "/tmp/var/log/shared-dir",
		},
		{
			name:      "return shared dir with length 102",
			sharedDir: "/var/lib/kubelet/pods/#UUID#/volumes/kubernetes.io/backup-dir/shared-dir",
			expDir:    "/var/lib/kubelet/pods/#UUID#/volumes/kubernetes.io/backup-dir/shared-dir",
		},
		{
			name:      "return shared dir with empty_dir and length 102",
			sharedDir: "/var/lib/kubelet/pods/#UUID#/volumes/kubernetes.io/empty_dir/shared-dir",
			expDir:    "/var/lib/kubelet/pods/#UUID#/volumes/kubernetes.io/empty_dir/shared-dir",
		},
		{
			name:      "return shared dir with empty-dir and length 88",
			sharedDir: "/var/lib/kubelet/pods/#UUID#/volumes/kubernetes.~empty-dir",
			expDir:    "/var/lib/kubelet/pods/#UUID#/volumes/kubernetes.~empty-dir",
		},
		{
			name:      "shorten shared dir with empty-dir and length 89",
			sharedDir: "/var/lib/kubelet/pods/#UUID#/volumes/kubernetes.i~empty-dir",
			expDir:    "/var/lib/vhost_sockets/#UUID#",
		},
		{
			name:      "shorten shared dir with empty-dir and length 101",
			sharedDir: "/var/lib/kubelet/pods/#UUID#/volumes/kubernetes.io~empty-dir/shared-dir",
			expDir:    "/var/lib/vhost_sockets/#UUID#",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			id := string(uuid.NewUUID())
			tc.sharedDir = strings.Replace(tc.sharedDir, "#UUID#", id, -1)
			tc.expDir = strings.Replace(tc.expDir, "#UUID#", id, -1)
			shortDir := getShortSharedDir(tc.sharedDir)
			assert.Equal(t, tc.expDir, shortDir, "Unexpected result")
		})
	}
}

func TestCreateSharedDir(t *testing.T) {
	testCases := []struct {
		name         string
		sharedDir    string
		oldSharedDir string
		expErr       error
	}{
		{
			name:         "shared dir exists",
			sharedDir:    "#sharedDir#",
			oldSharedDir: "#sharedDir#",
			expErr:       nil,
		},
		{
			name:         "fail to create shared dir",
			sharedDir:    "/proc/broken-shared-dir",
			oldSharedDir: "/proc/broken-shared-dir",
			expErr:       errors.New("mkdir "),
		},
		{
			name:         "shared dir in socket dir",
			sharedDir:    "/var/lib/vhost_sockets/#sharedDirNoPath#",
			oldSharedDir: "#sharedDir#",
			expErr:       nil,
		},
		{
			name:         "fail to mount old shared dir to socket dir",
			sharedDir:    "/var/lib/vhost_sockets/#sharedDirNoPath#",
			oldSharedDir: "/proc/broken-shared-dir",
			expErr:       errors.New("no such file"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sharedDir, dirErr := ioutil.TempDir("/tmp", "test-cniovs-")
			sharedDirNoPath := strings.Split(sharedDir, "/")[2]
			require.NoError(t, dirErr, "Can't create temporary directory")
			tc.sharedDir = strings.Replace(tc.sharedDir, "#sharedDir#", sharedDir, -1)
			tc.sharedDir = strings.Replace(tc.sharedDir, "#sharedDirNoPath#", sharedDirNoPath, -1)
			tc.oldSharedDir = strings.Replace(tc.oldSharedDir, "#sharedDir#", sharedDir, -1)
			defer os.RemoveAll(sharedDir)

			err := createSharedDir(tc.sharedDir, tc.oldSharedDir)
			if tc.expErr == nil {
				assert.Equal(t, tc.expErr, err, "Unexpected result")
			} else {
				require.Error(t, err, "Unexpected result")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected result")
			}

			// cleanup
			unix.Unmount(tc.sharedDir, 0)
			os.RemoveAll(tc.sharedDir)
			os.RemoveAll(tc.oldSharedDir)

		})
	}
}

func TestSetSharedDirGroup(t *testing.T) {
	testCases := []struct {
		name      string
		sharedDir string
		group     string
		expErr    string
	}{
		{
			name:      "set group",
			sharedDir: "#sharedDir#",
			group:     "#group#",
			expErr:    "",
		},
		{
			name:      "fail to set bad group",
			sharedDir: "#sharedDir#",
			group:     "B@DGrO0P!",
			expErr:    "^(group: unknown group|user: lookup groupname)",
		},
		{
			name:      "fail to set group of broken shared dir",
			sharedDir: "/proc/broken_shared_dir",
			group:     "root",
			expErr:    "^chown /proc/broken_shared_dir",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sharedDir, dirErr := ioutil.TempDir("/tmp", "test-cniovs-")
			require.NoError(t, dirErr, "Can't create temporary directory")
			tc.sharedDir = strings.Replace(tc.sharedDir, "#sharedDir#", sharedDir, -1)
			defer os.RemoveAll(sharedDir)

			// read sharedDir original group to avoid system changes
			if tc.group == "#group#" {
				dirInfo, _ := os.Stat(sharedDir)
				dirSys := dirInfo.Sys().(*syscall.Stat_t)
				group, _ := user.LookupGroupId(string(48 + dirSys.Gid))
				tc.group = group.Name
			}

			// create default socket dir if needed, so its group can be set
			if _, err := os.Stat(DefaultHostVhostuserBaseDir); os.IsNotExist(err) {
				require.NoError(t, os.MkdirAll(DefaultHostVhostuserBaseDir, 0700), "Can't create default socket dir")
				defer os.RemoveAll(DefaultHostVhostuserBaseDir)
			}

			err := setSharedDirGroup(tc.sharedDir, tc.group)
			if tc.expErr == "" {
				assert.NoError(t, err, "Unexpected result")
			} else {
				require.Error(t, err, "Unexpected result")
				assert.Regexp(t, tc.expErr, err.Error(), "Unexpected result")
			}
		})
	}
}
func TestAddLocalDeviceVhost(t *testing.T) {
	var data OvsSavedData

	testCases := []struct {
		name      string
		netConf   *types.NetConf
		brokenDir string
		createDir bool
		fakeErr   error
		expErr    string
	}{
		{
			name:      "add port with default socket file",
			netConf:   &types.NetConf{},
			createDir: true,
			expErr:    "",
		},
		{
			name:      "add port with given socket file",
			netConf:   &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", NetType: "bridge", VhostConf: types.VhostConf{Socketfile: "test-socket"}}},
			createDir: true,
			expErr:    "",
		},
		{
			name:      "add port with default socket file without sharedDir",
			netConf:   &types.NetConf{},
			createDir: false,
			expErr:    "",
		},
		{
			name:      "add port with default socket file with bad sharedDir",
			netConf:   &types.NetConf{},
			brokenDir: "/proc/broken_shared_dir",
			createDir: false,
			expErr:    "^mkdir ",
		},
		{
			name:      "add port with client mode",
			netConf:   &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", NetType: "bridge", VhostConf: types.VhostConf{Mode: "client"}}},
			createDir: true,
			expErr:    "",
		},
		{
			name:      "fail to create vhost port in client mode",
			netConf:   &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", NetType: "bridge", VhostConf: types.VhostConf{Mode: "client"}}},
			createDir: true,
			fakeErr:   errors.New("MAC error"),
			expErr:    "^MAC error",
		},
		{
			name:      "fail to create port with bad group",
			netConf:   &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", NetType: "bridge", VhostConf: types.VhostConf{Mode: "client", Group: "B@DGrO0P!"}}},
			createDir: true,
			expErr:    "^(group: unknown group|user: lookup groupname)",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := testdata.GetTestArgs()
			execCommand := &FakeExecCommand{Err: tc.fakeErr}

			sharedDir, dirErr := ioutil.TempDir("/tmp", "test-cniovs-")
			require.NoError(t, dirErr, "Can't create temporary directory")
			if !tc.createDir {
				require.NoError(t, os.RemoveAll(sharedDir))
			}
			if tc.brokenDir != "" {
				sharedDir = tc.brokenDir
			} else {
				defer os.RemoveAll(sharedDir)
			}

			socketFile := fmt.Sprintf("%s-%s", args.ContainerID[:12], args.IfName)
			// randomize socket file name if specified
			if tc.netConf.HostConf.VhostConf.Socketfile != "" {
				rand := strings.Split(sharedDir, "-")[2]
				socketFile = tc.netConf.HostConf.VhostConf.Socketfile + "-" + rand
				tc.netConf.HostConf.VhostConf.Socketfile = socketFile
			}
			// prepare fake socket file for vhost user SERVER port
			if tc.netConf.HostConf.VhostConf.Mode != "client" {
				if _, err := os.Stat(defaultOvSSocketDir); os.IsNotExist(err) {
					require.NoError(t, os.MkdirAll(defaultOvSSocketDir, 0700), "Can't create ovs dir")
					defer os.RemoveAll(defaultOvSSocketDir)
				}
				path := path.Join(defaultOvSSocketDir, socketFile)
				require.NoError(t, ioutil.WriteFile(path, []byte(""), 0644), "Can't create test file")
				defer os.Remove(path)

			}

			// add trailing slash due to bug in the createVhostPort - see os.Rename part
			sharedDir = sharedDir + "/"
			SetExecCommand(execCommand)
			err := addLocalDeviceVhost(tc.netConf, args, sharedDir, &data)
			SetDefaultExecCommand()

			if tc.expErr == "" {
				require.NoError(t, err, "Unexpected result")
				assert.DirExists(t, sharedDir, "Shared directory was not created")
				assert.Equal(t, socketFile, data.Vhostname, "Unexpected vhost socket name")
				// test presence of vhost SERVER port socket
				if tc.netConf.HostConf.VhostConf.Mode != "client" {
					assert.FileExists(t, path.Join(sharedDir, socketFile), "Vhost user server port socket not found")
				}
			} else {
				require.Error(t, err, "Unexpected result")
				assert.Regexp(t, tc.expErr, err.Error(), "Unexpected result")
			}
		})
	}
}

func createSocket(t *testing.T, dir, socket string, isDir bool) (socketPath string) {
	socketPath = path.Join(dir, socket)
	if isDir {
		// error case - create dir instead of file and create file inside it
		require.NoError(t, os.MkdirAll(socketPath, 0700), "Can't create test directory")
		require.NoError(t, ioutil.WriteFile(path.Join(socketPath, socket), []byte(""), 0644), "Can't create test file")
	} else {
		require.NoError(t, ioutil.WriteFile(socketPath, []byte(""), 0644), "Can't create test file")
	}
	return
}

func TestDelLocalDeviceVhost(t *testing.T) {
	var data OvsSavedData

	testCases := []struct {
		name        string
		netConf     *types.NetConf
		sharedDir   string
		socketFiles int
		noiseFiles  int
		brokenFiles bool
		brokenDir   string
		expErr      error
	}{
		{
			name:        "fail to open read only shared dir",
			netConf:     &types.NetConf{},
			brokenDir:   "proc",
			socketFiles: 0,
			expErr:      errors.New("open "),
		},
		{
			name:        "fail to open file pretending to be shared dir",
			netConf:     &types.NetConf{},
			brokenDir:   "file",
			socketFiles: 0,
			expErr:      errors.New("readdir"),
		},
		{
			name:        "delete empty shared dir",
			netConf:     &types.NetConf{},
			socketFiles: 0,
			expErr:      nil,
		},
		{
			name:        "delete shared dir with 1 socket file",
			netConf:     &types.NetConf{},
			socketFiles: 1,
			expErr:      nil,
		},
		{
			name:        "fail to delete directory pretending to be a socket file",
			netConf:     &types.NetConf{},
			socketFiles: 1,
			brokenFiles: true,
			expErr:      errors.New("remove "),
		},
		{
			name:        "delete shared dir with multiple socket files",
			netConf:     &types.NetConf{},
			socketFiles: 5,
			expErr:      nil,
		},
		{
			name:        "delete shared dir with configured socket file",
			netConf:     &types.NetConf{HostConf: types.UserSpaceConf{VhostConf: types.VhostConf{Socketfile: "test-socket0"}}},
			socketFiles: 0,
			expErr:      nil,
		},
		{
			name:        "fail to delete directory pretending to be a configured socket file",
			netConf:     &types.NetConf{HostConf: types.UserSpaceConf{VhostConf: types.VhostConf{Socketfile: "test-socket0"}}},
			brokenFiles: true,
			expErr:      errors.New("remove "),
		},
		{
			name:        "delete shared dir with both default and configured socket file",
			netConf:     &types.NetConf{HostConf: types.UserSpaceConf{VhostConf: types.VhostConf{Socketfile: "test-socket0"}}},
			socketFiles: 5,
			expErr:      nil,
		},
		{
			name:        "keep shared dir with both socket and noise file",
			netConf:     &types.NetConf{},
			socketFiles: 1,
			noiseFiles:  1,
			expErr:      nil,
		},
		{
			name:       "keep shared dir with configured socket and noise file",
			netConf:    &types.NetConf{HostConf: types.UserSpaceConf{VhostConf: types.VhostConf{Socketfile: "test-socket0"}}},
			noiseFiles: 1,
			expErr:     nil,
		},
		{
			name:        "keep shared dir with all default, configured and noise files",
			netConf:     &types.NetConf{HostConf: types.UserSpaceConf{VhostConf: types.VhostConf{Socketfile: "test-socket0"}}},
			socketFiles: 2,
			noiseFiles:  2,
			expErr:      nil,
		},
		{
			name:      "delete shared dir with long name",
			netConf:   &types.NetConf{},
			sharedDir: "/var/lib/kubelet/pods/#UUID#/volumes/kubernetes.io/empty-dir/shared-dir",
			expErr:    nil,
		},
		{
			name:      "delete shared dir with long name - dir doesn't exist",
			netConf:   &types.NetConf{},
			sharedDir: "/var/lib/kubelet/pods/#UUID#/volumes/kubernetes.io/empty-dir/shared-dir",
			brokenDir: "none",
			expErr:    nil,
		},
		{
			name:      "fail to delete shared dir with long name - dir isn't mounted",
			netConf:   &types.NetConf{},
			sharedDir: "/var/lib/kubelet/pods/#UUID#/volumes/kubernetes.io/empty-dir/shared-dir",
			brokenDir: "unmount",
			expErr:    errors.New("invalid argument"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var sharedDir string
			args := testdata.GetTestArgs()
			execCommand := &FakeExecCommand{}
			assert := assert.New(t)
			require := require.New(t)

			if tc.sharedDir != "" {
				tc.sharedDir = strings.Replace(tc.sharedDir, "#UUID#", string(uuid.NewUUID()), -1)
				require.NoError(os.MkdirAll(tc.sharedDir, 0700), "Can't create old shared dir")
				sharedDir = getShortSharedDir(tc.sharedDir)
				switch tc.brokenDir {
				case "none":
					// directory shall not exist - do nothing
				case "unmount":
					require.NoError(createSharedDir(sharedDir, tc.sharedDir), "Can't create new short shared dir")
					require.NoError(unix.Unmount(sharedDir, 0), "Can't unmount shared dir")

				default:
					require.NoError(createSharedDir(sharedDir, tc.sharedDir), "Can't create new short shared dir")
				}
				// cleanup if needed
				defer os.RemoveAll(tc.sharedDir)
				defer os.RemoveAll(sharedDir)
				defer unix.Unmount(sharedDir, 0)
			} else {
				var dirErr error
				sharedDir, dirErr = ioutil.TempDir("/tmp", "test-cniovs-")
				require.NoError(dirErr, "Can't create temporary directory")
				switch tc.brokenDir {
				case "proc":
					require.NoError(os.RemoveAll(sharedDir))
					sharedDir = "/proc/broken_shared_dir"
				case "file":
					require.NoError(os.RemoveAll(sharedDir))
					require.NoError(ioutil.WriteFile(sharedDir, []byte(""), 0644), "Can't create test file")
					defer os.Remove(sharedDir)
				}
				// cleanup if needed
				defer os.RemoveAll(sharedDir)
			}

			// prepare fake socket files in shared dir
			for i := 0; i < tc.socketFiles; i++ {
				socketFile := fmt.Sprintf("%s-%s.%v", args.ContainerID[:12], args.IfName, i)
				socketPath := createSocket(t, sharedDir, socketFile, tc.brokenFiles)
				defer os.RemoveAll(socketPath)
			}
			// prepare socket file if specified
			if tc.netConf.HostConf.VhostConf.Socketfile != "" {
				socketPath := createSocket(t, sharedDir, tc.netConf.HostConf.VhostConf.Socketfile, tc.brokenFiles)
				defer os.RemoveAll(socketPath)
			}
			// prepare noise files, which shall remain and avoid shared dir removal
			for i := 0; i < tc.noiseFiles; i++ {
				path := path.Join(sharedDir, fmt.Sprintf("noise-file-%v", i))
				require.NoError(ioutil.WriteFile(path, []byte(""), 0644), "Can't create test file")
				defer os.Remove(path)
			}

			SetExecCommand(execCommand)
			err := delLocalDeviceVhost(tc.netConf, args, sharedDir, &data)
			SetDefaultExecCommand()

			if tc.expErr == nil {
				assert.Equal(tc.expErr, err, "Unexpected result")
				if tc.noiseFiles == 0 {
					assert.NoDirExists(sharedDir, "Shared directory was not removed")
				} else {
					assert.DirExists(sharedDir, "Shared directory was not removed")
				}
			} else {
				require.Error(err, "Unexpected result")
				assert.Contains(err.Error(), tc.expErr.Error(), "Unexpected result")
			}
		})
	}
}

func TestAddLocalNetworkBridge(t *testing.T) {
	testCases := []struct {
		name    string
		fakeOut string
		fakeErr error
		expErr  error
		expCmd  string
		expArg  string
	}{
		{
			name:   "add new bridge",
			expCmd: "ovs-vsctl",
			expArg: "add-br",
		},
		{
			name:    "fail to create new bridge",
			fakeErr: errors.New("bridge error"),
			expErr:  errors.New("bridge error"),
		},
		{
			name:    "skip creation of existing bridge",
			fakeOut: "br0",
			expCmd:  "ovs-vsctl",
			expArg:  "find",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var data OvsSavedData
			args := testdata.GetTestArgs()
			execCommand := &FakeExecCommand{Out: []byte(tc.fakeOut), Err: tc.fakeErr}

			SetExecCommand(execCommand)
			err := addLocalNetworkBridge(&types.NetConf{}, args, &data)
			SetDefaultExecCommand()

			if tc.expErr == nil {
				assert.Equal(t, tc.expErr, err, "Unexpected result")
				assert.Equal(t, tc.expCmd, execCommand.Cmd, "Unexpected ovs command executed")
				assert.Contains(t, execCommand.Args, tc.expArg, "Unexpected ovs command arguments")
			} else {
				require.Error(t, err, "Unexpected result")
				assert.Equal(t, err.Error(), tc.expErr.Error(), "Unexpected result")
			}
		})
	}
}

func TestDelLocalNetworkBridge(t *testing.T) {
	testCases := []struct {
		name    string
		fakeOut string
		fakeErr error
		expErr  error
		expCmd  string
		expArg  string
	}{
		{
			name:   "delete bridge",
			expCmd: "ovs-vsctl",
			expArg: "del-br",
		},
		{
			name:    "fail to delete bridge",
			fakeErr: errors.New("bridge error"),
			expErr:  errors.New("bridge error"),
		},
		{
			name:    "skip deletion of bridge with interfaces",
			fakeErr: nil,
			fakeOut: "eth0",
			expErr:  nil,
			expCmd:  "ovs-vsctl",
			expArg:  "list-ports",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var data OvsSavedData
			args := testdata.GetTestArgs()
			execCommand := &FakeExecCommand{Out: []byte(tc.fakeOut), Err: tc.fakeErr}

			SetExecCommand(execCommand)
			err := delLocalNetworkBridge(&types.NetConf{}, args, &data)
			SetDefaultExecCommand()

			if tc.expErr == nil {
				assert.Equal(t, tc.expErr, err, "Unexpected result")
				assert.Equal(t, tc.expCmd, execCommand.Cmd, "Unexpected ovs command executed")
				assert.Contains(t, execCommand.Args, tc.expArg, "Unexpected ovs command arguments")
			} else {
				require.Error(t, err, "Unexpected result")
				assert.Equal(t, err.Error(), tc.expErr.Error(), "Unexpected result")
			}
		})
	}
}
