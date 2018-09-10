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

package ovsdb

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
const defaultBaseCNIDir = "/var/run/ovs/cni"
const defaultLocalCNIDir = "/var/run/ovs/cni/data"

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

// This structure is used to pass additional data outside of the usrsptypes date into the container.
type additionalData struct {
	ContainerId string         `json:"containerId"` // ContainerId used locally. Used in several place, namely in the socket filenames.
	IPResult    current.Result `json:"ipResult"`    // Data structure returned from IPAM plugin.
}

//
// API Functions
//

// SaveConfig() - Some data needs to be saved for cmdDel().
//  This function squirrels the data away to be retrieved later.
func SaveConfig(conf *usrsptypes.NetConf, args *skel.CmdArgs, data *OvsSavedData) error {

	// Current implementation is to write data to a file with the name:
	//   /var/run/ovs/cni/data/local-<ContainerId:12>-<IfName>.json

	fileName := fmt.Sprintf("local-%s-%s.json", args.ContainerID[:12], args.IfName)
	if dataBytes, err := json.Marshal(data); err == nil {
		sockDir := defaultLocalCNIDir
		// OLD: sockDir := filepath.Join(defaultCNIDir, args.ContainerID)

		if _, err := os.Stat(sockDir); err != nil {
			if os.IsNotExist(err) {
				if err := os.MkdirAll(sockDir, 0700); err != nil {
					return err
				}
			} else {
				return err
			}
		}

		path := filepath.Join(sockDir, fileName)

		return ioutil.WriteFile(path, dataBytes, 0644)
	} else {
		return fmt.Errorf("ERROR: serializing delegate VPP saved data: %v", err)
	}
}

func LoadConfig(conf *usrsptypes.NetConf, args *skel.CmdArgs, data *OvsSavedData) error {

	fileName := fmt.Sprintf("local-%s-%s.json", args.ContainerID[:12], args.IfName)
	sockDir := defaultLocalCNIDir
	path := filepath.Join(sockDir, fileName)

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
	fileCleanup(sockDir, path)

	return nil
}

// This function deletes the input file (if provided) and the associated
// directory (if provided) if the directory is empty.
//  directory string - Directory file is located in, Use "" if directory
//    should remain unchanged.
//  filepath string - File (including directory) to be deleted. Use "" if
//    only the directory should be deleted.
func fileCleanup(directory string, filepath string) (err error) {

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
