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
package vppmemif

// Generates Go bindings for all VPP APIs located in the json directory.
//go:generate go run go.fd.io/govpp/cmd/binapi-generator --output-dir=../../bin_api

import (
	// "net"

	"os"
	"path/filepath"

	"go.fd.io/govpp/api"

	"github.com/intel/userspace-cni-network-plugin/cnivpp/bin_api/interface_types"
	"github.com/intel/userspace-cni-network-plugin/cnivpp/bin_api/memif"
	"github.com/intel/userspace-cni-network-plugin/logging"
)

//
// Constants
//

const debugMemif = false

type MemifRole uint8

const (
	RoleMaster MemifRole = 0
	RoleSlave  MemifRole = 1
)

type MemifMode uint8

const (
	ModeEthernet   MemifMode = 0
	ModeIP         MemifMode = 1
	ModePuntInject MemifMode = 2
)

// Dump Strings
var modeStr = [...]string{"eth", "ip ", "pnt"}
var roleStr = [...]string{"master", "slave "}
var stateStr = [...]string{"dn", "up"}

//
// API Functions
//

// Attempt to create a MemIf Interface.
// Input:
//
//	ch api.Channel
//	socketId uint32
//	role MemifRole - RoleMaster or RoleSlave
func CreateMemifInterface(ch api.Channel, socketId uint32, role memif.MemifRole, mode memif.MemifMode) (swIfIndex interface_types.InterfaceIndex, err error) {

	// Populate the Add Structure
	req := &memif.MemifCreate{
		Role:     role,
		Mode:     mode,
		RxQueues: 1,
		TxQueues: 1,
		ID:       0,
		SocketID: socketId,
		//Secret: "",
		RingSize:   1024,
		BufferSize: 2048,
		//HwAddr: "",
	}

	reply := &memif.MemifCreateReply{}

	err = ch.SendRequest(req).ReceiveReply(reply)

	if err != nil {
		if debugMemif {
			logging.Verbosef("Error creating memif interface: %v", err)
		}
		return
	} else {
		swIfIndex = interface_types.InterfaceIndex(reply.SwIfIndex)
	}

	return
}

// Attempt to delete a memif interface. If the deleted MemIf Interface
// is the last interface associated with a socketfile, this function
// will attempt to delete it.
func DeleteMemifInterface(ch api.Channel, swIfIndex interface_types.InterfaceIndex) (err error) {

	// Determine if memif interface exists
	socketId, exist := findMemifInterface(ch, swIfIndex)
	if debugMemif {
		if exist == false {
			logging.Verbosef("Error deleting memif interface: memif interface (swIfIndex=%d) Does NOT Exist", swIfIndex)
		} else {
			logging.Verbosef("Attempting to delete memif interface %d with SocketId %d", swIfIndex, socketId)
		}
	}

	// Populate the Delete Structure
	req := &memif.MemifDelete{
		SwIfIndex: swIfIndex,
	}

	reply := &memif.MemifDeleteReply{}

	err = ch.SendRequest(req).ReceiveReply(reply)

	if err != nil {
		if debugMemif {
			logging.Verbosef("Error deleting memif interface: %v", err)
		}
		return err
	}

	// If the socketFile is not the default (0), then determine if it is no longer in use,
	// and if not, then delete it.
	if socketId != 0 {
		count := findMemifSocketCnt(ch, socketId)
		if debugMemif {
			logging.Verbosef("SocketId %d has %d attached interfaces", socketId, count)
		}
		if count == 0 {
			err = DeleteMemifSocket(ch, socketId)
			if err != nil {
				if debugMemif {
					logging.Verbosef("Error deleting memif socket: %v", err)
				}
				return err
			}
		}
	}

	return err
}

// Dump the set of existing memif interfaces to stdout.
func DumpMemif(ch api.Channel) {
	var count int

	// Populate the Message Structure
	req := &memif.MemifDump{}
	reqCtx := ch.SendMultiRequest(req)

	logging.Verbosef("Memif Interface List:")
	for {
		reply := &memif.MemifDetails{}
		stop, err := reqCtx.ReceiveReply(reply)
		if stop {
			break // break out of the loop
		}
		if err != nil {
			logging.Verbosef("Error dumping memif interface: %v", err)
		}
		//logging.Verbosef("%+v", reply)

		macAddr := reply.HwAddr
		logging.Verbosef("    SwIfId=%d ID=%d Socket=%d Role=%s Mode=%s IfName=%s HwAddr=%v RingSz=%d BufferSz=%d",
			reply.SwIfIndex,
			reply.ID,
			reply.SocketID,
			roleStr[reply.Role],
			modeStr[reply.Mode],
			string(reply.IfName),
			macAddr,
			reply.RingSize,
		)
		// reply.BufferSize,
		// stateStr[reply.AdminUpDown],
		// stateStr[reply.LinkUpDown])

		count++
	}

	logging.Verbosef("  Interface Count: %d", count)
}

// API to Create the MemIf Socketfile.
func CreateMemifSocket(ch api.Channel, socketFile string) (socketId uint32, err error) {

	found, socketId := findMemifSocket(ch, socketFile)
	if found {
		if debugMemif {
			logging.Verbosef("Socketfile already exists")
		}
		return
	}

	if debugMemif {
		logging.Verbosef("Attempting to create SocketId=%d File=%s", socketId, socketFile)
	}

	// Determine if directory socket is created in exists. If it doesn't, create it.
	sockDir := filepath.Dir(socketFile)
	if _, err = os.Stat(sockDir); err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(sockDir, 0700); err != nil {
				if debugMemif {
					logging.Verbosef("Unable to create Socketfile directory")
				}
				return
			}
		} else {
			if debugMemif {
				logging.Verbosef("Error getting status of Socketfile directory")
			}
			return
		}
	}

	// Populate the Request Structure
	req := &memif.MemifSocketFilenameAddDel{
		IsAdd:          true,
		SocketID:       socketId,
		SocketFilename: socketFile,
	}

	reply := &memif.MemifSocketFilenameAddDelReply{}

	err = ch.SendRequest(req).ReceiveReply(reply)

	if debugMemif {
		if err != nil {
			logging.Verbosef("Error creating memif socket: %v", err)
		} else {
			logging.Verbosef("Creating memif socket: rval=%d", reply.Retval)
		}
	}

	return
}

