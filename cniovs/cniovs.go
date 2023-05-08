// Copyright (c) 2018-2020 Red Hat, Intel Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//
// This module provides the library functions to implement the
// OVS UserSpace CNI implementation. The input to the library is json
// data defined in pkg/types. If the configuration contains local data,
// the code builds up an 'ovsctl' command to proviosn the local OVS,
// instance. If the configuration contains remote data, the database
// library is used to store the data, which is later read and processed
// locally by the remotes agent.
//

package cniovs

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/containernetworking/cni/pkg/skel"
	current "github.com/containernetworking/cni/pkg/types/100"

	"github.com/intel/userspace-cni-network-plugin/logging"
	"github.com/intel/userspace-cni-network-plugin/pkg/configdata"
	"github.com/intel/userspace-cni-network-plugin/pkg/types"
)

//
// Constants
//
const (
	defaultBridge               = "br0"
	DefaultHostVhostuserBaseDir = "/var/lib/vhost_sockets/"
)

//
// Types
//
type CniOvs struct {
}

//
// API Functions
//
func (cniOvs CniOvs) AddOnHost(conf *types.NetConf,
	args *skel.CmdArgs,
	kubeClient kubernetes.Interface,
	sharedDir string,
	ipResult *current.Result) error {

	var err error
	var data OvsSavedData

	logging.Infof("OVS AddOnHost: ENTER - Container %s Iface %s", args.ContainerID[:12], args.IfName)

	//
	// Mandatory attribute of "ovs-vsctl add-port" is a BridgeName. So even if
	// NetType is not set to "bridge", "conf.HostConf.BridgeConf.BridgeName"
	// should be set. If it is not, set it to default value.
	//
	if conf.HostConf.BridgeConf.BridgeName == "" {
		conf.HostConf.BridgeConf.BridgeName = defaultBridge
	}

	//
	// Create bridge before creating Interface
	//
	err = addLocalNetworkBridge(conf, args, &data)
	if err != nil {
		logging.Debugf("AddOnHost(ovs): %v", err)
		return err
	}

	//
	// Create Local Interface
	//
	if conf.HostConf.IfType == "vhostuser" {
		err = addLocalDeviceVhost(conf, args, sharedDir, &data)
	} else {
		err = errors.New("ERROR: Unknown HostConf.IfType:" + conf.HostConf.IfType)
	}
	if err != nil {
		logging.Debugf("AddOnHost(ovs): %v", err)
		return err
	}

	//
	// Bring Interface UP
	//

	//
	// Add Interface to Local Network
	//
	if conf.HostConf.NetType == "interface" {
		err = errors.New("ERROR: HostConf.NetType \"interface\" not supported.")
	} else if conf.HostConf.NetType != "bridge" && conf.HostConf.NetType != "" {
		err = errors.New("ERROR: Unknown HostConf.NetType:" + conf.HostConf.NetType)
	}
	if err != nil {
		logging.Debugf("AddOnHost(ovs): %v", err)
		return err
	}

	//
	// Save Config - Save Create Data for Delete
	//
	err = SaveConfig(conf, args, &data)

	return err
}

func (cniOvs CniOvs) AddOnContainer(conf *types.NetConf,
	args *skel.CmdArgs,
	kubeClient kubernetes.Interface,
	sharedDir string,
	pod *v1.Pod,
	ipResult *current.Result) (*v1.Pod, error) {
	logging.Infof("OVS AddOnContainer: ENTER - Container %s Iface %s", args.ContainerID[:12], args.IfName)
	return configdata.SaveRemoteConfig(conf, args, kubeClient, sharedDir, pod, ipResult)
}

func (cniOvs CniOvs) DelFromHost(conf *types.NetConf, args *skel.CmdArgs, sharedDir string) error {
	var data OvsSavedData
	var err error

	logging.Infof("OVS DelFromHost: ENTER - Container %s Iface %s", args.ContainerID[:12], args.IfName)

	//
	// Load Config - Retrieved squirreled away data needed for processing delete
	//
	err = LoadConfig(conf, args, &data)
	if err != nil {
		logging.Debugf("DelFromHost(ovs): %v", err)
		return err
	}

	//
	// Manditory attribute of "ovs-vsctl add-port" is a BridgeName. So even if
	// NetType is not set to "bridge", "conf.HostConf.BridgeConf.BridgeName"
	// should be set. If it is not, set it to default value.
	//
	if conf.HostConf.BridgeConf.BridgeName == "" {
		conf.HostConf.BridgeConf.BridgeName = defaultBridge
	}

	//
	// Remove Interface from Local Network
	//

	//
	// Delete Local Interface
	//
	if conf.HostConf.IfType == "vhostuser" {
		err = delLocalDeviceVhost(conf, args, sharedDir, &data)
	} else {
		err = errors.New("ERROR: Unknown HostConf.Type:" + conf.HostConf.IfType)
	}
	if err != nil {
		return err
	}

	//
	// Delete Bridge if empty
	//
	err = delLocalNetworkBridge(conf, args, &data)
	if err != nil {
		return err
	}

	return err
}

