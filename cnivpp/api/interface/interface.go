// Copyright (c) 2017 Cisco and/or its affiliates.
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

// Binary simple-client is an example VPP management application that exercises the
// govpp API on real-world use-cases.
package vppinterface

// Generates Go bindings for all VPP APIs located in the json directory.
//go:generate go run go.fd.io/govpp/cmd/binapi-generator --output-dir=../../bin_api

import (
	"fmt"

	current "github.com/containernetworking/cni/pkg/types/100"
	interfaces "github.com/intel/userspace-cni-network-plugin/cnivpp/bin_api/interface"
	"github.com/intel/userspace-cni-network-plugin/cnivpp/bin_api/interface_types"
	"github.com/intel/userspace-cni-network-plugin/cnivpp/bin_api/ip_types"
	"go.fd.io/govpp/api"
)

// Constants
const debugInterface = false

//
// API Functions
//

// Attempt to set an interface state. isUp (1 = up, 0 = down)
func SetState(ch api.Channel, swIfIndex interface_types.InterfaceIndex, isUp interface_types.IfStatusFlags) error {
	// Populate the Add Structure
	req := &interfaces.SwInterfaceSetFlags{
		SwIfIndex: swIfIndex,
		// 1 = up, 0 = down
		Flags: isUp,
	}

	reply := &interfaces.SwInterfaceSetFlagsReply{}

	err := ch.SendRequest(req).ReceiveReply(reply)

	if err != nil {
		if debugInterface {
			fmt.Println("Error:", err)
		}
		return err
	}

	return nil
}

func AddDelIpAddress(ch api.Channel, swIfIndex interface_types.InterfaceIndex, isAdd bool, ipResult *current.Result) error {

	// Populate the Add Structure
	req := &interfaces.SwInterfaceAddDelAddress{
		SwIfIndex: swIfIndex,
		IsAdd:     isAdd, // 1 = add, 0 = delete
		DelAll:    false,
	}
	for _, ip := range ipResult.IPs {
		var addressWithPrefix ip_types.AddressWithPrefix

		if prefix, _ := ip.Address.Mask.Size(); prefix == 4 {
			addressWithPrefix = ip_types.AddressWithPrefix{Address: ip_types.AddressFromIP(ip.Address.IP.To4()), Len: 4}

		} else if prefix, _ := ip.Address.Mask.Size(); prefix == 16 {
			addressWithPrefix = ip_types.AddressWithPrefix{Address: ip_types.AddressFromIP(ip.Address.IP.To16()), Len: 16}
		} else {
			break
		}
		fmt.Println(addressWithPrefix)
		req.Prefix = addressWithPrefix

		// Only one address is currently supported.
		if req.Prefix.Len != 0 {
			break
		}
	}

	reply := &interfaces.SwInterfaceAddDelAddressReply{}

	err := ch.SendRequest(req).ReceiveReply(reply)

	if err != nil {
		if debugInterface {
			fmt.Println("Error:", err)
		}
		return err
	}

	return nil
}
