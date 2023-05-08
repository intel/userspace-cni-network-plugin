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

package configdata

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/containernetworking/cni/pkg/skel"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/intel/userspace-cni-network-plugin/pkg/types"
	"github.com/intel/userspace-cni-network-plugin/userspace/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSaveRemoteConfig(t *testing.T) {
	testCases := []struct {
		name          string
		netConf       *types.NetConf
		kubeClientNil bool
		testType      string
		ipResult      *current.Result
		brokenDir     string
		expJson       string
		expErr        error
	}{
		{
			name:    "save to pod vhostuser with host mode client and NetConf name",
			netConf: &types.NetConf{Name: "Simple NetConf", HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", VhostConf: types.VhostConf{Mode: "client"}}},
			expJson: `[{"containerId":"#UUID#","ifName":"#ifName#","name":"","config":{"iftype":"vhostuser","memif":{},"vhost":{"mode":"server"},"bridge":{}},"ipResult":{"dns":{}},"name":"Simple NetConf"}]`,
		},
		{
			name:    "save to pod vhostuser with host mode server",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", VhostConf: types.VhostConf{Mode: "server"}}},
			expJson: `[{"containerId":"#UUID#","ifName":"#ifName#","name":"","config":{"iftype":"vhostuser","memif":{},"vhost":{"mode":"client"},"bridge":{}},"ipResult":{"dns":{}}}]`,
		},
		{
			name:    "save to pod vhostuser with host mode client",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", VhostConf: types.VhostConf{Mode: "client"}}},
			expJson: `[{"containerId":"#UUID#","ifName":"#ifName#","name":"","config":{"iftype":"vhostuser","memif":{},"vhost":{"mode":"server"},"bridge":{}},"ipResult":{"dns":{}}}]`,
		},
		{
			name:    "save to pod vhostuser with no host mode",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser"}},
			expJson: `[{"containerId":"#UUID#","ifName":"#ifName#","name":"","config":{"iftype":"vhostuser","memif":{},"vhost":{"mode":"client"},"bridge":{}},"ipResult":{"dns":{}}}]`,
		},
		{
			name:    "save to pod vhostuser with both host and ContainerConf mode server",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", VhostConf: types.VhostConf{Mode: "server"}}, ContainerConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", VhostConf: types.VhostConf{Mode: "server"}}},
			expJson: `[{"containerId":"#UUID#","ifName":"#ifName#","name":"","config":{"engine":"ovs-dpdk","iftype":"vhostuser","memif":{},"vhost":{"mode":"server"},"bridge":{}},"ipResult":{"dns":{}}}]`,
		},
		{
			name:    "save to pod vhostuser with ContainerConf mode server and Socketfile override",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", VhostConf: types.VhostConf{Mode: "client", Socketfile: "vhostuser-hostconf.sock"}}, ContainerConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", VhostConf: types.VhostConf{Mode: "server", Socketfile: "vhostuser-containerconf.sock"}}},
			// FIXME: possible bug - Socketfile from ContainerConf is overrided by value from HostConf!
			expJson: `[{"containerId":"#UUID#","ifName":"#ifName#","name":"","config":{"engine":"ovs-dpdk","iftype":"vhostuser","memif":{},"vhost":{"mode":"server","socketfile":"vhostuser-hostconf.sock"},"bridge":{}},"ipResult":{"dns":{}}}]`,
		},

		{
			name:    "save to pod without ContainerConf netType",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", NetType: "bridge", VhostConf: types.VhostConf{Mode: "client"}}},
			expJson: `[{"containerId":"#UUID#","ifName":"#ifName#","name":"","config":{"iftype":"vhostuser","memif":{},"vhost":{"mode":"server"},"bridge":{}},"ipResult":{"dns":{}}}]`,
		},
		{
			name:     "save to pod with ipResult and without ContainerConf netType",
			netConf:  &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", NetType: "bridge", VhostConf: types.VhostConf{Mode: "client"}}},
			ipResult: &current.Result{Interfaces: []*current.Interface{&current.Interface{Name: "vlan0", Mac: "fe:ed:de:ad:be:ef"}}},
			expJson:  `[{"containerId":"#UUID#","ifName":"#ifName#","name":"","config":{"iftype":"vhostuser","memif":{},"netType":"interface","vhost":{"mode":"server"},"bridge":{}},"ipResult":{"interfaces":[{"name":"vlan0","mac":"fe:ed:de:ad:be:ef"}],"dns":{}}}]`,
		},
		{
			name:     "save to pod with ipResult and with ContainerConf netType set",
			netConf:  &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", NetType: "bridge", VhostConf: types.VhostConf{Mode: "client"}}, ContainerConf: types.UserSpaceConf{NetType: "bridge"}},
			ipResult: &current.Result{Interfaces: []*current.Interface{&current.Interface{Name: "vlan0", Mac: "fe:ed:de:ad:be:ef"}}},
			expJson:  `[{"containerId":"#UUID#","ifName":"#ifName#","name":"","config":{"iftype":"vhostuser","memif":{},"netType":"bridge","vhost":{"mode":"server"},"bridge":{}},"ipResult":{"interfaces":[{"name":"vlan0","mac":"fe:ed:de:ad:be:ef"}],"dns":{}}}]`,
		},
		{
			name:    "save to pod with ContainerConf ifType set",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", NetType: "bridge", VhostConf: types.VhostConf{Mode: "client"}}, ContainerConf: types.UserSpaceConf{IfType: "interface"}},
			expJson: `[{"containerId":"#UUID#","ifName":"#ifName#","name":"","config":{"iftype":"interface","memif":{}, "vhost":{},"bridge":{}},"ipResult":{"dns":{}}}]`,
		},
		{
			name:    "save to pod with ifType memif and no host role",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "memif"}},
			expJson: `[{"containerId":"#UUID#","ifName":"#ifName#","name":"","config":{"iftype":"memif","memif":{"role":"master"},"vhost":{},"bridge":{}},"ipResult":{"dns":{}}}]`,
		},
		{
			name:    "save to pod with ifType memif and ContainerConf role master and socketfile override",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "memif", MemifConf: types.MemifConf{Role: "master", Socketfile: "memif-hostconf.sock"}}, ContainerConf: types.UserSpaceConf{IfType: "memif", MemifConf: types.MemifConf{Role: "master", Socketfile: "memif-memifconf.sock"}}},
			// FIXME: possible bug - Socketfile from ContainerConf is overrided by value from HostConf!
			expJson: `[{"containerId":"#UUID#","ifName":"#ifName#","name":"","config":{"iftype":"memif","memif":{"role":"master","socketfile":"memif-hostconf.sock"},"vhost":{},"bridge":{}},"ipResult":{"dns":{}}}]`,
		},
		{
			name:    "save to pod with ifType memif and host role master",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "memif", MemifConf: types.MemifConf{Role: "master"}}},
			expJson: `[{"containerId":"#UUID#","ifName":"#ifName#","name":"","config":{"iftype":"memif","memif":{"role":"slave"},"vhost":{},"bridge":{}},"ipResult":{"dns":{}}}]`,
		},
		{
			name:    "save to pod with ifType memif and host role slave",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "memif", MemifConf: types.MemifConf{Role: "slave"}}},
			expJson: `[{"containerId":"#UUID#","ifName":"#ifName#","name":"","config":{"iftype":"memif","memif":{"role":"master"},"vhost":{},"bridge":{}},"ipResult":{"dns":{}}}]`,
		},
		{
			name:    "save to pod with ifType memif and host role master and mode ip",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "memif", MemifConf: types.MemifConf{Role: "master", Mode: "ip"}}},
			expJson: `[{"containerId":"#UUID#","ifName":"#ifName#","name":"","config":{"iftype":"memif","memif":{"role":"slave","mode":"ip"},"vhost":{},"bridge":{}},"ipResult":{"dns":{}}}]`,
		},
		{
			name:    "save to pod with ifType memif and ContainerConf role master and mode ethernet",
			netConf: &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "memif", MemifConf: types.MemifConf{Role: "slave", Mode: "ip"}}, ContainerConf: types.UserSpaceConf{IfType: "memif", MemifConf: types.MemifConf{Role: "master", Mode: "ethernet"}}},

			expJson: `[{"containerId":"#UUID#","ifName":"#ifName#","name":"","config":{"iftype":"memif","memif":{"role":"master","mode":"ethernet"},"vhost":{},"bridge":{}},"ipResult":{"dns":{}}}]`,
		},
		{
			name:     "save to file",
			netConf:  &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", NetType: "bridge", VhostConf: types.VhostConf{Mode: "client"}}},
			testType: "client_nil",
			expJson:  `{"containerId":"#UUID#","ifName":"#ifName#","name":"","config":{"iftype":"vhostuser","memif":{},"vhost":{"mode":"server"},"bridge":{}},"ipResult":{"dns":{}}}`,
		},
		{
			name:      "save to file to newly created shared dir",
			netConf:   &types.NetConf{HostConf: types.UserSpaceConf{Engine: "ovs-dpdk", IfType: "vhostuser", NetType: "bridge", VhostConf: types.VhostConf{Mode: "client"}}},
			testType:  "client_nil",
			brokenDir: "none",
			expJson:   `{"containerId":"#UUID#","ifName":"#ifName#","name":"","config":{"iftype":"vhostuser","memif":{},"vhost":{"mode":"server"},"bridge":{}},"ipResult":{"dns":{}}}`,
			expErr:        nil,
		},
		{
			name:      "fail to save file to broken dir",
			netConf:   &types.NetConf{},
			testType:  "client_nil",
			brokenDir: "proc",
			expErr:    errors.New("mkdir "),
		},
		{
			name:      "fail to save to file pretending to be shared dir",
			netConf:   &types.NetConf{},
			testType:  "client_nil",
			brokenDir: "file",
			expErr:    errors.New("stat "),
		},
		{
			name:     "fail with netconf set to nil",
			netConf:  nil,
			testType: "netconf_nil",
			expErr:   errors.New("SaveRemoteConfig(): Error conf is set to: <nil>"),
		},
		{
			name:     "fail with args set to nil",
			netConf:  &types.NetConf{},
			testType: "args_nil",
			expErr:   errors.New("SaveRemoteConfig(): Error args is set to: <nil>"),
		},
		{
			name:     "fail with pod set tot nil",
			netConf:  &types.NetConf{},
			testType: "pod_nil",
			expErr:   errors.New("SaveRemoteConfig(): Error pod is set to: nil"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var kubeClient kubernetes.Interface
			var resPod *v1.Pod
			var resErr error

			sharedDir, dirErr := ioutil.TempDir("/tmp", "test-configdata-")
			require.NoError(t, dirErr, "Can't create temporary directory")
			// remove sharedDir if needed
			defer os.RemoveAll(sharedDir)

			switch tc.brokenDir {
			case "proc":
				sharedDir = "/proc/broken_dir"
			case "file":
				os.RemoveAll(sharedDir)
				_, err := os.Create(sharedDir)
				require.NoError(t, err, "Can't create temp file")
				// add trailing slash to fail directory check by os.stat()
				sharedDir = sharedDir + "/"
			case "none":
				os.RemoveAll(sharedDir)
			}
			args := testdata.GetTestArgs()
			pod := testdata.GetTestPod(sharedDir)
			podOrig := pod.DeepCopy()
			kubeClient = fake.NewSimpleClientset(pod)

			tc.expJson = strings.Replace(tc.expJson, "#UUID#", args.ContainerID, -1)
			tc.expJson = strings.Replace(tc.expJson, "#ifName#", args.IfName, -1)

			// NOTE: ipResult set to nil is a valid case tested by several TCs already
			switch tc.testType {
			case "conf_nil":
				resPod, resErr = SaveRemoteConfig(nil, args, kubeClient, sharedDir, pod, tc.ipResult)
			case "args_nil":
				resPod, resErr = SaveRemoteConfig(tc.netConf, nil, kubeClient, sharedDir, pod, tc.ipResult)
			case "client_nil":
				resPod, resErr = SaveRemoteConfig(tc.netConf, args, nil, sharedDir, pod, tc.ipResult)
			case "pod_nil":
				resPod, resErr = SaveRemoteConfig(tc.netConf, args, kubeClient, sharedDir, nil, tc.ipResult)
			default:
				resPod, resErr = SaveRemoteConfig(tc.netConf, args, kubeClient, sharedDir, pod, tc.ipResult)
			}
			var data []byte
			if tc.expErr == nil {
				assert.NoError(t, resErr, "Unexpected error")
				if tc.testType == "client_nil" {
					// data saved to the file
					assert.Equal(t, podOrig, resPod, "Unexpected change of pod data")
					assert.Empty(t, resPod.Annotations["userspace/configuration-data"], "Unexpected pod Annotations were found")
					fileName := fmt.Sprintf("configData-%s-%s.json", args.ContainerID[:12], args.IfName)
					fileName = filepath.Join(sharedDir, fileName)
					require.FileExists(t, fileName, "Container data were not saved to file")
					var err error
					data, err = ioutil.ReadFile(fileName)
					require.NoError(t, err, "Can't read saved container data")
				} else {
					// data saved to resPod Annotations
					assert.NotEqual(t, podOrig, resPod, "Pod data shall be modified")
					require.NotEmpty(t, resPod.Annotations["userspace/configuration-data"], "Data are not saved to pod Annotations")
					data = []byte(resPod.Annotations["userspace/configuration-data"])
				}
				assert.JSONEq(t, tc.expJson, string(data), fmt.Sprintf("Unexpected result\n%v\n", string(data)))
			} else {
				require.Error(t, resErr, "Unexpected result")
				assert.Contains(t, resErr.Error(), tc.expErr.Error(), "Unexpected result")
			}

		})
	}
}

