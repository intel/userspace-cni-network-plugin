/*
 * Copyright(c) 2022 Intel Corporation.
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package fuzz

import (
	"strings"
	"testing"

	"github.com/intel/userspace-cni-network-plugin/userspace/cni"
)

func FuzzLoadNetConf(f *testing.F) {

	seed := `
        {
                "cniVersion": "0.3.1",
                "type": "userspace",
                "name": "userspace-ovs-net-1",
                "kubeconfig": "/etc/cni/net.d/multus.d/multus.kubeconfig",
                "logFile": "/var/log/userspace-ovs-net-1-cni.log",
                "logLevel": "debug",
                "host": {
                        "engine": "ovs-dpdk",
                        "iftype": "vhostuser",
                        "netType": "bridge",
                        "vhost": {
                                "mode": "client"
                        },
                        "bridge": {
                                "bridgeName": "br-4"
                        }
                },
                "container": {
                        "engine": "ovs-dpdk",
                        "iftype": "vhostuser",
                        "netType": "interface",
                        "vhost": {
                                "mode": "server"
                        }
                },
                "ipam": {
                        "type": "host-local",
                        "subnet": "10.56.217.0/24",
                        "rangeStart": "10.56.217.131",
                        "rangeEnd": "10.56.217.190",
                        "routes": [
                                {
                                        "dst": "0.0.0.0/0"
                                }
                        ],
                        "gateway": "10.56.217.1"
                }
            }
        `

	f.Add([]byte(seed))

	f.Fuzz(func(t *testing.T, fcfg []byte) {
		_, err := cni.LoadNetConf(fcfg)
		if err != nil {
			if strings.Contains(err.Error(), "failed to load netconf:") {
				return
			} else {
				t.Errorf("Error: %s, for input %s", err.Error(), string(fcfg))
			}

		} else {
			return
		}
	})
}
