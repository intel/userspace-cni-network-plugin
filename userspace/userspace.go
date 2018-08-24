// Copyright 2017 Intel Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	cniSpecVersion "github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"

	"github.com/intel/vhost-user-net-plugin/cniovs/cniovs"
	"github.com/intel/vhost-user-net-plugin/cnivpp/cnivpp"
	"github.com/intel/vhost-user-net-plugin/usrsptypes"

	"github.com/vishvananda/netlink"
)

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

//
// Local functions
//

// loadNetConf() - Unmarshall the inputdata into the NetConf Structure
func loadNetConf(bytes []byte) (*usrsptypes.NetConf, error) {
	n := &usrsptypes.NetConf{}
	if err := json.Unmarshal(bytes, n); err != nil {
		return nil, fmt.Errorf("failed to load netconf: %v", err)
	}

	return n, nil
}

func cmdAdd(args *skel.CmdArgs) error {
	var result *current.Result
	var netConf *usrsptypes.NetConf
	var containerEngine string

	vpp := cnivpp.CniVpp{}
	ovs := cniovs.CniOvs{}

	// Convert the input bytestream into local NetConf structure
	netConf, err := loadNetConf(args.StdinData)
	if err != nil {
		return err
	}

	//
	// HOST:
	//

	// Add the requested interface and network
	if netConf.HostConf.Engine == "vpp" {
		err = vpp.AddOnHost(netConf, args, result)
	} else if netConf.HostConf.Engine == "ovs-dpdk" {
		err = ovs.AddOnHost(netConf, args, result)
	} else {
		return fmt.Errorf("ERROR: Unknown Host Engine:" + netConf.HostConf.Engine)
	}
	if err != nil {
		return err
	}

	//
	// CONTAINER:
	//

	// Get IPAM data for Container Interface, if provided.
	if netConf.IPAM.Type != "" {

		// run the IPAM plugin and get back the config to apply
		ipamResult, err := ipam.ExecAdd(netConf.IPAM.Type, args.StdinData)
		if err != nil {
			return err
		}

		// Convert whatever the IPAM result was into the current Result type
		result, err = current.NewResultFromResult(ipamResult)
		if err != nil {
			// TBD: CLEAN-UP
			return err
		}

		if len(result.IPs) == 0 {
			// TBD: CLEAN-UP
			return fmt.Errorf("ERROR: Unable to get IP Address")
		}

		// Clear out the Gateway if set by IPAM, not being used.
		for _, ip := range result.IPs {
			ip.Gateway = nil
		}

	} else {
		result = &current.Result{}
	}

	// Determine the Engine that will process the request. Default to host
	// if not provided.
	if netConf.ContainerConf.Engine != "" {
		containerEngine = netConf.ContainerConf.Engine
	} else {
		containerEngine = netConf.HostConf.Engine
	}

	// Add the requested interface and network
	if containerEngine == "vpp" {
		err = vpp.AddOnContainer(netConf, args, result)
	} else if containerEngine == "ovs-dpdk" {
		err = ovs.AddOnContainer(netConf, args, result)
	} else {
		return fmt.Errorf("ERROR: Unknown Container Engine:" + containerEngine)
	}
	if err != nil {
		return err
	}

	return cnitypes.PrintResult(result, netConf.CNIVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	var netConf *usrsptypes.NetConf
	var containerEngine string

	vpp := cnivpp.CniVpp{}
	ovs := cniovs.CniOvs{}

	// Convert the input bytestream into local NetConf structure
	netConf, err := loadNetConf(args.StdinData)
	if err != nil {
		return err
	}

	//
	// HOST:
	//

	// Delete the requested interface
	if netConf.HostConf.Engine == "vpp" {
		err = vpp.DelFromHost(netConf, args)
	} else if netConf.HostConf.Engine == "ovs-dpdk" {
		err = ovs.DelFromHost(netConf, args)
	} else {
		return fmt.Errorf("ERROR: Unknown Host Engine:" + netConf.HostConf.Engine)
	}
	if err != nil {
		return err
	}

	//
	// CONTAINER
	//

	// Determine the Engine that will process the request. Default to host
	// if not provided.
	if netConf.ContainerConf.Engine != "" {
		containerEngine = netConf.ContainerConf.Engine
	} else {
		containerEngine = netConf.HostConf.Engine
	}

	// Delete the requested interface
	if containerEngine == "vpp" {
		err = vpp.DelFromContainer(netConf, args)
	} else if containerEngine == "ovs-dpdk" {
		err = ovs.DelFromContainer(netConf, args)
	} else {
		return fmt.Errorf("ERROR: Unknown Container Engine:" + containerEngine)
	}
	if err != nil {
		return err
	}

	//
	// Cleanup IPAM data, if provided.
	//
	if netConf.IPAM.Type != "" {
		err = ipam.ExecDel(netConf.IPAM.Type, args.StdinData)
		if err != nil {
			return err
		}
	}

	//
	// Cleanup Namespace
	//
	if args.Netns == "" {
		return nil
	}

	err = ns.WithNetNSPath(args.Netns, func(_ ns.NetNS) error {
		var err error
		_, err = ip.DelLinkByNameAddr(args.IfName, netlink.FAMILY_V4)
		if err != nil && err == ip.ErrLinkNotFound {
			return nil
		}
		return err
	})

	if err != nil {
		return err
	}

	return nil
}

func main() {
	skel.PluginMain(cmdAdd, cmdDel, cniSpecVersion.All)
}