// API to Delete the MemIf Socketfile.
func DeleteMemifSocket(ch api.Channel, socketId uint32) (err error) {
	// Populate the Add Structure
	req := &memif.MemifSocketFilenameAddDel{
		IsAdd:    false,
		SocketID: socketId,
	}

	reply := &memif.MemifSocketFilenameAddDelReply{}

	err = ch.SendRequest(req).ReceiveReply(reply)

	if debugMemif {
		if err != nil {
			logging.Verbosef("Error deleting memif socket: %v", err)
		} else {
			logging.Verbosef("Deleting memif socket: rval=%d", reply.Retval)
		}
	}

	return
}

// Dump the set of existing memif socketfiles to stdout.
func DumpMemifSocket(ch api.Channel) {
	var count int

	// Populate the Message Structure
	req := &memif.MemifSocketFilenameDump{}
	reqCtx := ch.SendMultiRequest(req)

	logging.Verbosef("Memif Socket List:")
	for {
		reply := &memif.MemifSocketFilenameDetails{}
		stop, err := reqCtx.ReceiveReply(reply)
		if stop {
			break // break out of the loop
		}
		if err != nil {
			logging.Verbosef("Error dumping memif socket: %v", err)
		}
		//logging.Verbosef("%+v", reply)

		socketId := reply.SocketID
		filename := string(reply.SocketFilename)
		logging.Verbosef("    SocketId=%d Filename=%s", socketId, filename)
		count++
	}

	logging.Verbosef("  Socket Count: %d", count)
}

//
// Local Functions
//

// Find the given memif interface and return socketId if it exists
func findMemifInterface(ch api.Channel, swIfIndex interface_types.InterfaceIndex) (socketId uint32, found bool) {

	// Populate the Message Structure
	req := &memif.MemifDump{}
	reqCtx := ch.SendMultiRequest(req)

	for {
		reply := &memif.MemifDetails{}
		stop, err := reqCtx.ReceiveReply(reply)
		if stop {
			break // break out of the loop
		}
		if err != nil {
			if debugMemif {
				logging.Verbosef("Error searching memif interface: %v", err)
			}
		} else if swIfIndex == reply.SwIfIndex {
			found = true
			socketId = reply.SocketID
		}
	}
	return
}

// Loop through the memif interfaces and return the number of interfaces using the given socketId
func findMemifSocketCnt(ch api.Channel, socketId uint32) (count uint32) {

	// Populate the Message Structure
	req := &memif.MemifDump{}
	reqCtx := ch.SendMultiRequest(req)

	for {
		reply := &memif.MemifDetails{}
		stop, err := reqCtx.ReceiveReply(reply)
		if stop {
			break // break out of the loop
		}
		if err != nil {
			if debugMemif {
				logging.Verbosef("Error searching memif interface: %v", err)
			}
		} else if socketId == reply.SocketID {
			count++
		}
	}
	return
}

// Loop through the list of Memif Sockets and determine if input
// socketId exists.
// Returns: true - exists  false - otherwise
//func findMemifSocket(ch api.Channel, socketId uint32) (found bool) {
//
//	// Populate the Message Structure
//	req := &memif.MemifSocketFilenameDump{}
//	reqCtx := ch.SendMultiRequest(req)
//
//	for {
//		reply := &memif.MemifSocketFilenameDetails{}
//		stop, err := reqCtx.ReceiveReply(reply)
//		if stop {
//			break // break out of the loop
//		}
//		if err != nil {
//			logging.Verbosef("Error dumping memif socket: %v", err)
//		}
//
//		if socketId == reply.SocketID {
//			found = true
//		}
//	}
//
//	return
//}

// Loop through the list of Memif Sockets and determine if input
// socketFile exists. If it does, return the associated socketId.
// If it doesn't, return the next available socketId.
// Returns:
//
//	bool - Found flag
//	uint32 - If found is true: associated socketId.
//	         If found is false: next free socketId.
func findMemifSocket(ch api.Channel, socketFilename string) (found bool, socketId uint32) {

	var count int
	var usedList [20]uint32
	var done bool

	// Populate the Message Structure
	req := &memif.MemifSocketFilenameDump{}
	reqCtx := ch.SendMultiRequest(req)

	//
	// Loop through the exisiting SocketFiles and see if input
	// socketFilename already exists,
	//
	for {
		reply := &memif.MemifSocketFilenameDetails{}
		stop, err := reqCtx.ReceiveReply(reply)
		if stop {
			break // break out of the loop
		}
		if err != nil {
			if debugMemif {
				logging.Verbosef("Error retrieving memif socket: %v", err)
			}
		}

		if socketFilename == string(reply.SocketFilename) {
			found = true
			socketId = reply.SocketID
			break // break out of the loop
		} else {
			usedList[count] = reply.SocketID
			count++
		}
	}

	//
	// If input SocketFilename has not been created, then loop
	// through the list of existing SocketIds and find an unused Id.
	//
	if found == false {
		socketId = 1

		for done == false {

			done = true
			for i := 0; i < count; i++ {
				if socketId == usedList[i] {
					socketId++
					done = false
					break
				}
			}

		}
	}

	return
}
