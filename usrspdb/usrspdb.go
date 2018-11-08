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
// UserSpace CNI DB implementation. In this context, the DB is the
// configuration being passed to the container.
//

package usrspdb

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types/current"

	"github.com/intel/userspace-cni-network-plugin/usrsptypes"
)

//
// Constants
//
const DefaultBaseCNIDir = "/var/lib/cni/usrspcni"
const DefaultLocalCNIDir = "/var/lib/cni/usrspcni/data"
const DefaultSocketDir = "/var/lib/cni/usrspcni/shared"
const debugUsrSpDb = true

//
// Types
//

// This structure is used to pass additional data outside of the usrsptypes data into the container.
type additionalData struct {
	ContainerId string         `json:"containerId"` // ContainerId used locally. Used in several place, namely in the socket filenames.
	IfName      string         `json:"ifName"`      // IfName used locally. Used in several place, namely in the socket filenames.
	IPResult    current.Result `json:"ipResult"`    // Data structure returned from IPAM plugin.
}

//
// API Functions
//

//
// Functions for processing Remote Configs (configs for within a Container)
//

// saveRemoteConfig() - When a config read on the host is for a Container,
//      flip the location and write the data to a file. When the Container
//      comes up, it will read the file via () and delete the file. This function
//      writes the file.
func SaveRemoteConfig(conf *usrsptypes.NetConf, ipResult *current.Result, args *skel.CmdArgs) error {

	var dataCopy usrsptypes.NetConf
	var addData additionalData

	// Current implementation is to write data to a file with the name:
	//   <DefaultBaseCNIDir>/<ContainerId>/remote-<IfName>.json
	//   <DefaultBaseCNIDir>/<ContainerId>/addData-<IfName>.json

	sockDir := filepath.Join(DefaultBaseCNIDir, args.ContainerID)

	if _, err := os.Stat(sockDir); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(sockDir, 0700); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	//
	// Convert the remote configuration into a local configuration
	//
	dataCopy = *conf
	dataCopy.HostConf = dataCopy.ContainerConf
	dataCopy.ContainerConf = usrsptypes.UserSpaceConf{}

	// IPAM is processed by the host and sent to the Container. So blank out what was already processed.
	dataCopy.IPAM.Type = ""

	// Convert empty variables to valid data based on the original HostConf
	if dataCopy.HostConf.Engine == "" {
		dataCopy.HostConf.Engine = conf.HostConf.Engine
	}
	if dataCopy.HostConf.IfType == "" {
		dataCopy.HostConf.IfType = conf.HostConf.IfType
	}
	if dataCopy.HostConf.NetType == "" {
		dataCopy.HostConf.NetType = "interface"
	}

	if dataCopy.HostConf.IfType == "memif" {
		if dataCopy.HostConf.MemifConf.Role == "" {
			if conf.HostConf.MemifConf.Role == "master" {
				dataCopy.HostConf.MemifConf.Role = "slave"
			} else {
				dataCopy.HostConf.MemifConf.Role = "master"
			}
		}
		if dataCopy.HostConf.MemifConf.Mode == "" {
			dataCopy.HostConf.MemifConf.Mode = conf.HostConf.MemifConf.Mode
		}
	} else if dataCopy.HostConf.IfType == "vhostuser" {
		if dataCopy.HostConf.VhostConf.Mode == "" {
			if conf.HostConf.VhostConf.Mode == "client" {
				dataCopy.HostConf.VhostConf.Mode = "server"
			} else {
				dataCopy.HostConf.VhostConf.Mode = "client"
			}
		}
	}

	//
	// Gather the additional data
	//
	addData.ContainerId = args.ContainerID
	addData.IfName = args.IfName
	addData.IPResult = *ipResult

	//
	// Marshall data and write to file
	//
	fileName := fmt.Sprintf("remote-%s.json", args.IfName)
	path := filepath.Join(sockDir, fileName)

	dataBytes, err := json.Marshal(dataCopy)

	if err == nil {
		if debugUsrSpDb {
			fmt.Printf("SAVE FILE: path=%s dataBytes=%s", path, dataBytes)
		}
		err = ioutil.WriteFile(path, dataBytes, 0644)
	} else {
		return fmt.Errorf("ERROR: serializing REMOTE NetConf data: %v", err)
	}

	if err == nil {
		fileName = fmt.Sprintf("addData-%s.json", args.IfName)
		path = filepath.Join(sockDir, fileName)

		dataBytes, err = json.Marshal(addData)

		if err == nil {
			if debugUsrSpDb {
				fmt.Printf("SAVE FILE: path=%s dataBytes=%s", path, dataBytes)
			}
			err = ioutil.WriteFile(path, dataBytes, 0644)
		} else {
			return fmt.Errorf("ERROR: serializing ADDDATA NetConf data: %v", err)
		}
	}

	return err
}

