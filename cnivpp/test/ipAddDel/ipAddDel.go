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
//go:generate binapi-generator --input-dir=../../bin_api --output-dir=../../bin_api

import (
	"fmt"
	_ "net"
	"os"
	"runtime"
	"time"

	_ "go.fd.io/govpp/core"
	current "github.com/containernetworking/cni/pkg/types/100"
	_ "github.com/sirupsen/logrus"

	"github.com/intel/userspace-cni-network-plugin/cnivpp/api/infra"
	"github.com/intel/userspace-cni-network-plugin/cnivpp/api/interface"
	"github.com/intel/userspace-cni-network-plugin/cnivpp/api/memif"
)

//
// Constants
//
const (
	dbgIp    = true
	dbgMemif = true
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
	var ipString string = "192.168.172.100/24"
	var ipResult *current.Result
	var memifSocketId uint32
	var memifSocketFile string = "/var/run/vpp/123456/memif-3.sock"
	var memifRole vppmemif.MemifRole = vppmemif.RoleMaster
	var memifMode vppmemif.MemifMode = vppmemif.ModeEthernet

	// Set log level
	//   Logrus has six logging levels: DebugLevel, InfoLevel, WarningLevel, ErrorLevel, FatalLevel and PanicLevel.
	//core.SetLogger(&logrus.Logger{Level: logrus.InfoLevel})

	fmt.Println("Starting User Space client...")

	// Create Channel to pass requests to VPP
	vppCh, err = vppinfra.VppOpenCh()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	defer vppinfra.VppCloseCh(vppCh)

	// Create Memif Socket
	memifSocketId, err = vppmemif.CreateMemifSocket(vppCh.Ch, memifSocketFile)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	} else {
		fmt.Println("MEMIF SOCKET", memifSocketId, memifSocketFile, "created")
		if dbgMemif {
			vppmemif.DumpMemifSocket(vppCh.Ch)
		}
	}

	// Create MemIf Interface
	swIfIndex, err = vppmemif.CreateMemifInterface(vppCh.Ch, memifSocketId, memifRole, memifMode)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	} else {
		fmt.Println("MEMIF", swIfIndex, "created")
		if dbgMemif {
			vppmemif.DumpMemif(vppCh.Ch)
		}
	}

	// Set interface to up (1)
	err = vppinterface.SetState(vppCh.Ch, swIfIndex, 1)
	if err != nil {
		fmt.Println("Error bringing interface UP:", err)
		os.Exit(1)
	}

	// Add IP to MemIf to Bridge.
	err = vppinterface.AddDelIpAddress(vppCh.Ch, swIfIndex, 1, ipResult)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	} else {
		fmt.Printf("IP %s added to INTERFACE %d\n", ipString, swIfIndex)
	}

	fmt.Println("Sleeping for 30 seconds...")
	time.Sleep(30 * time.Second)
	fmt.Println("User Space VPP client wakeup.")

	// Remove IP from MemIf.
	err = vppinterface.AddDelIpAddress(vppCh.Ch, swIfIndex, 0, ipResult)

	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	} else {
		fmt.Printf("IP %s removed from INTERFACE %d\n", ipString, swIfIndex)
	}

	fmt.Println("Sleeping for 30 seconds...")
	time.Sleep(30 * time.Second)
	fmt.Println("User Space VPP client wakeup.")

	fmt.Println("Delete memif interface.")
	err = vppmemif.DeleteMemifInterface(vppCh.Ch, swIfIndex)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	} else {
		fmt.Printf("INTERFACE %d deleted\n", swIfIndex)
		if dbgMemif {
			vppmemif.DumpMemif(vppCh.Ch)
			vppmemif.DumpMemifSocket(vppCh.Ch)
		}
	}
}
