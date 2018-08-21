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
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
)

//
// Exported Types
//
type UsrSpCni interface {
	AddOnHost(conf *NetConf, containerID string, ipResult *current.Result) error
	AddOnContainer(conf *NetConf, containerID string, ipResult *current.Result) error
	DelFromHost(conf *NetConf, containerID string) error
	DelFromContainer(conf *NetConf, containerID string) error
}

type MemifConf struct {
	Role string `json:"role"` // Role of memif: master|slave
	Mode string `json:"mode"` // Mode of memif: ip|ethernet|inject-punt
}

type VhostConf struct {
	Mode string `json:"mode"` // vhost-user mode: client|server
}

type BridgeConf struct {
	BridgeId int `json:"bridgeId"`         // Bridge Id
	VlanId   int `json:"vlanId,onitempty"` // Optional VLAN Id
}

type UserSpaceConf struct {
	// The Container Instance will default to the Host Instance value if a given attribute
	// is not provided. However, they are not required to be the same and a Container
	// attribute can be provided to override. All values are listed as 'omitempty' to
	// allow the Container struct to be empty where desired.
	Engine     string     `json:"engine,omitempty"`  // CNI Implementation {vpp|ovs|ovs-dpdk|linux}
	IfType     string     `json:"iftype,omitempty"`  // Type of interface {memif|vhostuser|veth|tap}
	NetType    string     `json:"netType,omitempty"` // Interface network type {none|bridge|interface}
	MemifConf  MemifConf  `json:"memif,omitempty"`
	VhostConf  VhostConf  `json:"vhost,omitempty"`
	BridgeConf BridgeConf `json:"bridge,omitempty"`
}

type NetConf struct {
	types.NetConf
	Name          string        `json:"name"`
	If0name       string        `json:"if0name,omitempty"` // Interface name
	HostConf      UserSpaceConf `json:"host,omitempty"`
	ContainerConf UserSpaceConf `json:"container,omitempty"`
}