func TestCleanupRemoteConfig(t *testing.T) {
	testCases := []struct {
		name   string
		dir    string
		expOut string
	}{
		{
			name: "remove empty dir",
			dir:  "",
		},
		{
			name: "remove dir with content",
			dir:  "full",
		},
		{
			name:   "fail to remove broken dir",
			dir:    "proc",
			expOut: "unlinkat ",
		},
		{
			name: "remove file pretending to be dir",
			dir:  "file",
		},
		{
			name: "remove dir which doesn't exist",
			dir:  "none",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir, dirErr := ioutil.TempDir("/tmp", "test-configdata-")
			require.NoError(t, dirErr, "Can't create temporary directory")
			// remove tempDir if needed
			defer os.RemoveAll(tempDir)

			switch tc.dir {
			case "full":
				fileName := filepath.Join(tempDir, "test-configdata.txt")
				_, fileErr := os.Create(fileName)
				require.NoError(t, fileErr, "Can't create temp file")
			case "proc":
				os.RemoveAll(tempDir)
				tempDir = "/proc/meminfo"
			case "file":
				os.RemoveAll(tempDir)
				_, err := os.Create(tempDir)
				require.NoError(t, err, "Can't create temp file")
			case "none":
				os.RemoveAll(tempDir)
			}

			stdR, stdW, err := os.Pipe()
			require.NoError(t, err, "Can't capture stdout")
			origStdOut := os.Stdout
			os.Stdout = stdW

			CleanupRemoteConfig(nil, tempDir)

			os.Stdout = origStdOut
			stdW.Close()
			var buf bytes.Buffer
			io.Copy(&buf, stdR)

			if tc.expOut != "" {
				assert.Contains(t, buf.String(), tc.expOut, "Unexpected standard output")
			} else {
				assert.NoDirExists(t, tempDir, "Dir was not removed")
				assert.NoFileExists(t, tempDir, "Dir was not removed")
			}
		})
	}
}

