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
// data defined in usrsptypes. If the configuration contains local data,
// the 'api' library is used to send the request to the local govpp-agent,
// which provisions the local VPP instance. If the configuration contains
// remote data, the database library is used to store the data, which is
// later read and processed locally by the remotes agent (usrapp-app running
// in the container)
//

package cnivpp

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types/current"

	"github.com/intel/userspace-cni-network-plugin/cnivpp/api/bridge"
	"github.com/intel/userspace-cni-network-plugin/cnivpp/api/infra"
	"github.com/intel/userspace-cni-network-plugin/cnivpp/api/interface"
	"github.com/intel/userspace-cni-network-plugin/cnivpp/api/memif"
	_ "github.com/intel/userspace-cni-network-plugin/cnivpp/api/vhostuser"
	"github.com/intel/userspace-cni-network-plugin/cnivpp/vppdb"
	"github.com/intel/userspace-cni-network-plugin/logging"
	"github.com/intel/userspace-cni-network-plugin/usrspdb"
	"github.com/intel/userspace-cni-network-plugin/usrsptypes"
)

//
// Constants
//
const (
	dbgBridge    = false
	dbgInterface = false
)

//
// Types
//
type CniVpp struct {
}

//
// API Functions
//
func (cniVpp CniVpp) AddOnHost(conf *usrsptypes.NetConf, args *skel.CmdArgs, ipResult *current.Result) error {
	var vppCh vppinfra.ConnectionData
	var err error
	var data vppdb.VppSavedData

	logging.Debugf("VPP AddOnHost: ENTER")

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
		err = addLocalDeviceMemif(vppCh, conf, args, &data)
	} else if conf.HostConf.IfType == "vhostuser" {
		err = fmt.Errorf("GOOD: Found HostConf.IfType:" + conf.HostConf.IfType)
	} else {
		err = fmt.Errorf("ERROR: Unknown HostConf.IfType:" + conf.HostConf.IfType)
	}

	if err != nil {
		return err
	}

	//
	// Set interface to up (1)
	//
	err = vppinterface.SetState(vppCh.Ch, data.SwIfIndex, 1)
	if err != nil {
		if dbgInterface {
			fmt.Println("Error bringing interface UP:", err)
		}
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
				if dbgBridge {
					fmt.Println("Error - VPP BridgeName not an ID: ", err)
				}
				return err
			}
		}

		// Add Interface to Bridge. If Bridge does not exist, AddBridgeInterface()
		// will create.
		err = vppbridge.AddBridgeInterface(vppCh.Ch, bridgeDomain, data.SwIfIndex)
		if err != nil {
			if dbgBridge {
				fmt.Println("Error:", err)
			}
			return err
		} else {
			if dbgBridge {
				fmt.Printf("INTERFACE %d added to BRIDGE %d\n", data.SwIfIndex, bridgeDomain)
				vppbridge.DumpBridge(vppCh.Ch, bridgeDomain)
			}
		}
		// Add L3 Network if supplied
	} else if conf.HostConf.NetType == "interface" {
		if len(ipResult.IPs) != 0 {
			err = vppinterface.AddDelIpAddress(vppCh.Ch, data.SwIfIndex, 1, ipResult)
			if err != nil {
				if dbgInterface {
					fmt.Println("Error:", err)
				}
				return err
			}
		}
	}

	//
	// Save Create Data for Delete
	//
	err = vppdb.SaveVppConfig(conf, args, &data)

	if err != nil {
		return err
	}

	return err
}

func (cniVpp CniVpp) AddOnContainer(conf *usrsptypes.NetConf, args *skel.CmdArgs, ipResult *current.Result) error {
	logging.Debugf("VPP AddOnContainer: ENTER")
	return usrspdb.SaveRemoteConfig(conf, ipResult, args)
}

