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
	"fmt"
	"io"
	"os"

	v1 "k8s.io/api/core/v1"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types/current"

	"github.com/intel/userspace-cni-network-plugin/k8sclient"
	"github.com/intel/userspace-cni-network-plugin/usrsptypes"
	"github.com/intel/userspace-cni-network-plugin/annotations"
	"github.com/intel/userspace-cni-network-plugin/logging"
)

//
// Constants
//
const DefaultBaseCNIDir = "/var/lib/cni/usrspcni"
const DefaultLocalCNIDir = "/var/lib/cni/usrspcni/data"
const debugUsrSpDb = false

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
func SaveRemoteConfig(conf *usrsptypes.NetConf,
					  args *skel.CmdArgs,
					  kubeClient k8sclient.KubeClient,
					  sharedDir string,
					  pod *v1.Pod,
					  ipResult *current.Result) (*v1.Pod, error) {
	var err error
	var dataCopy usrsptypes.NetConf
	var configData annotations.ConfigData

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
		if ipResult != nil {
			dataCopy.HostConf.NetType = "interface"
		}
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
	// Write configuration data into annotation to be consumed by container
	//
	logging.Debugf("SaveRemoteConfig(): Store in PodSpec")

	configData.ContainerId = args.ContainerID
	configData.IfName = args.IfName
	configData.NetConf = dataCopy
	if ipResult != nil {
		configData.IPResult = *ipResult
	}

	pod, err = annotations.SetPodAnnotationConfigData(kubeClient, conf.KubeConfig, pod, &configData)
	if err != nil {
		logging.Errorf("SaveRemoteConfig: Error writing annotation configData: %v", err)
		return pod, err
	}

	// Retrieve the mappedSharedDir from the Containers in podSpec. Directory
	// in container Socket Files will be fread from. Write this data back as an
	// annotation so container knows where directory is located.
	mappedSharedDir, err := annotations.GetPodVolumeMountHostMappedSharedDir(pod)
	if err != nil {
		logging.Errorf("SaveRemoteConfig: VolumeMount mappedSharedDir not provided - %v", err)
		return pod, err
	}
	pod, err = annotations.SetPodAnnotationMappedDir(kubeClient, conf.KubeConfig, pod, mappedSharedDir)
	if err != nil {
		logging.Errorf("SaveRemoteConfig: Error writing annotation mappedSharedDir - %v", err)
		return pod, err
	}

	return pod, err
}

// CleanupRemoteConfig() - This function cleans up any remaining files
//   in the passed in directory. Some of these files were used to squirrel
//   data from the create so interface can be deleted properly.
func CleanupRemoteConfig(conf *usrsptypes.NetConf, sharedDir string) {

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
	NetConf   usrsptypes.NetConf
	IPResult  current.Result
}

func GetRemoteConfig() ([]*InterfaceData, string, error) {
	var ifaceList []*InterfaceData

	// Retrieve the directory that is shared between host and container.
	// No conversion necessary
	sharedDir, err := annotations.GetFileAnnotationMappedDir()
	if err != nil {
		return ifaceList, sharedDir, err
	}

	// Retrieve the configuration data for each interface. This is a list of 1 to n interfaces.
	configDataList, err := annotations.GetFileAnnotationConfigData()
	if err != nil {
		return ifaceList, sharedDir, err
	}

	// Convert the data to usrsptypes.NetConf
	for _, configData := range configDataList {
		var ifaceData InterfaceData

		ifaceData.NetConf = configData.NetConf

		ifaceData.Args = skel.CmdArgs{}
		ifaceData.Args.ContainerID = configData.ContainerId
		ifaceData.Args.IfName = configData.IfName

		ifaceData.IPResult = configData.IPResult

		ifaceList = append(ifaceList, &ifaceData)
	}


	return ifaceList, sharedDir, err
}
