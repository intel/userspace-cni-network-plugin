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
// This module provides the database library functions. Initial implementaion
// copies the json data to a known file location.
//

package cniovs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/pkg/skel"

	"github.com/intel/userspace-cni-network-plugin/pkg/annotations"
	"github.com/intel/userspace-cni-network-plugin/pkg/configdata"
	"github.com/intel/userspace-cni-network-plugin/pkg/types"
)

//
// Constants
//

//
// Types
//

// This structure is a union of all the OVS data (for all types of
// interfaces) that need to be preserved for later use.
type OvsSavedData struct {
	Vhostname string `json:"vhostname"` // Vhost Port name
	VhostMac  string `json:"vhostmac"`  // Vhost port MAC address
	IfMac     string `json:"ifmac"`     // Interface Mac address
}

//
// API Functions
//

// SaveConfig() - Some data needs to be saved for cmdDel().
//  This function squirrels the data away to be retrieved later.
func SaveConfig(conf *types.NetConf, args *skel.CmdArgs, data *OvsSavedData) error {

	// Current implementation is to write data to a file with the name:
	//   /var/run/ovs/cni/data/local-<ContainerId:12>-<IfName>.json

	fileName := fmt.Sprintf("local-%s-%s.json", args.ContainerID[:12], args.IfName)
	if dataBytes, err := json.Marshal(data); err == nil {
		localDir := annotations.DefaultLocalCNIDir

		if _, err := os.Stat(localDir); err != nil {
			if os.IsNotExist(err) {
				if err := os.MkdirAll(localDir, 0700); err != nil {
					return err
				}
			} else {
				return err
			}
		}

		path := filepath.Join(localDir, fileName)

		return ioutil.WriteFile(path, dataBytes, 0644)
	} else {
		return fmt.Errorf("ERROR: serializing delegate VPP saved data: %v", err)
	}
}

func LoadConfig(conf *types.NetConf, args *skel.CmdArgs, data *OvsSavedData) error {

	fileName := fmt.Sprintf("local-%s-%s.json", args.ContainerID[:12], args.IfName)
	localDir := annotations.DefaultLocalCNIDir
	path := filepath.Join(localDir, fileName)

	if _, err := os.Stat(path); err == nil {
		if dataBytes, err := ioutil.ReadFile(path); err == nil {
			if err = json.Unmarshal(dataBytes, data); err != nil {
				return fmt.Errorf("ERROR: Failed to parse VPP saved data: %v", err)
			}
		} else {
			return fmt.Errorf("ERROR: Failed to read VPP saved data: %v", err)
		}

	} else {
		path = ""
	}

	// Delete file (and directory if empty)
	configdata.FileCleanup(localDir, path)

	return nil
}