func (cniOvs CniOvs) DelFromContainer(conf *types.NetConf, args *skel.CmdArgs, sharedDir string, pod *v1.Pod) error {
	logging.Infof("OVS DelFromContainer: ENTER - Container %s Iface %s", args.ContainerID[:12], args.IfName)

	configdata.FileCleanup(sharedDir, "")

	return nil
}

//
// Utility Functions
//

func generateRandomMacAddress() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}

	// Set the local bit and make sure not MC address
	macAddr := fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		(buf[0]|0x2)&0xfe, buf[1], buf[2],
		buf[3], buf[4], buf[5])
	return macAddr
}

func getShortSharedDir(sharedDir string) string {
	// sun_path for unix domain socket has an array size of 108
	// When the sharedDir path length is greater than 89 (108 - 19)
	// 19 is the possible vhostuser socket file name length "/abcdefghijkl-net99" (1 + 12 + 1 + 3 + 2)
	// FIXME: why shareddir is shortened only in case that it contains "empty-dir"?
	if len(sharedDir) >= 89 && strings.Contains(sharedDir, "empty-dir") {
		// Format - /var/lib/kubelet/pods/<podID>/volumes/kubernetes.io~empty-dir/shared-dir
		parts := strings.Split(sharedDir, "/")
		// FIXME: it's not safe; can we assure that shareDir with "empty-dir" will always have at least 5 dirs?
		podID := parts[5]
		newSharedDir := filepath.Join(DefaultHostVhostuserBaseDir, podID)
		logging.Infof("getShortSharedDir: Short shared directory: %s", newSharedDir)
		return newSharedDir
	}
	return sharedDir

}

func createSharedDir(sharedDir, oldSharedDir string) error {
	var err error

	_, err = os.Stat(sharedDir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(sharedDir, 0750)
		if err != nil {
			logging.Errorf("createSharedDir: Failed to create dir (%s): %v", sharedDir, err)
			return err
		}

		if strings.Contains(sharedDir, DefaultHostVhostuserBaseDir) {
			logging.Debugf("createSharedDir: Mount from %s to %s", oldSharedDir, sharedDir)
			err = unix.Mount(oldSharedDir, sharedDir, "", unix.MS_BIND, "")
			if err != nil {
				logging.Errorf("createSharedDir: Failed to bind mount: %s", err)
				return err
			}
		}
		return nil

	}
	return err
}

func setSharedDirGroup(sharedDir string, group string) error {
	groupInfo, err := user.LookupGroup(group)
	if err != nil {
		return err
	}

	logging.Debugf("setSharedDirGroup: group %s's gid is %s", group, groupInfo.Gid)
	gid, err := strconv.Atoi(groupInfo.Gid)
	if err != nil {
		return err
	}

	err = os.Chown(DefaultHostVhostuserBaseDir, -1, gid)
	if err != nil {
		return err
	}

	err = os.Chown(sharedDir, -1, gid)
	if err != nil {
		return err
	}
	return nil
}

func addLocalDeviceVhost(conf *types.NetConf, args *skel.CmdArgs, actualSharedDir string, data *OvsSavedData) error {
	var err error
	var vhostName string

	if conf.HostConf.VhostConf.Socketfile == "" {
		conf.HostConf.VhostConf.Socketfile = fmt.Sprintf("%s-%s", args.ContainerID[:12], args.IfName)
	}

	sharedDir := getShortSharedDir(actualSharedDir)
	err = createSharedDir(sharedDir, actualSharedDir)
	if err != nil {
		logging.Errorf("addLocalDeviceVhost: Failed to create shared dir: %v", err)
		return err
	}

	group := conf.HostConf.VhostConf.Group
	if group != "" {
		err = setSharedDirGroup(sharedDir, group)
		if err != nil {
			logging.Errorf("addLocalDeviceVhost: Failed to set shared dir group: %v", err)
			return err
		}
	}

	// Validate and convert input data
	clientMode := false
	if conf.HostConf.VhostConf.Mode == "client" {
		clientMode = true
	}

	// ovs-vsctl add-port
	if vhostName, err = createVhostPort(sharedDir,
		conf.HostConf.VhostConf.Socketfile,
		clientMode,
		conf.HostConf.BridgeConf.BridgeName); err == nil {
		if vhostPortMac, err := getVhostPortMac(vhostName); err == nil {
			data.VhostMac = vhostPortMac
		} else {
			return err
		}

		data.Vhostname = vhostName
		data.IfMac = generateRandomMacAddress()
	} else {
		return err
	}

	return err
}