func TestFileCleanup(t *testing.T) {
	testCases := []struct {
		name      string
		directory string
		filepath  string
		noiseFile bool
		expErr    error
	}{
		{
			name:      "remove dir and file",
			directory: "#tempDir#",
			filepath:  "#tempDir#/test-file.txt",
		},
		{
			name:      "remove file and keep directory",
			directory: "",
			filepath:  "#tempDir#/test-file.txt",
		},
		{
			name:      "remove file and keep directory due to other content",
			directory: "#tempDir#",
			filepath:  "#tempDir#/test-file.txt",
			noiseFile: true,
		},
		{
			name:      "remove only empty directory",
			directory: "#tempDir#",
			filepath:  "",
		},
		{
			name:      "don't remove only directory with data",
			directory: "#tempDir#",
			filepath:  "",
			noiseFile: true,
		},
		{
			name:      "fail to remove file",
			directory: "",
			filepath:  "/proc/meminfo",
			expErr:    errors.New("ERROR: Failed to delete file: remove"),
		},
		{
			name:      "fail to remove directory but remove file",
			directory: "/proc",
			filepath:  "#tempDir#/test-file.txt",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir, dirErr := ioutil.TempDir("/tmp", "test-configdata-")
			require.NoError(t, dirErr, "Can't create temporary directory")
			// remove tempDir if needed
			defer os.RemoveAll(tempDir)

			tc.directory = strings.Replace(tc.directory, "#tempDir#", tempDir, -1)
			tc.filepath = strings.Replace(tc.filepath, "#tempDir#", tempDir, -1)

			if tc.filepath != "" {
				if _, err := os.Stat(tc.filepath); err != nil {
					_, fileErr := os.Create(tc.filepath)
					require.NoError(t, fileErr, "Can't create test file")
				}
			}
			if tc.noiseFile {
				noiseFile := filepath.Join(tempDir, "noise-file.txt")
				_, fileErr := os.Create(noiseFile)
				require.NoError(t, fileErr, "Can't create temp file")
			}

			err := FileCleanup(tc.directory, tc.filepath)

			if tc.expErr != nil {
				require.Error(t, err, "Error was expected")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected error")
			} else {
				require.NoError(t, err, "Unexpected error")
				if !(tc.directory == "" || tc.noiseFile || strings.Contains(tc.directory, "/proc")) {
					assert.NoDirExists(t, tempDir, "Directory was not removed")
				} else {
					assert.DirExists(t, tempDir, "Directory was removed")
				}

				if tc.filepath != "" {
					assert.NoFileExists(t, filepath.Join(tempDir, tc.filepath), "File was not removed")
				}
			}
		})
	}
}

