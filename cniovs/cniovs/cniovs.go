// Copyright (c) 2018 Red Hat.
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
// data defined in usrsptypes. If the configuration contains local data,
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
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types/current"

	"github.com/intel/userspace-cni-network-plugin/cniovs/ovsdb"
	"github.com/intel/userspace-cni-network-plugin/usrsptypes"
	"github.com/intel/userspace-cni-network-plugin/logging"
)

//
// Constants
//
const defaultCNIDir = "/var/lib/cni/vhostuser"
const defaultOvsScript = "/usr/share/openvswitch/scripts/ovs-config.py"

//
// Types
//
type CniOvs struct {
}

//
// API Functions
//
func (cniOvs CniOvs) AddOnHost(conf *usrsptypes.NetConf, args *skel.CmdArgs, ipResult *current.Result) error {
	var err error
	var data ovsdb.OvsSavedData

	logging.Debugf("OVS AddOnHost: ENTER")

	//
	// Create Local Interface
	//
	if conf.HostConf.IfType == "vhostuser" {
		err = addLocalDeviceVhost(conf, args, &data)
	} else {
		err = errors.New("ERROR: Unknown HostConf.IfType:" + conf.HostConf.IfType)
	}
	if err != nil {
		return err
	}

	//
	// Bring Interface UP
	//

	//
	// Add Interface to Local Network
	//
	if conf.HostConf.NetType == "bridge" {
		err = errors.New("ERROR: NetType bridge not currenly supported")
	} else if conf.HostConf.NetType == "interface" {
		if len(ipResult.IPs) != 0 {
		}
	}

	//
	// Save Config - Save Create Data for Delete
	//
	err = ovsdb.SaveConfig(conf, args, &data)
	if err != nil {
		return err
	}

	return err
}

func (cniOvs CniOvs) AddOnContainer(conf *usrsptypes.NetConf, args *skel.CmdArgs, ipResult *current.Result) error {
	logging.Debugf("OVS AddOnContainer: ENTER")
	return nil
}

func (cniOvs CniOvs) DelFromHost(conf *usrsptypes.NetConf, args *skel.CmdArgs) error {
	var data ovsdb.OvsSavedData
	var err error

	logging.Debugf("OVS DelFromHost: ENTER")

	//
	// Load Config - Retrieved squirreled away data needed for processing delete
	//
	err = ovsdb.LoadConfig(conf, args, &data)
	if err != nil {
		return err
	}

	//
	// Remove Interface from Local Network
	//

	//
	// Delete Local Interface
	//
	if conf.HostConf.IfType == "vhostuser" {
		return delLocalDeviceVhost(conf, args, &data)
	} else {
		return errors.New("ERROR: Unknown HostConf.Type:" + conf.HostConf.IfType)
	}

	return err
}

func (cniOvs CniOvs) DelFromContainer(conf *usrsptypes.NetConf, args *skel.CmdArgs) error {
	logging.Debugf("OVS DelFromContainer: ENTER")
	return nil
}

//
// Utility Functions
//

// execCommand Execute shell commands and return the output.
func execCommand(cmd string, args []string) ([]byte, error) {
	return exec.Command(cmd, args...).Output()
}

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

func addLocalDeviceVhost(conf *usrsptypes.NetConf, args *skel.CmdArgs, data *ovsdb.OvsSavedData) error {

	s := []string{args.ContainerID[:12], args.IfName}
	sockRef := strings.Join(s, "-")

	sockDir := filepath.Join(defaultCNIDir, args.ContainerID)
	if _, err := os.Stat(sockDir); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(sockDir, 0700); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	sockPath := filepath.Join(sockDir, sockRef)

	// ovs-vsctl add-port
	cmd_args := []string{"create", sockPath}
	if output, err := execCommand(defaultOvsScript, cmd_args); err == nil {
		vhostName := strings.Replace(string(output), "\n", "", -1)

		cmd_args = []string{"getmac", vhostName}
		if output, err := execCommand(defaultCNIDir, cmd_args); err == nil {
			data.VhostMac = strings.Replace(string(output), "\n", "", -1)
		}

		data.Vhostname = vhostName
		data.IfMac = generateRandomMacAddress()
	}

	return nil
}

func delLocalDeviceVhost(conf *usrsptypes.NetConf, args *skel.CmdArgs, data *ovsdb.OvsSavedData) error {

	// ovs-vsctl --if-exists del-port
	cmd_args := []string{"delete", data.Vhostname}
	if _, err := execCommand(defaultOvsScript, cmd_args); err == nil {
		path := filepath.Join(defaultCNIDir, args.ContainerID)

		folder, err := os.Open(path)
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

		// Remove files with matching container ID and IF name
		for _, fileName := range filesForContainerID {
			if match, _ := regexp.MatchString(fileBaseName+".*", fileName); match == true {
				file := filepath.Join(path, fileName)
				if err = os.Remove(file); err != nil {
					return err
				}
				numDeletedFiles++
			}
		}
		// Remove folder for container ID if it's empty
		if numDeletedFiles == len(filesForContainerID) {
			if err = os.Remove(path); err != nil {
				return err
			}
		}
	}

	return nil
}
