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
//go:generate binapi-generator --input-dir=../../bin_api --output-dir=../../bin_api

import (
	"fmt"
	"net"

	"git.fd.io/govpp.git/api"
	"git.fd.io/govpp.git/core/bin_api/memif"
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

// Check whether generated API messages are compatible with the version
// of VPP which the library is connected to.
func MemifCompatibilityCheck(ch *api.Channel) (err error) {
	err = ch.CheckMessageCompatibility(
		&memif.MemifSocketFilenameAddDel{},
		&memif.MemifCreate{},
		&memif.MemifDelete{},
		&memif.MemifSocketFilenameDetails{},
		&memif.MemifSocketFilenameDump{},
		&memif.MemifDump{},
	)
	if err != nil {
		if debugMemif {
			fmt.Println("VPP memif failed compatibility")
		}
	}

	return err
}

// Attempt to create a MemIf Interface.
// Input:
//   ch *api.Channel
//   socketId uint32
//   role MemifRole - RoleMaster or RoleSlave
func CreateMemifInterface(ch *api.Channel, socketId uint32, role MemifRole, mode MemifMode) (swIfIndex uint32, err error) {

	// Populate the Add Structure
	req := &memif.MemifCreate{
		Role:     uint8(role),
		Mode:     uint8(mode),
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
			fmt.Println("Error creating memif interface:", err)
		}
		return
	} else {
		swIfIndex = reply.SwIfIndex
	}

	return
}

// Attempt to delete a memif interface. If the deleted MemIf Interface
// is the last interface associated with a socketfile, this function
// will attempt to delete it.
func DeleteMemifInterface(ch *api.Channel, swIfIndex uint32) (err error) {

	// Determine if memif interface exists
	socketId, exist := findMemifInterface(ch, swIfIndex)
	if debugMemif {
		if exist == false {
			fmt.Printf("Error deleting memif interface: memif interface (swIfIndex=%d) Does NOT Exist", swIfIndex)
		} else {
			fmt.Printf("Attempting to delete memif interface %d with SocketId %d", swIfIndex, socketId)
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
			fmt.Println("Error deleting memif interface:", err)
		}
		return err
	}

	// If the socketFile is not the default (0), then determine if it is no longer in use,
	// and if not, then delete it.
	if socketId != 0 {
		count := findMemifSocketCnt(ch, socketId)
		if debugMemif {
			fmt.Printf("SocketId %d has %d attached interfaces", socketId, count)
		}
		if count == 0 {
			err = DeleteMemifSocket(ch, socketId)
			if err != nil {
				if debugMemif {
					fmt.Println("Error deleting memif socket:", err)
				}
				return err
			}
		}
	}

	return err
}

// Dump the set of existing memif interfaces to stdout.
func DumpMemif(ch *api.Channel) {
	var count int

	// Populate the Message Structure
	req := &memif.MemifDump{}
	reqCtx := ch.SendMultiRequest(req)

	fmt.Printf("Memif Interface List:\n")
	for {
		reply := &memif.MemifDetails{}
		stop, err := reqCtx.ReceiveReply(reply)
		if stop {
			break // break out of the loop
		}
		if err != nil {
			fmt.Println("Error dumping memif interface:", err)
		}
		//fmt.Printf("%+v\n", reply)

		macAddr := net.HardwareAddr(reply.HwAddr)
		fmt.Printf("    SwIfId=%d ID=%d Socket=%d Role=%s Mode=%s IfName=%s HwAddr=%s RingSz=%d BufferSz=%d Admin=%s Link=%s\n",
			reply.SwIfIndex,
			reply.ID,
			reply.SocketID,
			roleStr[reply.Role],
			modeStr[reply.Mode],
			string(reply.IfName),
			macAddr.String(),
			reply.RingSize,
			reply.BufferSize,
			stateStr[reply.AdminUpDown],
			stateStr[reply.LinkUpDown])

		count++
	}

	fmt.Printf("  Interface Count: %d\n", count)
}

// API to Create the MemIf Socketfile.
func CreateMemifSocket(ch *api.Channel, socketFile string) (socketId uint32, err error) {

	found, socketId := findMemifSocket(ch, socketFile)
	if found {
		fmt.Println("Socketfile already exists")
		return
	}

	if debugMemif {
		fmt.Printf("Attempting to create SocketId=%d File=%s\n", socketId, socketFile)
	}

	// Populate the Request Structure
	req := &memif.MemifSocketFilenameAddDel{
		IsAdd:          1,
		SocketID:       socketId,
		SocketFilename: []byte(socketFile),
	}

	reply := &memif.MemifSocketFilenameAddDelReply{}

	err = ch.SendRequest(req).ReceiveReply(reply)

	if debugMemif {
		if err != nil {
			fmt.Println("Error creating memif socket:", err)
		} else {
			fmt.Printf("Creating memif socket: rval=%d\n", reply.Retval)
		}
	}

	return
}

// API to Delete the MemIf Socketfile.
func DeleteMemifSocket(ch *api.Channel, socketId uint32) (err error) {
	// Populate the Add Structure
	req := &memif.MemifSocketFilenameAddDel{
		IsAdd:    0,
		SocketID: socketId,
	}

	reply := &memif.MemifSocketFilenameAddDelReply{}

	err = ch.SendRequest(req).ReceiveReply(reply)

	if debugMemif {
		if err != nil {
			fmt.Println("Error deleting memif socket:", err)
		} else {
			fmt.Printf("Deleting memif socket: rval=%d\n", reply.Retval)
		}
	}

	return
}

// Dump the set of existing memif socketfiles to stdout.
func DumpMemifSocket(ch *api.Channel) {
	var count int

	// Populate the Message Structure
	req := &memif.MemifSocketFilenameDump{}
	reqCtx := ch.SendMultiRequest(req)

	fmt.Printf("Memif Socket List:\n")
	for {
		reply := &memif.MemifSocketFilenameDetails{}
		stop, err := reqCtx.ReceiveReply(reply)
		if stop {
			break // break out of the loop
		}
		if err != nil {
			fmt.Println("Error dumping memif socket:", err)
		}
		//fmt.Printf("%+v\n", reply)

		socketId := reply.SocketID
		filename := string(reply.SocketFilename)
		fmt.Printf("    SocketId=%d Filename=%s\n", socketId, filename)
		count++
	}

	fmt.Printf("  Socket Count: %d\n", count)
}

//
// Local Functions
//

// Find the given memif interface and return socketId if it exists
func findMemifInterface(ch *api.Channel, swIfIndex uint32) (socketId uint32, found bool) {

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
				fmt.Println("Error searching memif interface:", err)
			}
		} else if swIfIndex == reply.SwIfIndex {
			found = true
			socketId = reply.SocketID
		}
	}
	return
}

// Loop through the memif interfaces and return the number of interfaces using the given socketId
func findMemifSocketCnt(ch *api.Channel, socketId uint32) (count uint32) {

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
				fmt.Println("Error searching memif interface:", err)
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
//func findMemifSocket(ch *api.Channel, socketId uint32) (found bool) {
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
//			fmt.Println("Error dumping memif socket:", err)
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
//   bool - Found flag
//   uint32 - If found is true: associated socketId.
//            If found is false: next free socketId.
func findMemifSocket(ch *api.Channel, socketFilename string) (found bool, socketId uint32) {

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
				fmt.Println("Error retrieving memif socket:", err)
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
