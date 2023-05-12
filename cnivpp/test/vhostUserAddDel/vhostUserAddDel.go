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
package main

// Generates Go bindings for all VPP APIs located in the json directory.
//go:generate go run go.fd.io/govpp/cmd/binapi-generator --output-dir=../../bin_api

import (
	"fmt"
	_ "net"
	"os"
	"runtime"
	"time"

	_ "go.fd.io/govpp/core"
	_ "github.com/sirupsen/logrus"

	vppbridge "github.com/intel/userspace-cni-network-plugin/cnivpp/api/bridge"
	vppinfra "github.com/intel/userspace-cni-network-plugin/cnivpp/api/infra"
	vppvhostuser "github.com/intel/userspace-cni-network-plugin/cnivpp/api/vhostuser"
)

//
// Constants
//
const (
	dbgBridge    = true
	dbgVhostUser = true
)

//
// Functions
//
func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func main() {
	var vppCh vppinfra.ConnectionData
	var err error
	var swIfIndex uint32

	// Dummy Input Data
	var bridgeDomain uint32 = 4
	var vhostUserSocketFile string = "/var/run/vpp/123456/vhost3.sock"
	var vhostUserMode vppvhostuser.VhostUserMode = vppvhostuser.ModeServer

	// Set log level
	//   Logrus has six logging levels: DebugLevel, InfoLevel, WarningLevel, ErrorLevel, FatalLevel and PanicLevel.
	//core.SetLogger(&logrus.Logger{Level: logrus.InfoLevel})

	fmt.Println("Starting Vhost-User Test client...")

	// Create Channel to pass requests to VPP
	vppCh, err = vppinfra.VppOpenCh()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	defer vppinfra.VppCloseCh(vppCh)

	// Create Vhost-User Interface
	swIfIndex, err = vppvhostuser.CreateVhostUserInterface(vppCh.Ch, vhostUserMode, vhostUserSocketFile)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	} else {
		fmt.Println("Vhost-User", swIfIndex, "created")
		if dbgVhostUser {
			vppvhostuser.DumpVhostUser(vppCh.Ch)
		}
	}

	// Add Vhost-User to Bridge. If Bridge does not exist, AddBridgeInterface()
	// will create.
	err = vppbridge.AddBridgeInterface(vppCh.Ch, bridgeDomain, swIfIndex)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	} else {
		fmt.Printf("INTERFACE %d add to BRIDGE %d\n", swIfIndex, bridgeDomain)
		if dbgBridge {
			vppbridge.DumpBridge(vppCh.Ch, bridgeDomain)
		}
	}

	fmt.Println("Sleeping for 30 seconds...")
	time.Sleep(30 * time.Second)
	fmt.Println("User Space VPP client wakeup.")

	// Remove Vhost-User from Bridge. RemoveBridgeInterface() will delete Bridge if
	// no more interfaces are associated with the Bridge.
	err = vppbridge.RemoveBridgeInterface(vppCh.Ch, bridgeDomain, swIfIndex)

	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	} else {
		fmt.Printf("INTERFACE %d removed from BRIDGE %d\n", swIfIndex, bridgeDomain)
		if dbgBridge {
			vppbridge.DumpBridge(vppCh.Ch, bridgeDomain)
		}
	}

	fmt.Println("Sleeping for 30 seconds...")
	time.Sleep(30 * time.Second)
	fmt.Println("User Space VPP client wakeup.")

	fmt.Println("Delete Vhost-User interface.")
	err = vppvhostuser.DeleteVhostUserInterface(vppCh.Ch, swIfIndex)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	} else {
		fmt.Printf("INTERFACE %d deleted\n", swIfIndex)
		if dbgVhostUser {
			vppvhostuser.DumpVhostUser(vppCh.Ch)
		}
	}
}