func delLocalDeviceVhost(conf *types.NetConf, args *skel.CmdArgs, actualSharedDir string, data *OvsSavedData) error {
	sharedDir := getShortSharedDir(actualSharedDir)

	// ovs-vsctl --if-exists del-port
	err := deleteVhostPort(data.Vhostname, conf.HostConf.BridgeConf.BridgeName)
	if err != nil {
		logging.Errorf("delLocalDeviceVhost: Failed to delete port: %v", err)
		return err
	}

	// Check if sharedDir is a mount dir of EmptyDir
	if strings.Contains(sharedDir, DefaultHostVhostuserBaseDir) {
		logging.Debugf("delLocalDeviceVhost: Unmount shared directory: %v", sharedDir)
		_, err = os.Stat(sharedDir)
		if os.IsNotExist(err) {
			logging.Errorf("delLocalDeviceVhost: shared directory %s does not exist to unmount", sharedDir)
			return nil
		}
		err = unix.Unmount(sharedDir, 0)
		if err != nil {
			logging.Errorf("Failed to unmount dir: %v", err)
			return err
		}
		err = os.Remove(sharedDir)
		if err != nil {
			logging.Errorf("Failed to remove dir: %v", err)
			return err
		}
	} else {
		folder, err := os.Open(sharedDir)
		if err != nil {
			return err
		}
		defer folder.Close()

		fileBaseName := fmt.Sprintf("%s-%s", args.ContainerID[:12], args.IfName)
		filesForContainerID, err := folder.Readdirnames(0)
		if err != nil {
			return err
		}
		numDeletedFiles := 0

		// Remove files with matching container ID and IfName
		for _, fileName := range filesForContainerID {
			if match, _ := regexp.MatchString(fileBaseName+"*", fileName); match == true {
				logging.Debugf("OVS DelFromContainer: %s matches file %s", fileBaseName, fileName)
				file := filepath.Join(sharedDir, fileName)
				if err = os.Remove(file); err != nil {
					return err
				}
				numDeletedFiles++
			} else {
				logging.Debugf("OVS DelFromContainer: %s does NOT match file %s", fileBaseName, fileName)
			}

			// In case the Socketfile name was passed in:
			if conf.HostConf.VhostConf.Socketfile != fileBaseName {
				if match, _ := regexp.MatchString(conf.HostConf.VhostConf.Socketfile+"*", fileName); match == true {
					file := filepath.Join(sharedDir, fileName)
					if err = os.Remove(file); err != nil {
						return err
					}
					numDeletedFiles++
				}
			}
		}
		// Remove folder for container ID if it's empty
		if numDeletedFiles == len(filesForContainerID) {
			if err = os.Remove(sharedDir); err != nil {
				return err
			}
		}
	}

	return nil
}

func addLocalNetworkBridge(conf *types.NetConf, args *skel.CmdArgs, data *OvsSavedData) error {
	var err error

	if found := findBridge(conf.HostConf.BridgeConf.BridgeName); found == false {
		logging.Debugf("addLocalNetworkBridge(): Bridge %s not found, creating", conf.HostConf.BridgeConf.BridgeName)
		err = createBridge(conf.HostConf.BridgeConf.BridgeName)

		if err == nil {
			// Bridge is always created because it is required for interface.
			// If bridge type was actually called out, then set the
			// bridge up as L2 bridge. Otherwise, a controller is
			// responsible for writing flows to OvS.
			if conf.HostConf.NetType == "bridge" {
				err = configL2Bridge(conf.HostConf.BridgeConf.BridgeName)
			}
		}
	} else {
		logging.Debugf("addLocalNetworkBridge(): Bridge %s exists, skip creating", conf.HostConf.BridgeConf.BridgeName)
	}

	return err
}

func delLocalNetworkBridge(conf *types.NetConf, args *skel.CmdArgs, data *OvsSavedData) error {
	var err error

	if containInterfaces := doesBridgeContainInterfaces(conf.HostConf.BridgeConf.BridgeName); containInterfaces == false {
		logging.Debugf("delLocalNetworkBridge(): No interfaces found, deleting Bridge %s", conf.HostConf.BridgeConf.BridgeName)
		err = deleteBridge(conf.HostConf.BridgeConf.BridgeName)
	} else {
		logging.Debugf("delLocalNetworkBridge(): Interfaces found, skip deleting Bridge %s", conf.HostConf.BridgeConf.BridgeName)
	}

	return err
}