func (cniVpp CniVpp) DelFromHost(conf *usrsptypes.NetConf, args *skel.CmdArgs) error {
	var vppCh vppinfra.ConnectionData
	var data vppdb.VppSavedData
	var err error

	logging.Debugf("VPP DelFromHost: ENTER")

	// Create Channel to pass requests to VPP
	vppCh, err = vppinfra.VppOpenCh()
	if err != nil {
		return err
	}
	defer vppinfra.VppCloseCh(vppCh)

	// Retrieved squirreled away data needed for processing delete
	err = vppdb.LoadVppConfig(conf, args, &data)

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
			fmt.Printf("INTERFACE %d retrieved from CONF - attempt to DELETE Bridge %d\n", data.SwIfIndex, bridgeDomain)
		}

		// Remove MemIf from Bridge. RemoveBridgeInterface() will delete Bridge if
		// no more interfaces are associated with the Bridge.
		err = vppbridge.RemoveBridgeInterface(vppCh.Ch, bridgeDomain, data.SwIfIndex)

		if err != nil {
			if dbgBridge {
				fmt.Println("Error:", err)
			}
			return err
		} else {
			if dbgBridge {
				fmt.Printf("INTERFACE %d removed from BRIDGE %d\n", data.SwIfIndex, bridgeDomain)
				vppbridge.DumpBridge(vppCh.Ch, bridgeDomain)
			}
		}
	}

	//
	// Delete Local Interface
	//
	if conf.HostConf.IfType == "memif" {
		return delLocalDeviceMemif(vppCh, conf, args, &data)
	} else if conf.HostConf.IfType == "vhostuser" {
		return fmt.Errorf("GOOD: Found HostConf.Type:" + conf.HostConf.IfType)
	} else {
		return fmt.Errorf("ERROR: Unknown HostConf.Type:" + conf.HostConf.IfType)
	}

	return err
}

func (cniVpp CniVpp) DelFromContainer(conf *usrsptypes.NetConf, args *skel.CmdArgs) error {
	logging.Debugf("VPP DelFromContainer: ENTER")

	usrspdb.CleanupRemoteConfig(conf, args.ContainerID)
	return nil
}

//
// Local Functions
//

func addLocalDeviceMemif(vppCh vppinfra.ConnectionData, conf *usrsptypes.NetConf, args *skel.CmdArgs, data *vppdb.VppSavedData) (err error) {
	var ok bool

	// Validate and convert input data
	var memifSocketFile string
	var memifRole vppmemif.MemifRole
	var memifMode vppmemif.MemifMode

	if memifSocketFile, ok = os.LookupEnv("USERSPACE_MEMIF_SOCKFILE"); ok == false {
		fileName := fmt.Sprintf("memif-%s-%s.sock", args.ContainerID[:12], args.IfName)
		memifSocketFile = filepath.Join(usrspdb.DefaultSocketDir, fileName)
	}

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
	data.MemifSocketId, err = vppmemif.CreateMemifSocket(vppCh.Ch, memifSocketFile)
	if err != nil {
		if dbgInterface {
			fmt.Println("Error:", err)
		}
		return
	} else {
		if dbgInterface {
			fmt.Println("MEMIF SOCKET", data.MemifSocketId, memifSocketFile, "created")
			vppmemif.DumpMemifSocket(vppCh.Ch)
		}
	}

	// Create MemIf Interface
	data.SwIfIndex, err = vppmemif.CreateMemifInterface(vppCh.Ch, data.MemifSocketId, memifRole, memifMode)
	if err != nil {
		if dbgInterface {
			fmt.Println("Error:", err)
		}
		return
	} else {
		if dbgInterface {
			fmt.Println("MEMIF", data.SwIfIndex, "created", args.IfName)
			vppmemif.DumpMemif(vppCh.Ch)
		}
	}

	return
}

func delLocalDeviceMemif(vppCh vppinfra.ConnectionData, conf *usrsptypes.NetConf, args *skel.CmdArgs, data *vppdb.VppSavedData) (err error) {

	var ok bool
	var memifSocketFile string

	if memifSocketFile, ok = os.LookupEnv("USERSPACE_MEMIF_SOCKFILE"); ok == false {
		fileName := fmt.Sprintf("memif-%s-%s.sock", args.ContainerID[:12], args.IfName)
		memifSocketFile = filepath.Join(usrspdb.DefaultSocketDir, fileName)
	}

	err = vppmemif.DeleteMemifInterface(vppCh.Ch, data.SwIfIndex)
	if err != nil {
		if dbgInterface {
			fmt.Println("Error:", err)
		}
		return
	} else {
		if dbgInterface {
			fmt.Printf("INTERFACE %d deleted\n", data.SwIfIndex)
			vppmemif.DumpMemif(vppCh.Ch)
			vppmemif.DumpMemifSocket(vppCh.Ch)
		}
	}

	// Remove file
	err = usrspdb.FileCleanup("", memifSocketFile)

	return
}
