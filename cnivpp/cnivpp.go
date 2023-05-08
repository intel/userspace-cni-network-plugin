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
// later read and processed locally by the remotes agent (usrapp-app running
// in the container)
//

package cnivpp

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/containernetworking/cni/pkg/skel"
	current "github.com/containernetworking/cni/pkg/types/100"

	"github.com/intel/userspace-cni-network-plugin/cnivpp/api/bridge"
	"github.com/intel/userspace-cni-network-plugin/cnivpp/api/infra"
	"github.com/intel/userspace-cni-network-plugin/cnivpp/api/interface"
	"github.com/intel/userspace-cni-network-plugin/cnivpp/api/memif"
	"github.com/intel/userspace-cni-network-plugin/cnivpp/bin_api/memif"
	"github.com/intel/userspace-cni-network-plugin/cnivpp/bin_api/interface_types"
	"github.com/intel/userspace-cni-network-plugin/logging"
	"github.com/intel/userspace-cni-network-plugin/pkg/configdata"
	"github.com/intel/userspace-cni-network-plugin/pkg/types"
)

// Constants
const (
	dbgBridge    = false
	dbgInterface = false
)

// Types
type CniVpp struct {
}

// API Functions
func (cniVpp CniVpp) AddOnHost(conf *types.NetConf,
	args *skel.CmdArgs,
	kubeClient kubernetes.Interface,
	sharedDir string,
	ipResult *current.Result) error {
	var vppCh vppinfra.ConnectionData
	var err error
	var data VppSavedData

	logging.Infof("VPP AddOnHost: ENTER - Container %s Iface %s", args.ContainerID[:12], args.IfName)

	// Create Channel to pass requests to VPP
	vppCh, err = vppinfra.VppOpenCh()
	if err != nil {
		return err
	}
	defer vppinfra.VppCloseCh(vppCh)

	//
	// Create Local Interface
	//
	if conf.HostConf.IfType == "memif" {
		err = addLocalDeviceMemif(vppCh, conf, args, sharedDir, &data)
	} else {
		err = errors.New("ERROR: Unknown HostConf.IfType:" + conf.HostConf.IfType)
	}

	if err != nil {
		return err
	}

	//
	// Set interface to up (1)
	//
	err = vppinterface.SetState(vppCh.Ch, data.interfaceSwIfIndex, 1)
	if err != nil {
		logging.Debugf("AddOnHost(vpp): Error bringing interface UP: %v", err)
		return err
	}

	//
	// Add Interface to Local Network
	//

	// Add L2 Network if supplied
	if conf.HostConf.NetType == "bridge" {

		var bridgeDomain uint32

		// Check if DEPRECATED Attribute is being used.
		if conf.HostConf.BridgeConf.BridgeId != 0 {
			bridgeDomain = uint32(conf.HostConf.BridgeConf.BridgeId)
		}

		// Determine if BridgeName was entered
		if conf.HostConf.BridgeConf.BridgeName != "" {
			var tmpBridgeDomain uint64
			tmpBridgeDomain, err = strconv.ParseUint(conf.HostConf.BridgeConf.BridgeName, 10, 32)
			bridgeDomain = uint32(tmpBridgeDomain)
			if err != nil {
				logging.Debugf("AddOnHost(vpp): Error - VPP BridgeName not an ID: %v", err)
				return err
			}
		}

		// Add Interface to Bridge. If Bridge does not exist, AddBridgeInterface()
		// will create.
		err = vppbridge.AddBridgeInterface(vppCh.Ch, bridgeDomain, interface_types.InterfaceIndex(data.interfaceSwIfIndex))
		if err != nil {
			logging.Debugf("AddOnHost(vpp): Error adding interface to bridge: %v", err)
			return err
		} else {
			if dbgBridge {
				logging.Debugf("INTERFACE %d added to BRIDGE %d\n", data.interfaceSwIfIndex, bridgeDomain)
				vppbridge.DumpBridge(vppCh.Ch, bridgeDomain)
			}
		}
		// Add L3 Network if supplied
	} else if conf.HostConf.NetType == "interface" {
		if ipResult != nil && len(ipResult.IPs) != 0 {
			err = vppinterface.AddDelIpAddress(vppCh.Ch, data.interfaceSwIfIndex, true, ipResult)
			if err != nil {
				logging.Debugf("AddOnHost(vpp): Error adding IP: %v", err)
				return err
			}
		}
	} else if conf.HostConf.NetType != "" {
		err = errors.New("ERROR: Unknown HostConf.NetType:" + conf.HostConf.NetType)
		logging.Debugf("AddOnHost(vpp): %v", err)
		return err
	}

	//
	// Save Create Data for Delete
	//
	err = SaveVppConfig(conf, args, &data)

	if err != nil {
		return err
	}

	return err
}

