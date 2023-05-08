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
// UserSpace CNI DB implementation. In this context, the DB is the
// configuration being passed to the container.
//

package configdata

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/containernetworking/cni/pkg/skel"
	current "github.com/containernetworking/cni/pkg/types/100"

	"github.com/intel/userspace-cni-network-plugin/pkg/annotations"
	"github.com/intel/userspace-cni-network-plugin/pkg/types"
	"github.com/intel/userspace-cni-network-plugin/logging"
)

//
// Constants
//
const debugUsrSpDb = false

const DefaultOvsCNIDir = "/usr/local/var/run/openvswitch"
const DefaultVppCNIDir = "/var/run/vpp"

//
// Types
//


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
func SaveRemoteConfig(conf *types.NetConf,
					  args *skel.CmdArgs,
					  kubeClient kubernetes.Interface,
					  sharedDir string,
					  pod *v1.Pod,
					  ipResult *current.Result) (*v1.Pod, error) {
	var configData types.ConfigurationData
	var err error

	// Check if required parameters are set and fail otherwise
	if conf == nil {
		return pod, logging.Errorf("SaveRemoteConfig(): Error conf is set to: %v", conf)
	}
	if args == nil {
		return pod, logging.Errorf("SaveRemoteConfig(): Error args is set to: %v", args)
	}
	if pod == nil {
		return pod, logging.Errorf("SaveRemoteConfig(): Error pod is set to: %v", pod)
	}
	//
	// Convert Local Data to types.ConfigurationData, which
	// will be written to the container.
	//
	configData.ContainerId = args.ContainerID
	configData.IfName = args.IfName
	configData.Name = conf.Name
	configData.Config = conf.ContainerConf

	if ipResult != nil {
		configData.IPResult = *ipResult
	}

	// Convert empty variables to valid data based on the original HostConf
	if configData.Config.IfType == "" {
		configData.Config.IfType = conf.HostConf.IfType
	}
	if configData.Config.NetType == "" {
		if ipResult != nil {
			configData.Config.NetType = "interface"
		}
	}

	if configData.Config.IfType == "memif" {
		if configData.Config.MemifConf.Role == "" {
			if conf.HostConf.MemifConf.Role == "master" {
				configData.Config.MemifConf.Role = "slave"
			} else {
				configData.Config.MemifConf.Role = "master"
			}
		}
		if configData.Config.MemifConf.Mode == "" {
			configData.Config.MemifConf.Mode = conf.HostConf.MemifConf.Mode
		}
		configData.Config.MemifConf.Socketfile = conf.HostConf.MemifConf.Socketfile
	} else if configData.Config.IfType == "vhostuser" {
		if configData.Config.VhostConf.Mode == "" {
			if conf.HostConf.VhostConf.Mode == "client" {
				configData.Config.VhostConf.Mode = "server"
			} else {
				configData.Config.VhostConf.Mode = "client"
			}
		}
		configData.Config.VhostConf.Socketfile = conf.HostConf.VhostConf.Socketfile
	}

	//
	// Write configuration data that will be consumed by container
	//
	if kubeClient != nil {
		//
		// Write configuration data into annotation
		//
		logging.Debugf("SaveRemoteConfig(): Store in PodSpec")

		pod, err = annotations.WritePodAnnotation(kubeClient, pod, &configData)
	} else {
		//
		// Write configuration data into file
		//

		// Make sure directory exists
		if _, err = os.Stat(sharedDir); err != nil {
			if os.IsNotExist(err) {
				if err := os.MkdirAll(sharedDir, 0700); err != nil {
					return pod, err
				}
			} else {
				return pod, err
			}
		}

		fileName := fmt.Sprintf("configData-%s-%s.json", args.ContainerID[:12], args.IfName)
		path := filepath.Join(sharedDir, fileName)

		dataBytes, jsonErr := json.Marshal(configData)
		if jsonErr == nil {
			err = ioutil.WriteFile(path, dataBytes, 0644)
		} else {
			return pod, fmt.Errorf("ERROR: serializing REMOTE NetConf data: %v", err)
		}
	}

	return pod, err
}

// CleanupRemoteConfig() - This function cleans up any remaining files
//   in the passed in directory. Some of these files were used to squirrel
//   data from the create so interface can be deleted properly.
//
// FIXME: parameter *conf* is not used. It shall be used or removed.
func CleanupRemoteConfig(conf *types.NetConf, sharedDir string) {

	if err := os.RemoveAll(sharedDir); err != nil {
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


type InterfaceData struct {
	Args      skel.CmdArgs
	NetConf   types.NetConf
	IPResult  current.Result
}

func GetRemoteConfig(annotFile string) ([]*InterfaceData, string, error) {
	var ifaceList []*InterfaceData

	// Retrieve the directory that is shared between host and container.
	// No conversion necessary
	mappedDir, err := annotations.GetFileAnnotationMappedDir(annotFile)
	if err != nil {
		return ifaceList, mappedDir, err
	}

	// Retrieve the configuration data for each interface. This is a list of 1 to n interfaces.
	configDataList, err := annotations.GetFileAnnotationConfigData(annotFile)
	if err != nil {
		// If annotation is not found, need to see if data was written
		// to a file.

		// BILLY: Pickup here.
		return ifaceList, mappedDir, err
	}

	// Convert the data to types.NetConf
	for _, configData := range configDataList {
		var ifaceData InterfaceData

		ifaceData.NetConf = types.NetConf{}
		ifaceData.NetConf.Name = configData.Name
		ifaceData.NetConf.HostConf = configData.Config

		ifaceData.Args = skel.CmdArgs{}
		ifaceData.Args.ContainerID = configData.ContainerId
		ifaceData.Args.IfName = configData.IfName

		ifaceData.IPResult = configData.IPResult

		ifaceList = append(ifaceList, &ifaceData)
	}


	return ifaceList, mappedDir, err
}
