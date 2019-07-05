// Copyright 2017 Intel Corp.
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

package usrsptypes

import (
	"net"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
)

//
// Exported Types
//
type MemifConf struct {
	Role          string `json:"role,omitempty"`          // Role of memif: master|slave
	Mode          string `json:"mode,omitempty"`          // Mode of memif: ip|ethernet|inject-punt
}

type VhostConf struct {
	Mode          string `json:"mode,omitempty"`          // vhost-user mode: client|server
}

type BridgeConf struct {
	// ovs-dpdk specific note:
	//   ovs-dpdk requires a bridge to create an interfaces. So if 'NetType' is set
	//   to something other than 'bridge', a bridge is still need and this field will
	//   be inspected. For ovs-dpdk, if bridge data is not populated, it will default
	//   to 'br-0'. 
	BridgeName string `json:"bridgeName,omitempty"` // Bridge Name
	BridgeId   int    `json:"bridgeId,omitempty"`   // Bridge Id - Deprecated in favor of BridgeName
	VlanId     int    `json:"vlanId,omitempty"`     // Optional VLAN Id
}

type UserSpaceConf struct {
	// The Container Instance will default to the Host Instance value if a given attribute
	// is not provided. However, they are not required to be the same and a Container
	// attribute can be provided to override. All values are listed as 'omitempty' to
	// allow the Container struct to be empty where desired.
	Engine     string     `json:"engine,omitempty"`  // CNI Implementation {vpp|ovs-dpdk}
	IfType     string     `json:"iftype,omitempty"`  // Type of interface {memif|vhostuser}
	NetType    string     `json:"netType,omitempty"` // Interface network type {none|bridge|interface}
	MemifConf  MemifConf  `json:"memif,omitempty"`
	VhostConf  VhostConf  `json:"vhost,omitempty"`
	BridgeConf BridgeConf `json:"bridge,omitempty"`
}

type NetConf struct {
	types.NetConf

	// Support chaining
	RawPrevResult *map[string]interface{} `json:"prevResult"`
	PrevResult    *current.Result         `json:"-"`
	KubeConfig    string                  `json:"kubeconfig"`

	Name          string        `json:"name"`
	HostConf      UserSpaceConf `json:"host,omitempty"`
	ContainerConf UserSpaceConf `json:"container,omitempty"`
	LogFile       string        `json:"logFile,omitempty"`
	LogLevel      string        `json:"logLevel,omitempty"`
}

// UnmarshallableString typedef for builtin string
type UnmarshallableString string

// K8sArgs is the valid CNI_ARGS used for Kubernetes
type K8sArgs struct {
	types.CommonArgs
	IP                         net.IP
	K8S_POD_NAME               types.UnmarshallableString
	K8S_POD_NAMESPACE          types.UnmarshallableString
	K8S_POD_INFRA_CONTAINER_ID types.UnmarshallableString
}

