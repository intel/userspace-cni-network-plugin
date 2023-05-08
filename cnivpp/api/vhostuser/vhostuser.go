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
package vppvhostuser

// Generates Go bindings for all VPP APIs located in the json directory.
//go:generate go run go.fd.io/govpp/cmd/binapi-generator --output-dir=../../bin_api

import (
	"fmt"

	"go.fd.io/govpp/api"
	"github.com/intel/userspace-cni-network-plugin/cnivpp/bin_api/vhost_user"
)

//
// Constants
//

const debugVhost = false

type VhostUserMode uint8

const (
	ModeClient VhostUserMode = 0
	ModeServer VhostUserMode = 1
)

// Dump Strings
var modeStr = [...]string{"client", "server"}

//
// API Functions
//

// Attempt to create a Vhost-User Interface.
// Input:
//   ch api.Channel
//   mode VhostUserMode - ModeClient or ModeServer
//   socketFile string - Directory and Filename of socket file
func CreateVhostUserInterface(ch api.Channel, mode VhostUserMode, socketFile string) (swIfIndex uint32, err error) {

	// Populate the Add Structure
	req := &vhost_user.CreateVhostUserIf{
		IsServer:          uint8(mode),
		SockFilename:      []byte(socketFile),
		Renumber:          0,
		CustomDevInstance: 0,
		UseCustomMac:      0,
		//MacAddress: "",
		//Tag: "",
	}

	reply := &vhost_user.CreateVhostUserIfReply{}

	err = ch.SendRequest(req).ReceiveReply(reply)

	if err != nil {
		if debugVhost {
			fmt.Println("Error creating vhostUser interface:", err)
		}
		return
	} else {
		swIfIndex = reply.SwIfIndex
	}

	return
}

// Attempt to delete a Vhost-User interface.
func DeleteVhostUserInterface(ch api.Channel, swIfIndex uint32) (err error) {

	// Populate the Delete Structure
	req := &vhost_user.DeleteVhostUserIf{
		SwIfIndex: swIfIndex,
	}

	reply := &vhost_user.DeleteVhostUserIfReply{}

	err = ch.SendRequest(req).ReceiveReply(reply)

	if err != nil {
		if debugVhost {
			fmt.Println("Error deleting vhostUser interface:", err)
		}
		return err
	}

	return err
}

// Dump the set of existing Vhost-User interfaces to stdout.
func DumpVhostUser(ch api.Channel) {
	var count int

	// Populate the Message Structure
	req := &vhost_user.SwInterfaceVhostUserDump{}
	reqCtx := ch.SendMultiRequest(req)

	fmt.Printf("Vhost-User Interface List:\n")
	for {
		reply := &vhost_user.SwInterfaceVhostUserDetails{}
		stop, err := reqCtx.ReceiveReply(reply)
		if stop {
			break // break out of the loop
		}
		if err != nil {
			fmt.Println("Error dumping vhostUser interface:", err)
		}
		//fmt.Printf("%+v\n", reply)

		fmt.Printf("    SwIfId=%d Mode=%s IfName=%s NumReg=%d SockErrno=%d Feature=0x16%x HdrSz=%d SockFile=%s\n",
			reply.SwIfIndex,
			modeStr[reply.IsServer],
			string(reply.InterfaceName),
			reply.NumRegions,
			reply.SockErrno,
			reply.Features,
			reply.VirtioNetHdrSz,
			string(reply.SockFilename))

		count++
	}

	fmt.Printf("  Interface Count: %d\n", count)
}
