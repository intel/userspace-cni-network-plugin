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
//go:generate go run git.fd.io/govpp.git/cmd/binapi-generator --output-dir=../../bin_api

import (
	"fmt"

	"github.com/containernetworking/cni/pkg/types/current"

	"git.fd.io/govpp.git/api"
	"github.com/intel/userspace-cni-network-plugin/cnivpp/bin_api/interfaces"
)

// Constants
const debugInterface = false

//
// API Functions
//

// Attempt to set an interface state. isUp (1 = up, 0 = down)
func SetState(ch api.Channel, swIfIndex interfaces.InterfaceIndex, isUp interfaces.IfStatusFlags) error {
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

func AddDelIpAddress(ch api.Channel, swIfIndex interfaces.InterfaceIndex, isAdd bool, ipResult *current.Result) error {

	// Populate the Add Structure
	req := &interfaces.SwInterfaceAddDelAddress{
		SwIfIndex: swIfIndex,
		IsAdd:     isAdd, // 1 = add, 0 = delete
		DelAll:    false,
	}

	// for _, ip := range ipResult.IPs {
	// 	if ip.Version == "4" {
	// 		req.IsIPv6 = 0
	// 		req.Address = []byte(ip.Address.IP.To4())
	// 		prefix, _ := ip.Address.Mask.Size()
	// 		req.AddressLength = byte(prefix)
	// 	} else if ip.Version == "6" {
	// 		req.IsIPv6 = 1
	// 		req.Address = []byte(ip.Address.IP.To16())
	// 		prefix, _ := ip.Address.Mask.Size()
	// 		req.AddressLength = byte(prefix)
	// 	}

	// 	// Only one address is currently supported.
	// 	if req.AddressLength != 0 {
	// 		break
	// 	}
	// }

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
