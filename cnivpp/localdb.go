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
// VPP UserSpace CNI implementation. The input to the library is json
// data defined in pkg/types. If the configuration contains local data,
// the 'api' library is used to send the request to the local govpp-agent,
// which provisions the local VPP instance. If the configuration contains
// remote data, the database library is used to store the data, which is
// later read and processed locally by the remotes agent (usrsp-app running
// in the container)
//

package cnivpp

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/pkg/skel"

	"github.com/intel/userspace-cni-network-plugin/cnivpp/bin_api/interfaces"
	"github.com/intel/userspace-cni-network-plugin/cnivpp/bin_api/l2"
	"github.com/intel/userspace-cni-network-plugin/pkg/annotations"
	"github.com/intel/userspace-cni-network-plugin/pkg/configdata"
	"github.com/intel/userspace-cni-network-plugin/pkg/types"
)

//
// Constants
//
const debugVppDb = false

//
// Types
//

// This structure is a union of all the VPP data (for all types of
// interfaces) that need to be preserved for later use.
type VppSavedData struct {
	l2SwIfIndex        l2.InterfaceIndex         `json:"swIfIndex"`
	interfaceSwIfIndex interfaces.InterfaceIndex `json:"swIfIndex"`     // Software Index, used to access the created interface, needed to delete interface.
	MemifSocketId      uint32                    `json:"memifSocketId"` // Memif SocketId, used to access the created memif Socket File, used for debug only.
}

//
// API Functions
//

// saveVppConfig() - Some data needs to be saved, like the swIfIndex, for cmdDel().
//  This function squirrels the data away to be retrieved later.
func SaveVppConfig(conf *types.NetConf, args *skel.CmdArgs, data *VppSavedData) error {

	// Current implementation is to write data to a file with the name:
	//   /var/run/vpp/cni/data/local-<ContainerId:12>-<IfName>.json

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

		if debugVppDb {
			fmt.Printf("SAVE FILE: swIfIndex=%d path=%s dataBytes=%s\n", data.interfaceSwIfIndex, path, dataBytes)
		}
		return ioutil.WriteFile(path, dataBytes, 0644)
	} else {
		return fmt.Errorf("ERROR: serializing delegate VPP saved data: %v", err)
	}
}

func LoadVppConfig(conf *types.NetConf, args *skel.CmdArgs, data *VppSavedData) error {

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
