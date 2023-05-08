// Copyright 2017-2020 Intel Corp.
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
	_ "flag"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	cniSpecVersion "github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"

	"github.com/intel/userspace-cni-network-plugin/cniovs"
	"github.com/intel/userspace-cni-network-plugin/cnivpp"
	"github.com/intel/userspace-cni-network-plugin/pkg/k8sclient"
	"github.com/intel/userspace-cni-network-plugin/logging"
	"github.com/intel/userspace-cni-network-plugin/pkg/types"
	"github.com/intel/userspace-cni-network-plugin/pkg/annotations"
	"github.com/intel/userspace-cni-network-plugin/pkg/configdata"

	_ "github.com/vishvananda/netlink"
)

var version = "master@git"
var commit = "unknown commit"
var date = "unknown date"

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

//
// Local functions
//

func printVersionString() string {
	return fmt.Sprintf("userspace-cni-network-plugin version:%s, commit:%s, date:%s",
		version, commit, date)
}

// loadNetConf() - Unmarshall the inputdata into the NetConf Structure
func loadNetConf(bytes []byte) (*types.NetConf, error) {
	netconf := &types.NetConf{}
	if err := json.Unmarshal(bytes, netconf); err != nil {
		return nil, fmt.Errorf("failed to load netconf: %v", err)
	}

	//
	// Logging
	//
	if netconf.LogFile != "" {
		logging.SetLogFile(netconf.LogFile)
	}
	if netconf.LogLevel != "" {
		logging.SetLogLevel(netconf.LogLevel)
	}

	//
	// Parse previous result
	//
	/*
	if netconf.RawPrevResult != nil {
		resultBytes, err := json.Marshal(netconf.RawPrevResult)
		if err != nil {
			return nil, logging.Errorf("could not serialize prevResult: %v", err)
		}
		res, err := cniSpecVersion.NewResult(netconf.CNIVersion, resultBytes)
		if err != nil {
			return nil, logging.Errorf("could not parse prevResult: %v", err)
		}
		netconf.RawPrevResult = nil
		netconf.PrevResult, err = current.NewResultFromResult(res)
		if err != nil {
			return nil, logging.Errorf("could not convert result to current version: %v", err)
		}
	}
	*/

	return netconf, nil
}

func getPodAndSharedDir(netConf *types.NetConf,
						args *skel.CmdArgs,
						kubeClient kubernetes.Interface) (kubernetes.Interface, *v1.Pod, string, error) {

	var found bool
	var pod *v1.Pod
	var sharedDir string
	var err error

	// Retrieve pod so any annotations from the podSpec can be inspected
	pod, kubeClient, err = k8sclient.GetPod(args, kubeClient, netConf.KubeConfig)
	if err != nil {
		logging.Debugf("getPodAndSharedDir: Failure to retrieve pod - %v", err)
	}

	// Retrieve the sharedDir from the Volumes in podSpec. Directory Socket
	// Files will be written to on host.
	if err == nil {
		sharedDir, err = annotations.GetPodVolumeMountHostSharedDir(pod)
		if err != nil {
			logging.Infof("getPodAndSharedDir: VolumeMount \"shared-dir\" not provided - %v", err)
		} else {
			found = true
		}
	}

	err = nil


	if found == false {
		if netConf.SharedDir != "" {
			if netConf.SharedDir[len(netConf.SharedDir)-1:] == "/" {
				sharedDir = fmt.Sprintf("%s%s/", netConf.SharedDir, args.ContainerID[:12])
			} else {
				sharedDir = fmt.Sprintf("%s/%s/", netConf.SharedDir, args.ContainerID[:12])
			}
		} else {
			if netConf.HostConf.Engine == "vpp" {
				sharedDir = fmt.Sprintf("%s/%s/", configdata.DefaultVppCNIDir, args.ContainerID[:12])
			} else if netConf.HostConf.Engine == "ovs-dpdk" {
				sharedDir = fmt.Sprintf("%s/%s/", configdata.DefaultOvsCNIDir, args.ContainerID[:12])
			} else {
				sharedDir = fmt.Sprintf("%s/%s/", annotations.DefaultBaseCNIDir, args.ContainerID[:12])
			}

			if netConf.KubeConfig == "" {
				logging.Warningf("getPodAndSharedDir: Neither \"KubeConfig\" nor \"SharedDir\" provided, defaulting to %s", sharedDir)
			} else {
				logging.Warningf("getPodAndSharedDir: \"KubeConfig\" invalid and \"SharedDir\" not provided, defaulting to %s", sharedDir)
			}	
		}
	}

	return kubeClient, pod, sharedDir, err
}

