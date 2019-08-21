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

	v1 "k8s.io/api/core/v1"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types/current"

	"github.com/intel/userspace-cni-network-plugin/k8sclient"
	"github.com/intel/userspace-cni-network-plugin/pkg/types"
	"github.com/intel/userspace-cni-network-plugin/annotations"
	"github.com/intel/userspace-cni-network-plugin/logging"
)

//
// Constants
//
const DefaultBaseCNIDir = "/var/lib/cni/usrspcni"
const DefaultLocalCNIDir = "/var/lib/cni/usrspcni/data"
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
					  kubeClient k8sclient.KubeClient,
					  sharedDir string,
					  pod *v1.Pod,
					  ipResult *current.Result) (*v1.Pod, error) {
	var configData types.ConfigurationData

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

	return WriteRemoteConfig(kubeClient, pod, &configData, sharedDir, args.ContainerID, args.IfName)
}

func WriteRemoteConfig(kubeClient k8sclient.KubeClient,
					   pod *v1.Pod,
					   configData *types.ConfigurationData,
					   sharedDir string,
					   containerID string,
					   ifName string) (*v1.Pod, error) {
	var err error
	var modifiedConfig bool
	var modifiedMappedDir bool

	//
	// Write configuration data that will be consumed by container
	//
	if kubeClient != nil {
		//
		// Write configuration data into annotation
		//
		logging.Debugf("SaveRemoteConfig(): Store in PodSpec")

		modifiedConfig, err = annotations.SetPodAnnotationConfigData(pod, configData)
		if err != nil {
			logging.Errorf("SaveRemoteConfig: Error formatting annotation configData: %v", err)
			return pod, err
		}

		// Retrieve the mappedSharedDir from the Containers in podSpec. Directory
		// in container Socket Files will be read from. Write this data back as an
		// annotation so container knows where directory is located.
		mappedSharedDir, err := annotations.GetPodVolumeMountHostMappedSharedDir(pod)
		if err != nil {
			mappedSharedDir = DefaultBaseCNIDir
			logging.Warningf("SaveRemoteConfig: Error reading VolumeMount: %v", err)
			logging.Warningf("SaveRemoteConfig: VolumeMount \"shared-dir\" not provided, defaulting to: %s", mappedSharedDir)
			err = nil
		}
		modifiedMappedDir, err = annotations.SetPodAnnotationMappedDir(pod, mappedSharedDir)
		if err != nil {
			logging.Errorf("SaveRemoteConfig: Error formatting annotation mappedSharedDir - %v", err)
			return pod, err
		}

		if modifiedConfig == true || modifiedMappedDir == true {
			pod, err = annotations.WritePodAnnotation(kubeClient, pod)
			if err != nil {
				logging.Errorf("SaveRemoteConfig: Error writing annotations - %v", err)
				return pod, err
			}
		}
	} else {
		//
		// Write configuration data into file
		//

		// Make sure directory exists
		if _, err := os.Stat(sharedDir); err != nil {
			if os.IsNotExist(err) {
				if err := os.MkdirAll(sharedDir, 0700); err != nil {
					return pod, err
				}
			} else {
				return pod, err
			}
		}

		fileName := fmt.Sprintf("configData-%s-%s.json", containerID[:12], ifName)
		path := filepath.Join(sharedDir, fileName)

		dataBytes, err := json.Marshal(configData)
		if err == nil {
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

func GetRemoteConfig() ([]*InterfaceData, string, error) {
	var ifaceList []*InterfaceData

	// Retrieve the directory that is shared between host and container.
	// No conversion necessary
	mappedDir, err := annotations.GetFileAnnotationMappedDir()
	if err != nil {
		return ifaceList, mappedDir, err
	}

	// Retrieve the configuration data for each interface. This is a list of 1 to n interfaces.
	configDataList, err := annotations.GetFileAnnotationConfigData()
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
