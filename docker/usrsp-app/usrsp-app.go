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

	"k8s.io/client-go/kubernetes"

	"github.com/intel/userspace-cni-network-plugin/cnivpp"
	"github.com/intel/userspace-cni-network-plugin/logging"
	"github.com/intel/userspace-cni-network-plugin/pkg/configdata"
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
	var kubeClient kubernetes.Interface
	var found bool

	vpp := cnivpp.CniVpp{}
	//ovs := cniovs.CniOvs{}

	ifaceList, sharedDir, err := configdata.GetRemoteConfig()
	if err != nil || len(ifaceList) == 0 {
		return found, engine, err
	} else {
		found = true
	}

	for i := range ifaceList {
		if ifaceList[i].NetConf.LogFile != "" {
			logging.SetLogFile(ifaceList[i].NetConf.LogFile)
		}
		if ifaceList[i].NetConf.LogLevel != "" {
			logging.SetLogLevel(ifaceList[i].NetConf.LogLevel)
		}

		logging.Debugf("USRSP_APP: iface %v - Data %v", i, ifaceList[i])


		// Add the requested interface and network
		engine = ifaceList[i].NetConf.HostConf.Engine
		if ifaceList[i].NetConf.HostConf.Engine == "vpp" {
			err = vpp.AddOnHost(&ifaceList[i].NetConf, &ifaceList[i].Args, kubeClient, sharedDir, &ifaceList[i].IPResult)
		} else if ifaceList[i].NetConf.HostConf.Engine == "ovs-dpdk" {
			//err = ovs.AddOnHost(&ifaceList[i].NetConf, &ifaceList[i].Args, kubeClient, sharedDir, &ifaceList[i].IPResult)
			logging.Debugf("USRSP_APP: \"ovs-dpdk\" - Currently nothing TO DO!")
		} else {
			err = fmt.Errorf("ERROR: Unknown Host Engine:" + ifaceList[i].NetConf.HostConf.Engine)
		}
		if err != nil {
			logging.Errorf("USRSP_APP: %v", err)
		}
	}

	return found, engine, err
}

//
// main
//
func main() {
	var count int = 0
	var engine string
	var err error
	var found bool

	// Give VPP time to come up
	//  TBD - Look for /run/vpp-api.sock to exist or something like that
	time.Sleep(5 * time.Second)

	for {
		count++

		found, engine, err = cniContainerConfig()

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

		if count > 10 {
			break
		}

		time.Sleep(3 * time.Second)
	}
}