func cmdAdd(args *skel.CmdArgs, exec invoke.Exec, kubeClient kubernetes.Interface) error {
	var netConf *types.NetConf
	var containerEngine string

	vpp := cnivpp.CniVpp{}
	ovs := cniovs.CniOvs{}

	// Convert the input bytestream into local NetConf structure
	netConf, err := loadNetConf(args.StdinData)

	logging.Infof("cmdAdd: ENTER (AFTER LOAD) - Container %s Iface %s", args.ContainerID[:12], args.IfName)
	logging.Verbosef("   Args=%v netConf=%v, exec=%v, kubeClient%v",
		args, netConf, exec, kubeClient)

	if err != nil {
		logging.Errorf("cmdAdd: Parse NetConf - %v", err)
		return err
	}

	// Initialize returned Result

	// Multus will only copy Interface (i.e. ifName) into NetworkStatus
	// on Pod with Sandbox configured. Get Netns and populate in results.
	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netns, err)
	}
	defer netns.Close()

	result := &current.Result{}
	result.Interfaces = []*current.Interface{{
		Name:    args.IfName,
		Sandbox: netns.Path(),
	}}


	// Retrieve the "SharedDir", directory to create the socketfile in.
	// Save off kubeClient and pod for later use if needed.
	kubeClient, pod, sharedDir, err := getPodAndSharedDir(netConf, args, kubeClient)
	if err != nil {
		logging.Errorf("cmdAdd: Unable to determine \"SharedDir\" - %v", err)
		return err
	}


	//
	// HOST:
	//

	// Add the requested interface and network
	if netConf.HostConf.Engine == "vpp" {
		err = vpp.AddOnHost(netConf, args, kubeClient, sharedDir, result)
	} else if netConf.HostConf.Engine == "ovs-dpdk" {
		err = ovs.AddOnHost(netConf, args, kubeClient, sharedDir, result)
	} else {
		err = fmt.Errorf("ERROR: Unknown Host Engine:" + netConf.HostConf.Engine)
	}
	if err != nil {
		logging.Errorf("cmdAdd: Host ERROR - %v", err)
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
			logging.Errorf("cmdAdd: IPAM ERROR - %v", err)
			return err
		}

		// Convert whatever the IPAM result was into the current Result type
		newResult, err := current.NewResultFromResult(ipamResult)
		if err != nil {
			// TBD: CLEAN-UP
			logging.Errorf("cmdAdd: IPAM Result ERROR - %v", err)
			return err
		}

		if len(newResult.IPs) == 0 {
			// TBD: CLEAN-UP
			err = fmt.Errorf("ERROR: Unable to get IP Address")
			logging.Errorf("cmdAdd: IPAM ERROR - %v", err)
			return err
		}

		newResult.Interfaces = result.Interfaces
		//newResult.Interfaces[0].Mac = macAddr

		// Clear out the Gateway if set by IPAM, not being used.
		for _, ip := range newResult.IPs {
			ip.Gateway = nil
		}

		result = newResult
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
		_, err = vpp.AddOnContainer(netConf, args, kubeClient, sharedDir, pod, result)
	} else if containerEngine == "ovs-dpdk" {
		_, err = ovs.AddOnContainer(netConf, args, kubeClient, sharedDir, pod, result)
	} else {
		err = fmt.Errorf("ERROR: Unknown Container Engine:" + containerEngine)
	}
	if err != nil {
		logging.Errorf("cmdAdd: Container ERROR - %v", err)
		return err
	}

	return cnitypes.PrintResult(result, current.ImplementedSpecVersion)
}