func (cniVpp CniVpp) AddOnContainer(conf *types.NetConf,
	args *skel.CmdArgs,
	kubeClient kubernetes.Interface,
	sharedDir string,
	pod *v1.Pod,
	ipResult *current.Result) (*v1.Pod, error) {
	logging.Infof("VPP AddOnContainer: ENTER - Container %s Iface %s", args.ContainerID[:12], args.IfName)
	return configdata.SaveRemoteConfig(conf, args, kubeClient, sharedDir, pod, ipResult)
}

func (cniVpp CniVpp) DelFromHost(conf *types.NetConf, args *skel.CmdArgs, sharedDir string) error {
	var vppCh vppinfra.ConnectionData
	var data VppSavedData
	var err error

	logging.Infof("VPP DelFromHost: ENTER - Container %s Iface %s", args.ContainerID[:12], args.IfName)

	// Create Channel to pass requests to VPP
	vppCh, err = vppinfra.VppOpenCh()
	if err != nil {
		return err
	}
	defer vppinfra.VppCloseCh(vppCh)

	// Retrieved squirreled away data needed for processing delete
	err = LoadVppConfig(conf, args, &data)

	if err != nil {
		return err
	}

	//
	// Remove L2 Network if supplied
	//
	if conf.HostConf.NetType == "bridge" {

		// Validate and convert input data
		var bridgeDomain uint32 = uint32(conf.HostConf.BridgeConf.BridgeId)

		if dbgBridge {
			logging.Verbosef("INTERFACE %d retrieved from CONF - attempt to DELETE Bridge %d\n", data.interfaceSwIfIndex, bridgeDomain)
		}

		// Remove MemIf from Bridge. RemoveBridgeInterface() will delete Bridge if
		// no more interfaces are associated with the Bridge.
		err = vppbridge.RemoveBridgeInterface(vppCh.Ch, bridgeDomain, interface_types.InterfaceIndex(data.interfaceSwIfIndex))

		if err != nil {
			logging.Debugf("DelFromHost(vpp): Error removing interface from bridge: %v", err)
			return err
		} else {
			if dbgBridge {
				logging.Verbosef("INTERFACE %d removed from BRIDGE %d\n", data.interfaceSwIfIndex, bridgeDomain)
				vppbridge.DumpBridge(vppCh.Ch, bridgeDomain)
			}
		}
	}

	//
	// Delete Local Interface
	//
	if conf.HostConf.IfType == "memif" {
		return delLocalDeviceMemif(vppCh, conf, args, sharedDir, &data)
	} else if conf.HostConf.IfType == "vhostuser" {
		return fmt.Errorf("GOOD: Found HostConf.Type:" + conf.HostConf.IfType)
	} else {
		return fmt.Errorf("ERROR: Unknown HostConf.Type:" + conf.HostConf.IfType)
	}

	return err
}

func (cniVpp CniVpp) DelFromContainer(conf *types.NetConf, args *skel.CmdArgs, sharedDir string, pod *v1.Pod) error {
	logging.Infof("VPP DelFromContainer: ENTER - Container %s Iface %s", args.ContainerID[:12], args.IfName)

	configdata.FileCleanup(sharedDir, "")

	return nil
}