// CleanupRemoteConfig() - When a config read on the host is for a Container,
//      the data to a file. This function cleans up the remaining files.
func CleanupRemoteConfig(conf *usrsptypes.NetConf, containerID string) {

	// Current implementation is to write data to a file with the name:
	//   /var/run/vpp/cni/<ContainerId>/remote-<IfName>.json

	sockDir := filepath.Join(DefaultBaseCNIDir, containerID)

	if err := os.RemoveAll(sockDir); err != nil {
		fmt.Println(err)
	}
}

//
// Utility Functions
//

// This function deletes the input file (if provided) and the associated
// directory (if provided) if the directory is empty.
//  directory string - Directory file is located in, Use "" if directory
//    should remain unchanged.
//  filepath string - File (including directory) to be deleted. Use "" if
//    only the directory should be deleted.
func FileCleanup(directory string, filepath string) (err error) {

	// If File is provided, delete it.
	if filepath != "" {
		err = os.Remove(filepath)
		if err != nil {
			return fmt.Errorf("ERROR: Failed to delete file: %v", err)
		}
	}

	// If Directory is provided and it is empty, delete it.
	if directory != "" {
		f, dirErr := os.Open(directory)
		if dirErr == nil {
			_, dirErr = f.Readdir(1)
			if dirErr == io.EOF {
				err = os.Remove(directory)
			}
		}
		f.Close()
	}

	return
}

func FindRemoteConfig() (bool, usrsptypes.NetConf, current.Result, skel.CmdArgs, error) {
	var conf usrsptypes.NetConf
	var addData additionalData

	args := skel.CmdArgs{}

	//
	// Find Primary input file
	//
	found, dataBytes, err := findFile(filepath.Join(DefaultLocalCNIDir, "remote-*.json"))

	if err == nil {
		if found {
			if err = json.Unmarshal(dataBytes, &conf); err != nil {
				return found, conf, addData.IPResult, args, fmt.Errorf("failed to parse Remote config: %v", err)
			}

			//
			// Since Primary input was found, look for Additional Data file.
			//
			found, dataBytes, err = findFile(filepath.Join(DefaultLocalCNIDir, "addData-*.json"))
			if err == nil {
				if found {
					if err = json.Unmarshal(dataBytes, &addData); err != nil {
						return found, conf, addData.IPResult, args, fmt.Errorf("failed to parse AddData config: %v", err)
					}
				} else {
					return found, conf, addData.IPResult, args, fmt.Errorf("failed to read AddData config: %v", err)
				}
			}
		} else {
			return found, conf, addData.IPResult, args, fmt.Errorf("failed to read Remote config: %v", err)
		}
	}

	args.ContainerID = addData.ContainerId
	args.IfName = addData.IfName

	return found, conf, addData.IPResult, args, err
}

func findFile(filePath string) (bool, []byte, error) {
	var found bool = false

	if debugUsrSpDb {
		fmt.Println(filePath)
	}
	matches, err := filepath.Glob(filePath)

	if err != nil {
		if debugUsrSpDb {
			fmt.Println(err)
		}
		return found, nil, err
	}

	if debugUsrSpDb {
		fmt.Println(matches)
	}

	for i := range matches {
		if debugUsrSpDb {
			fmt.Printf("PROCESSING FILE: path=%s\n", matches[i])
		}

		found = true

		if dataBytes, err := ioutil.ReadFile(matches[i]); err == nil {
			if debugUsrSpDb {
				fmt.Printf("FILE DATA:\n%s\n", dataBytes)
			}

			// Delete file (and directory if empty)
			FileCleanup("", matches[i])

			return found, dataBytes, err
		} else {
			return found, nil, fmt.Errorf("failed to read Remote config: %v", err)
		}
	}

	return found, nil, err
}
