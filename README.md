# Vhostuser CNI plugin

- It is a vhostuser plugin in Kubernetes and provides accelerate DPDK workloads in container networking

- DPDK Vhostuser is new virtualization technology. Please refer to [here](http://dpdk.org/doc/guides/howto/virtio_user_for_container_networking.html) for more information.

## Build & Clean

This plugin is recommended to build with Go 1.7.5 which is fully tested. Other versions of Go are theoretically supported, but MIGHT cause unknown issue, please try to fix it by yourself.

```
#./build
```

Build the source codes to binary, copy the bin/vhostuser to the CNI folder for the tests.

```
#./clean
```

Remove the binary and temporary files generated whild building the source codes.

## Network configuration reference

* `name` (string, required): the name of the network
* `type` (string, required): "vhostuser"
* `if0name` (string, required): name of the virtual interface
* `vhost` (dictionary, required): Vhostuser configurations.
* `ipam` (dictionary, required): IPAM configuration to be used for this network.

## Usage

### Work standalone

Given the following network configuration:

```
# cat > /etc/cni/net.d/99-vhostuser.conf <<EOF
{
	"type": "vhostuser",
    	"name": "vhostuser-network",
	"if0name": "net0",
	"vhost": {
		"vhost_tool": "/path/to/vhost-user-net-plugin/tests/vpp-config.py"
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
EOF
```

### Integrated with Multus plugin

```
# cat > /etc/cni/net.d/10-multus.conf <<EOF
{
    "name": "multus-demo-network",
    "type": "multus",
    "delegates": [
        {
                "type": "sriov",
                "if0": "ens786f1",
		"if0name": "net0",
		"dpdk": {
			"kernel_driver": "ixgbevf",
			"dpdk_driver": "igb_uio",
			"dpdk_tool": "/path/to/dpdk/tools/dpdk-devbind.py"
		}
        },
        {
		"type": "vhostuser",
    		"name": "vhostuser-network",
		"if0name": "net1",
		"vhost": {
			"vhost_tool": "/path/to/vhost-user-net-plugin/tests/vpp-config.py"
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
        },
        {
                "type": "flannel",
    		"name": "control-network",
                "masterplugin": true,
                "delegate": {
                        "isDefaultGateway": true
    		}
        }
    ]
}
EOF
```

Note: The Vhostuser CNI supports different IPAM plugins for the IP addresses management. The generated IP address information will be stored in one configuration file.

```
# tree /var/lib/cni/vhostuser
/var/lib/cni/vhostuser
├── 4d578250ad8d760c0722be78badb4b4b6d57fed8f95dea23aaa0065aa8657b29
│   ├── 4d578250ad8d-net1
│   ├── 4d578250ad8d-net1-ip4.conf
│   └── 4d578250ad8d-net1.json
├── 65bc360690b648458b7cbad34f8f274b6028973e82a284353d9c3ca63e1ad35e
│   ├── 65bc360690b6-net1
│   ├── 65bc360690b6-net1-ip4.conf
│   └── 65bc360690b6-net1.json
└── get-prefix.sh
```

Shows that there are two vhostuser ports, each for one container.
* xxxxxxxxxxxx-net1: The socket file for the Vhostuser server/client communication.
* xxxxxxxxxxxx-net1-ip4.conf: IPAM information for the Vhostuser port.
* xxxxxxxxxxxx-net1.json: Vhostuser Port information for the management.

```
# cat 4d578250ad8d-net1-ip4.conf
{
        "ipAddr": "10.56.217.132/32",
        "macAddr": "e2:52:b5:7b:58:ad",
        "gateway": "10.56.217.1",
        "gwMac": "02:fe:fc:89:49:d8"
}
```

The IPAM management configuration for the port. 

```
# cat 4d578250ad8d-net1.json
{
    "vhostname": "VirtualEthernet0/0/0",
    "vhostmac": "02:fe:fc:89:49:d8",
    "ifname": "net1",
    "ifmac": "e2:52:b5:7b:58:ad",
    "vhost_tool": "/path/to/vhost-user-net-plugin/tests/vpp-config.py"
}
```

Login the container and run the VPP application:

```
$ /vhost-user-net-plugin/get-prefix.sh
4d578250ad8d760c0722be78badb4b4b6d57fed8f95dea23aaa0065aa8657b29
```

This container should use socket file/configuration file under the folder
/vhost-user-net-plugin/4d578250ad8d760c0722be78badb4b4b6d57fed8f95dea23aaa0065aa8657b29 .

Run the VPP in a container A as follows

```
# vpp unix {log /tmp/vpp.log cli-listen 0.0.0.0:5002} api-trace { on } \

  dpdk {coremask 0x2 no-multi-seg no-pci singlefile 512 extra --vdev=virtio_user0,path=/vhost-user-net-plugin/4d578250ad8d760c0722be78badb4b4b6d57fed8f95dea23aaa0065aa8657b29/4d578250ad8d-net1,mac=e2:52:b5:7b:58:ad } cpu {skip-cores 1}
# vppctl set int state virtio_user0 up
# vppctl set int ip table virtio_user0 0
```

Run the VPP in another container B and ping the Container A

```
# vppctl ping 10.56.217.132
```

If the system works well, the ping would be successful

### Contacts
For any questions about Acceleratus CNI, please reach out on github issue or feel free to contact the developer @john and @kural in our [Intel-Corp Slack](https://intel-corp.herokuapp.com/)