func cmdGet(args *skel.CmdArgs, exec invoke.Exec, kubeClient kubernetes.Interface) error {
/*
	netConf, err := loadNetConf(args.StdinData)

	logging.Infof("cmdGet: (AFTER LOAD) - Container %s Iface %s", args.ContainerID[:12], args.IfName)
	logging.Verbosef("   Args=%v netConf=%v, exec=%v, kubeClient%v",
		args, netConf, exec, kubeClient)

	if err != nil {
		return err
	}

	// FIXME: call all delegates

	return cnitypes.PrintResult(netConf.PrevResult, netConf.CNIVersion)
*/
	return nil
}


func cmdDel(args *skel.CmdArgs, exec invoke.Exec, kubeClient kubernetes.Interface) error {
	var netConf *types.NetConf
	var containerEngine string

	vpp := cnivpp.CniVpp{}
	ovs := cniovs.CniOvs{}

	// Convert the input bytestream into local NetConf structure
	netConf, err := loadNetConf(args.StdinData)

	logging.Infof("cmdDel: ENTER (AFTER LOAD) - Container %s Iface %s", args.ContainerID[:12], args.IfName)
	logging.Verbosef("   Args=%v netConf=%v, exec=%v, kubeClient%v",
		args, netConf, exec, kubeClient)

	if err != nil {
		logging.Errorf("cmdDel: Parse NetConf - %v", err)
		return err
	}


	// Retrieve the "SharedDir", directory to create the socketfile in.
	// Save off kubeClient and pod for later use if needed.
	_, pod, sharedDir, err := getPodAndSharedDir(netConf, args, kubeClient)
	if err != nil {
		logging.Errorf("cmdDel: Unable to determine \"SharedDir\" - %v", err)
		return err
	}


	//
	// HOST:
	//

	// Delete the requested interface
	if netConf.HostConf.Engine == "vpp" {
		err = vpp.DelFromHost(netConf, args, sharedDir)
	} else if netConf.HostConf.Engine == "ovs-dpdk" {
		err = ovs.DelFromHost(netConf, args, sharedDir)
	} else {
		err = fmt.Errorf("ERROR: Unknown Host Engine:" + netConf.HostConf.Engine)
	}
	if err != nil {
		logging.Errorf("cmdDel: Host ERROR - %v", err)
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
		err = vpp.DelFromContainer(netConf, args, sharedDir, pod)
	} else if containerEngine == "ovs-dpdk" {
		err = ovs.DelFromContainer(netConf, args, sharedDir, pod)
	} else {
		err = fmt.Errorf("ERROR: Unknown Container Engine:" + containerEngine)
	}
	if err != nil {
		logging.Errorf("cmdDel: Container ERROR - %v", err)
		return err
	}

	//
	// Cleanup IPAM data, if provided.
	//
	if netConf.IPAM.Type != "" {
		err = ipam.ExecDel(netConf.IPAM.Type, args.StdinData)
		if err != nil {
			logging.Errorf("cmdDel: IPAM ERROR - %v", err)
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
		_, err = ip.DelLinkByNameAddr(args.IfName)
		if err != nil && err == ip.ErrLinkNotFound {
			return nil
		}
		return err
	})

	return err
}

func main() {
	// Init command line flags to clear vendored packages' one, especially in init()
	//flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// add version flag
	//versionOpt := false
	//flag.BoolVar(&versionOpt, "version", false, "Show application version")
	//flag.BoolVar(&versionOpt, "v", false, "Show application version")
	//flag.Parse()
	//if versionOpt == true {
	//	fmt.Printf("%s\n", printVersionString())
	//	return
	//}

	// Extend the cmdAdd(), cmdGet() and cmdDel() functions to take
	// 'exec invoke.Exec' and 'kubeClient k8s.KubeClient' as input
	// parameters. They are passed in as nill from here, but unit test
	// code can then call these functions directly and fake out a
	// Kubernetes Client.
	skel.PluginMain(
		func(args *skel.CmdArgs) error {
			return cmdAdd(args, nil, nil)
		},
		func(args *skel.CmdArgs) error {
			return cmdGet(args, nil, nil)
		},
		func(args *skel.CmdArgs) error {
			return cmdDel(args, nil, nil)
		},
		cniSpecVersion.All,
		"CNI plugin that manages DPDK based interfaces")
}
