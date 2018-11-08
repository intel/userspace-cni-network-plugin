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
// VPP implementation of the UserSpace CNI on the host will write
// configuration data in the from of json data to local files. The
// directory containing the files is then mapped to a container.
//
// This application is designed to run in a container, process the
// files written by the host and config the local vSwitch instance in
// the container.
//

package main

import (
	"fmt"
	"time"

	"github.com/intel/userspace-cni-network-plugin/cniovs/cniovs"
	"github.com/intel/userspace-cni-network-plugin/cnivpp/cnivpp"
	"github.com/intel/userspace-cni-network-plugin/logging"
	"github.com/intel/userspace-cni-network-plugin/usrspdb"
)

//
// Constants
//
const (
	dbgApp = true
)

//
// Types
//

//
// API Functions
//

//
// Utility Functions
//
func cniContainerConfig() (bool, string, error) {
	var engine string

	vpp := cnivpp.CniVpp{}
	ovs := cniovs.CniOvs{}

	found, netConf, ipResult, args, err := usrspdb.FindRemoteConfig()

	if err == nil {
		if found {

			if netConf.LogFile != "" {
				logging.SetLogFile(netConf.LogFile)
			}
			if netConf.LogLevel != "" {
				logging.SetLogLevel(netConf.LogLevel)
			}

			if dbgApp {
				fmt.Println("ipResult:")
				fmt.Println(ipResult)
				fmt.Println("Logfile:")
				fmt.Println(netConf.LogFile)
			}

			// Add the requested interface and network
			engine = netConf.HostConf.Engine
			if netConf.HostConf.Engine == "vpp" {
				err = vpp.AddOnHost(&netConf, &args, &ipResult)
			} else if netConf.HostConf.Engine == "ovs-dpdk" {
				err = ovs.AddOnHost(&netConf, &args, &ipResult)
			} else {
				err = fmt.Errorf("ERROR: Unknown Host Engine:" + netConf.HostConf.Engine)
			}
			if err != nil {
				if dbgApp {
					fmt.Println(err)
				}
			}
		}
	}

	return found, engine, err
}

//
// main
//
func main() {
	var count int = 0
	var processed bool = false
	var processedCnt int = 0
	var engine string

	for {
		count++

		found, tmpEngine, err := cniContainerConfig()

		if dbgApp {
			if err != nil {
				fmt.Println("ERROR returned:", err)
			}

			fmt.Println("LOOP", count, " - FOUND:", found)
		}

		//
		// Once files have been found, wait 1 more loop and exit.
		//
		if found {
			fmt.Println("")
			fmt.Println("")
			fmt.Println("Found Configuration and applied.")
			processed = true
			engine = tmpEngine
		}

		if processed {
			processedCnt++

			if processedCnt > 1 {
				if engine == "vpp" {
					fmt.Println("")
					fmt.Println("Useful VPP CLI Commands:")
					fmt.Println(" vppctl show interface")
					fmt.Println(" vppctl show interface addr")
					fmt.Println(" vppctl show mode")
					fmt.Println(" vppctl show hardware")
				} else if engine == "ovs-dpdk" {
					fmt.Println("")
					fmt.Println("Useful OvS-DPDK CLI Commands:")
					fmt.Println(" ovs-vsctl list open_vswitch")
					fmt.Println(" ovs-vsctl list port")
					fmt.Println(" ovs-vsctl list bridge")
				}

				fmt.Println("")
				fmt.Println("DONE: Exiting app - Press <ENTER> to return to prompt")
				break
			}
		}

		time.Sleep(3 * time.Second)
	}
}