// Local Functions
func getMemifSocketfileName(conf *types.NetConf,
	sharedDir string,
	containerID string,
	ifName string) string {
	if conf.HostConf.MemifConf.Socketfile == "" {
		conf.HostConf.MemifConf.Socketfile = fmt.Sprintf("memif-%s-%s.sock", containerID[:12], ifName)
	}
	return filepath.Join(sharedDir, conf.HostConf.MemifConf.Socketfile)
}

func addLocalDeviceMemif(vppCh vppinfra.ConnectionData,
	conf *types.NetConf,
	args *skel.CmdArgs,
	sharedDir string,
	data *VppSavedData) (err error) {
	// Validate and convert input data
	var memifRole vppmemif.MemifRole
	var memifMode vppmemif.MemifMode

	// Retrieve the Socketfile path
	memifSocketPath := getMemifSocketfileName(conf, sharedDir, args.ContainerID, args.IfName)

	// Apply default values to input configuration
	if conf.HostConf.MemifConf.Role == "master" {
		memifRole = vppmemif.RoleMaster
	} else if conf.HostConf.MemifConf.Role == "slave" {
		memifRole = vppmemif.RoleSlave
	} else {
		return fmt.Errorf("ERROR: Invalid MEMIF Role:" + conf.HostConf.MemifConf.Role)
	}

	if conf.HostConf.MemifConf.Mode == "" {
		conf.HostConf.MemifConf.Mode = "ethernet"
	}
	if conf.HostConf.MemifConf.Mode == "ethernet" {
		memifMode = vppmemif.ModeEthernet
	} else if conf.HostConf.MemifConf.Mode == "ip" {
		memifMode = vppmemif.ModeIP
	} else if conf.HostConf.MemifConf.Mode == "inject-punt" {
		memifMode = vppmemif.ModePuntInject
	} else {
		return fmt.Errorf("ERROR: Invalid MEMIF Mode:" + conf.HostConf.MemifConf.Mode)
	}

	// Create Memif Socket
	data.MemifSocketId, err = vppmemif.CreateMemifSocket(vppCh.Ch, memifSocketPath)
	if err != nil {
		logging.Debugf("addLocalDeviceMemif(vpp): Error creating memif socket: %v", err)
		return
	} else {
		if dbgInterface {
			logging.Verbosef("MEMIF SOCKET", data.MemifSocketId, memifSocketPath, "created")
			vppmemif.DumpMemifSocket(vppCh.Ch)
		}
	}

	// Create MemIf Interface
	data.interfaceSwIfIndex, err = vppmemif.CreateMemifInterface(vppCh.Ch, data.MemifSocketId, memif.MemifRole(memifRole), memif.MemifMode(memifMode))
	if err != nil {
		logging.Debugf("addLocalDeviceMemif(vpp): Error creating memif inteface: %v", err)
		return
	} else {
		if dbgInterface {
			logging.Verbosef("MEMIF", data.interfaceSwIfIndex, "created", args.IfName)
			vppmemif.DumpMemif(vppCh.Ch)
		}
	}

	return
}

func delLocalDeviceMemif(vppCh vppinfra.ConnectionData, conf *types.NetConf, args *skel.CmdArgs, sharedDir string, data *VppSavedData) (err error) {
	// Retrieve the Socketfile name
	memifSocketPath := getMemifSocketfileName(conf, sharedDir, args.ContainerID, args.IfName)

	// Delete the memif interface
	err = vppmemif.DeleteMemifInterface(vppCh.Ch, interface_types.InterfaceIndex(data.interfaceSwIfIndex))
	if err != nil {
		logging.Debugf("delLocalDeviceMemif(vpp): Error deleting memif inteface: %v", err)
		return
	} else {
		if dbgInterface {
			logging.Verbosef("INTERFACE %d deleted\n", data.interfaceSwIfIndex)
			vppmemif.DumpMemif(vppCh.Ch)
			vppmemif.DumpMemifSocket(vppCh.Ch)
		}
	}

	// Remove socketfile
	err = configdata.FileCleanup("", memifSocketPath)

	return
}