func TestGetRemoteConfig(t *testing.T) {
	testCases := []struct {
		name        string
		annotations string
		expErr      error
		expDir      string
		expResult   []*InterfaceData
	}{
		{
			name:        "get config for one interface",
			annotations: `userspace/mapped-dir=#tempDir# userspace/configuration-data="[{\"Name\":\"Container New Name\",\"containerId\":\"123-456-789-007\",\"ifName\":\"eth7\",\"config\":{\"iftype\":\"memif\",\"memif\":{\"role\":\"master\"}}}]"`,
			expDir:      "#tempDir#",
			expResult:   []*InterfaceData{&InterfaceData{Args: skel.CmdArgs{ContainerID: "123-456-789-007", IfName: "eth7"}, NetConf: types.NetConf{Name: "Container New Name", HostConf: types.UserSpaceConf{IfType: "memif", MemifConf: types.MemifConf{Role: "master"}}}}},
		},
		{
			name:        "get config for two interfaces",
			annotations: `userspace/mapped-dir=#tempDir# userspace/configuration-data="[{\"Name\":\"Container New Name\",\"containerId\":\"123-456-789-007\",\"ifName\":\"eth7\",\"config\":{\"iftype\":\"memif\",\"memif\":{\"role\":\"master\"}}},{\"Name\":\"Container New Name\",\"containerId\":\"123-456-789-007\",\"ifName\":\"eth9\",\"config\":{\"iftype\":\"memif\",\"memif\":{\"role\":\"slave\"}}}]"`,
			expDir:      "#tempDir#",
			expResult:   []*InterfaceData{&InterfaceData{Args: skel.CmdArgs{ContainerID: "123-456-789-007", IfName: "eth7"}, NetConf: types.NetConf{Name: "Container New Name", HostConf: types.UserSpaceConf{IfType: "memif", MemifConf: types.MemifConf{Role: "master"}}}}, &InterfaceData{Args: skel.CmdArgs{ContainerID: "123-456-789-007", IfName: "eth9"}, NetConf: types.NetConf{Name: "Container New Name", HostConf: types.UserSpaceConf{IfType: "memif", MemifConf: types.MemifConf{Role: "slave"}}}}},
		},
		{
			name:        "get config for two containers",
			annotations: `userspace/mapped-dir=#tempDir# userspace/configuration-data="[{\"Name\":\"init-container\",\"containerId\":\"123-456-789-007\",\"ifName\":\"eth7\",\"config\":{\"iftype\":\"memif\",\"memif\":{\"role\":\"master\"}}},{\"Name\":\"worker container\",\"containerId\":\"123-456-789-042\",\"ifName\":\"vlan0\",\"config\":{\"iftype\":\"memif\",\"memif\":{\"role\":\"slave\"}}}]"`,
			expDir:      "#tempDir#",
			expResult:   []*InterfaceData{&InterfaceData{Args: skel.CmdArgs{ContainerID: "123-456-789-007", IfName: "eth7"}, NetConf: types.NetConf{Name: "init-container", HostConf: types.UserSpaceConf{IfType: "memif", MemifConf: types.MemifConf{Role: "master"}}}}, &InterfaceData{Args: skel.CmdArgs{ContainerID: "123-456-789-042", IfName: "vlan0"}, NetConf: types.NetConf{Name: "worker container", HostConf: types.UserSpaceConf{IfType: "memif", MemifConf: types.MemifConf{Role: "slave"}}}}},
		},
		{
			name:        "fail without annotation file",
			annotations: "",
			expErr:      errors.New("error reading "),
		},
		{
			name:        "fail without config data",
			annotations: "userspace/mapped-dir=#tempDir#",
			expErr:      errors.New(`ERROR: "userspace/configuration-data" missing from pod annotation`),
		},
		{
			name:        "fail without mapped dir",
			annotations: "userspace/no-mappedddir=#tempDir#",
			expDir:      "#tempDir#",
			expErr:      errors.New(`ERROR: "userspace/mapped-dir" missing from pod annotation`),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir, dirErr := ioutil.TempDir("/tmp", "test-configdata-")
			require.NoError(t, dirErr, "Can't create temporary directory")
			// remove tempDir if needed
			defer os.RemoveAll(tempDir)

			annotFile := filepath.Join(tempDir, "annotations")
			if tc.annotations != "" {
				tc.annotations = strings.Replace(tc.annotations, "#tempDir#", tempDir, -1)
				ioutil.WriteFile(annotFile, []byte(tc.annotations), 0644)
			}
			tc.expDir = strings.Replace(tc.expDir, "#tempDir#", tempDir, -1)

			ifcData, mappedDir, err := GetRemoteConfig(annotFile)

			if tc.expErr != nil {
				require.Error(t, err, "Error was expected")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected error")
			} else {
				require.NoError(t, err, "Unexpected error")
				assert.Equal(t, tc.expDir, mappedDir, "Unexpected result")
				require.NotNil(t, ifcData, "Unexpected result")
				assert.Equal(t, tc.expResult, ifcData, "Unexpected result")
			}
		})
	}
}
