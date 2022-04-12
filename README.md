
   * [Summary](#summary)
      * [Links](#links)
      * [Weekly Meeting](#weekly-meeting)
      * [Contacts](#contacts)
   * [Userspace CNI plugin](#userspace-cni-plugin)
   * [Build & Clean](#build--clean)
      * [Update dependencies](#update-dependencies)
   * [Network Configuration Reference](#network-configuration-reference)
      * [Work Standalone](#work-standalone)
      * [Integrated with Multus Plugin](#integrated-with-multus-plugin)
         * [Multus Configuration Details](#multus-configuration-details)
      * [Logging Options](#logging-options)
         * [Writing to a Log File](#writing-to-a-log-file)
         * [Logging Level](#logging-level)
   * [OVS CNI Library Intro](#ovs-cni-library-intro)
      * [Installing OVS](#installing-ovs)
      * [Running OVS CNI Library with OVS](#running-ovs-cni-library-with-ovs)
      * [Configuring the System](#configuring-the-system)
   * [VPP CNI Library Intro](#vpp-cni-library-intro)
      * [Building VPP CNI Library with OVS](#building-vpp-cni-library-with-ovs)
      * [Installing VPP](#installing-vpp)
         * [Prerequisites](#prerequisites)
         * [Install on CentOS](#install-on-centos)
         * [Install on Ubuntu](#install-on-ubuntu)
         * [Install on Other Distros](#install-on-other-distros)
   * [Testing](#testing)
      * [Testing with VPP Docker Image and CNI](#testing-with-vpp-docker-image-and-cni)
         * [Verify Host](#verify-host)
         * [Verify Container](#verify-container)
         * [Ping](#ping)
         * [Debug](#debug)
      * [Testing with DPDK Testpmd Application](#testing-with-dpdk-testpmd-application)
         * [1. Build the image to be used](#1-build-the-image-to-be-used)
         * [2. Create pod with multiple vhostuser interfaces](#2-create-pod-with-multiple-vhostuser-interfaces)
         * [3. Open terminal to pod and start testpmd](#3-open-terminal-to-pod-and-start-testpmd)
   * [Unit Testing](#unit-testing)
      * [Unit Tests Overview](#unit-tests-overview)
      * [Unit Tests Inside Container](#unit-tests-inside-container)

# Summary
The Userspace CNI is a Container Network Interface (CNI) plugin
designed to implement userspace networking (as opposed to kernel
space networking). An example is any DPDK based applications. It
is designed to run with either OVS-DPDK or VPP along with the Multus
CNI plugin in Kubernetes deployments. It enhances high performance
container Networking solution and Data Plane Acceleration for containers.

## Links
Useful Links:
* Source Code: https://github.com/intel/userspace-cni-network-plugin
* Design Document: [Userspace Design Document](https://docs.google.com/document/d/1jAFDNWhf6flTlPHmbWavlyLrkFJtAdQlcOnG3qhRYtU/edit?usp=sharing) 

## Biweekly Meeting
Join us for project discussions at _K8s Network & Resource management_ meetings

Time: 15:00 - 16:00 GMT / 10:00-11:00 ET /  07:00-08:00 PST  every other Monday
* [Meeting notes and agenda](https://docs.google.com/document/d/1sJQMHbxZdeYJPgAWK1aSt6yzZ4K_8es7woVIrwinVwI/edit?usp=sharing)
* [Zoom channel](https://zoom.us/j/2392609689?pwd=K1R4ZEQyWWJVSGV3V1pmVThLMlhqQT09)

## Contacts
For any questions about Userspace CNI, please reach out.
* Report issue via Github: [userspace-cni-network-plugin/issues](https://github.com/intel/userspace-cni-network-plugin/issues)
* Contact via slack: [NPWG Slack](https://npwg-team.slack.com/)
  * Feel free to contact the developer @garyloug or @bmcfall in slack
* Contact via Google Group: https://groups.google.com/forum/#!forum/userspace-cni

# Userspace CNI Plugin
The Userspace CNI is a Container Network Interface (CNI) plugin designed to
implement userspace networking (as opposed to kernel space networking), like
DPDK based applications. It is designed to run with either OVS-DPDK or VPP
running on the host, along with the
[Multus CNI plugin](https://github.com/intel/multus-cni), which enables additional
interfaces in a container.

Userspace networking requires additional considerations. For one, the interface
needs to be created/configured on a local vswitch (running on the host). There
may also be a desire to add the interface to a specfic network on the host
through the local vswitch. Second, when the interface is inserted into the
container, it is not owned by the kernel, so additional work needs to be done
in the container to consume the interface and add to a network within the
container. The Userspace CNI is designed to work with these additional
considerations by provisioning the local vswitch (OVS-DPDK/VPP), and by pushing
data into the container so the interface can be consumed.

The Userspace CNI, based on the input config data, adds interfaces (memif and/or
vhost-user interfaces) to a local OVS-DPDK or VPP instance running on the host.
Then adds that interface to a local network, like an L2 Bridge. The Userspace CNI
then processes config data intended for the container and adds that data to a
Database the container can consume.

DPDK Vhostuser is new virtualization technology. Please refer to
[here](http://dpdk.org/doc/guides/howto/virtio_user_for_container_networking.html)
for more information.

![Vhostuser plugin](doc/images/userspace-plugin.png)


# Build & Clean

This plugin is recommended to be built with Go 1.11.10 and either OVS-DPDK 2.9.0-3
or VPP 19.04. Other versions of Go, OVS-DPDK and VPP are theoretically
supported, but MIGHT cause unknown issue.

There are a few environmental variables used in building and teating this plugin.
Here is an example:
```
   cat ~/.bashrc
   :
   export GOPATH=~/go
   export CNI_PATH=$GOPATH/src/github.com/containernetworking/plugins/bin
```


The Userspace CNI requires several files from VPP in-order to build. If VPP
should be installed but is not installed, see [Installing VPP](#installing-vpp)
section below for instructions. If Userspace CNI is being built on a build
server or is using OVS-DPDK (i.e. - don't want VPP installed), then follow the
instructions below under
[Building VPP CNI Library with OVS](#building-vpp-cni-library-with-ovs).

To get and build the Userspace CNI plugin:
```
   cd $GOPATH/src/
   go get github.com/intel/userspace-cni-network-plugin
   cd github.com/intel/userspace-cni-network-plugin
   make
```

Once the binary is built, it needs to be copied to the CNI directory:
```
   cp userspace/userspace $CNI_PATH/.
```

To remove the binary and temporary files generated while building the source
code, perform a make clean:
```
   make clean
```


## Update dependencies
This project is currently using [go modules](https://github.com/golang/go/wiki/Modules)
to manage dependencies. Please refer to official documentation to learn more
about **go modules** behavior and typical [workflow](https://github.com/golang/go/wiki/Modules#daily-workflow).


# Network Configuration Reference

* `type` (string, required): "userspace"
* `name` (string, required): Name of the network
* `host` (dictionary, required): Host based configurations. Contains userspace
interface configuration data as well as host network data userspace interface
should be injected into.
* `container` (dictionary, optional): Container based configurations. Contains
userspace interface configuration data as well as container network data
userspace interface should be injected into. Defaults used when data omitted.
* `ipam` (dictionary, optional): IPAM configuration to be used for this network.


## Work Standalone

Given the following network configuration:
```
sudo cat > /etc/cni/net.d/90-userspace.conf <<EOF
{
	"cniVersion": "0.3.1",
        "type": "userspace",
        "name": "memif-network",
        "host": {
                "engine": "vpp",
                "iftype": "memif",
                "netType": "bridge",
                "memif": {
                        "role": "master",
                        "mode": "ethernet"
                },
                "bridge": {
                        "bridgeName": "4"
                }
        },
        "container": {
                "engine": "vpp",
                "iftype": "memif",
                "netType": "interface",
                "memif": {
                        "role": "slave",
                        "mode": "ethernet"
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
EOF
```


## Integrated with Multus Plugin
Integrate with the Multus plugin for a high performance container networking
solution for NFV Environments. Refer to Multus (NFV based Multi - Network
plugin), DPDK-SRIOV CNI plugins:
* [Multus - Multi Network plugin](https://github.com/Intel-Corp/multus-cni)
* [DPDK-SRIOV - Dataplane plugin](https://github.com/Intel-Corp/sriov-cni)

Encourage the users/developers to use Multus based Kubernetes CDR/TPR based
network objects. Please follow the configuration details in the link:
[Usage with Kubernetes CRD/TPR based Network Objects](https://github.com/Intel-Corp/multus-cni/blob/master/README.md#usage-with-kubernetes-crdtpr-based-network-objects)

Please refer the Kubernetes Network SIG - Multiple Network PoC proposal for more
details refer the link:
[K8s Multiple Network proposal](https://docs.google.com/document/d/1TW3P4c8auWwYy-w_5afIPDcGNLK3LZf0m14943eVfVg/edit)

![Userspace CNI with multus](doc/images/userspace-with-multus.png)

### Multus Configuration Details
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
		"cniVersion": "0.3.1",
		"type": "userspace",
		"name": "memif-network",
		"host": {
			"engine": "vpp",
			"iftype": "memif",
			"netType": "bridge",
			"memif": {
				"role": "master",
				"mode": "ethernet"
			},
			"bridge": {
				"bridgeName": "4"
			}
		},
		"container": {
			"engine": "vpp",
			"iftype": "memif",
			"netType": "interface",
			"memif": {
				"role": "slave",
				"mode": "ethernet"
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

**Note:** The Userspace CNI supports different IPAM plugins for the IP addresses
management. The generated IP address information will be stored in one
configuration file.

## Logging Options
You may wish to enable some enhanced logging, especially to understand what is
or isn't working with a particular configuration. Userspace CNI always log via
`STDERR`, which is the standard method by which CNI plugins communicate errors,
and these errors are logged by the Kubelet. This method is always enabled.

### Writing to a Log File
Optionally, Userspace CNI can log to a file on the filesystem. This file will
be written locally on each node where CNI is executed. Configure this via the
`LogFile` option in the CNI configuration. By default this additional logging
to a flat file is disabled.

For example in your CNI configuration, you may set:
```
    "LogFile": "/var/log/userspace-cni.log",
```

### Logging Level

The default logging level is set as `warning` -- this will log critical errors
and issues detect.

The available logging level values, in increasing order of verbosity are:

* `panic`
  * Code exiting immediately.
* `error`
  * Unusual event occurred (invalid input or system issue), so exiting code prematurely.
* `warning`
  * Unusual event occurred (invalid input or system issue), but continuing with default value.
* `info`
  * Basic information, indication of major code paths.
* `debug`
  * Additional information, indication of minor code branches.
* `verbose`
  * Output of larger variables in code and debug of low level functions.

You may configure the logging level by using the `LogLevel` option in your
CNI configuration. For example:

```
    "LogLevel": "debug",
```


# OVS CNI Library Intro
OVS CNI Library is written in GO and used by UserSpace CNI to interface with the
OVS. OVS currently does not have a GO-API, though there are some external
packages that are being explored. When the CNI is invoked, OVS CNI library
builds up an OVS CLI command (ovs-vsctl) and executes the request.

## Installing OVS
To install the DPDK-OVS, the source codes contains a
[document](https://github.com/openvswitch/ovs/blob/master/Documentation/intro/install/dpdk.rst)
for how to install the DPDK-OVS.

## Running OVS CNI Library with OVS
The Userspace CNI plugin builds the OVS CNI Library from the cniovs
sub-folder. In order to run with the OVS CNI Library, the OVS python script
must be installed on the system. To install the script, run *make install*
as described in
[Building VPP CNI Library with OVS](#building-vpp-cni-library-with-ovs). 

## Configuring the System
DPDK-OVS is a DPDK based application, so some detailed system requirements can
be found at
[DPDK requirements](http://dpdk.org/doc/guides/linux_gsg/sys_reqs.html).
Hugepages are the main requirement for the VHOST_USER virtual ports.
```
echo 'vm.nr_hugepages=2048' > /etc/sysctl.d/hugepages.conf
```
Or add the following configuration to the grub configuration:
```
default_hugepagesz=2m hugepagesz=2m hugepages=2048
```


# VPP CNI Library Intro
VPP CNI Library is written in GO and used by UserSpace CNI to interface with the
VPP GO-API. When the CNI is invoked, VPP CNI library opens a GO Channel to the
local VPP instance and passes gRPC messages between the two.

As mentioned above, to build the Userspace CNI, VPP needs to be installed, or
several VPP files to compile against. When VPP is installed, it copies it's
json API files to */usr/share/vpp/api/*. VPP CNI Libary uses these files to
compile against and generate the properly versioned messages to the local VPP
Instance. So to build the VPP CNI, VPP must be installed (or the proper json
files must be in */usr/share/vpp/api/*).


## Building VPP CNI Library with OVS
The Userspace CNI plugin builds the VPP CNI Library from the cnivpp
sub-folder. In order to run with the VPP CNI Library, VPP must be installed
on the system. If VPP should be installed but is not installed, see the
[Installing VPP](#installing-vpp) section below.

If the desire is to run the OVS CNI Library with OVS (i.e. - don't want
VPP installed), several files from a typical VPP install need to be on
the system to build. To install just these files and NOT VPP, run:
```
   cd $GOPATH/src/
   go get github.com/Billy99/user-space-net-plugin
   cd github.com/Billy99/user-space-net-plugin
   make install
```
This will install only the 5 or 6 files needed to build the VPP CNI Library.
To remove these files, run:
```
   make clean
```
*make install* requires several packages to execute, primarily *wget*,
*cpio* and *rpm2cpio* on CentOS, and *binutils* on Ubuntu. If these packages are
not installed on your system, the following can be run to install the required
packages:
```
   make install-dep
```
**NOTE:** *make install* has been made to work for CentOS and Ubuntu based
systems. Other platforms will be made to work long term. If there is an
immediate need for other platforms, please open an issue to expedite the
feature (https://github.com/intel/userspace-cni-network-plugin/issues).


## Installing VPP
There are several ways to install VPP. This code is based on a fixed release
VPP (VPP 19.04 initially), so it is best to install a released version (even
though it is possible to build your own).


### Prerequisites
Below are the VPP prerequisites:
* **Hugepages:** VPP requires 2M Hugepages. By default, VPP uses 1024
hugepages. If hugepages are not configured, on install VPP will allocate
them. This is primarily an issue if you are running in a VM that does not
already have hugepage backing, especially when you reboot the VM. If you
would like to change the number of hugepages VPP uses, after installing VPP,
edit */etc/sysctl.d/80-vpp.conf*. However, once VPP has been installed, the
default value has been applied. As an example, to reduce the number of
hugepages to 512, use:
```
   vm.nr_hugepages=512  
   vm.max_map_count=2048  
   kernel.shmmax=1073741824
```  
* **SELinux:** VPP works with SELinux enabled, but when running with
containers, work still needs to be done. Set SELinux to permissive.


### Install on CentOS
To install VPP on CentOS from https://packagecloud.io/fdio/ repository:
```
curl -s https://packagecloud.io/install/repositories/fdio/1904/script.rpm.sh | sudo bash
```
To resolve dependency issues, install the following packages:
```
yum install -y epel-release mbedtls python36
```
Installing RPMs:
```
yum install -y vpp vpp-lib vpp-plugins vpp-devel vpp-api-python vpp-api-lua vpp-selinux-policy
```

To start and enable VPP:
```
sudo systemctl start vpp
sudo systemctl enable vpp
```

### Install on Ubuntu
**OLD - Needs to be updated!**

To install on Ubuntu 16.04 (Xenial) as an example to demonstrate how to install VPP from pre-build packages:
```
export UBUNTU="xenial"
export RELEASE=".stable.1904"
sudo rm /etc/apt/sources.list.d/99fd.io.list
echo "deb [trusted=yes] https://nexus.fd.io/content/repositories/fd.io$RELEASE.ubuntu.$UBUNTU.main/ ./" | sudo tee -a /etc/apt/sources.list.d/99fd.io.list
sudo apt-get update
sudo apt-get install vpp vpp-lib
```


### Install on Other Distros
For installing VPP on other distros, see:
https://wiki.fd.io/view/VPP/Installing_VPP_binaries_from_packages



# Testing

## Testing with VPP Docker Image and CNI

There are a few environmental variables used in this test. Here is an example:
```
   cat ~/.bashrc
   :
   export GOPATH=~/go
   export CNI_PATH=$GOPATH/src/github.com/containernetworking/plugins/bin

```

In order to test, a container with VPP 19.04 and usrsp-app has been created:
```
  docker pull bmcfall/vpp-centos-userspace-cni:latest
```
More details on the Docker Image, how to build from scratch and other
information, see
[README.md](https://github.com/intel/userspace-cni-network-plugin/blob/master/docker/vpp-centos-userspace-cni/README.md)
in the '*./docker/vpp-centos-userspace-cni/*' subfolder.

Setup your configuration file in your CNI directory. An example is
*/etc/cni/net.d/*.

**NOTE:** The *userspace* netconf definition is still a work in progress. So
the example below is just an example, see *pkg/types/types.go* for latest definitions.

Example of how to setup a configuration for a VPP memif interface between the
host and container:
```
sudo vi /etc/cni/net.d/90-userspace.conf 
{
	"cniVersion": "0.3.1",
        "type": "userspace",
        "name": "memif-network",
        "host": {
                "engine": "vpp",
                "iftype": "memif",
                "netType": "bridge",
                "memif": {
                        "role": "master",
                        "mode": "ethernet"
                },
                "bridge": {
                        "bridgeName": "4"
                }
        },
        "container": {
                "engine": "vpp",
                "iftype": "memif",
                "netType": "interface",
                "memif": {
                        "role": "slave",
                        "mode": "ethernet"
                }
        },
        "ipam": {
                "type": "host-local",
                "subnet": "192.168.210.0/24",
                "routes": [
                        { "dst": "0.0.0.0/0" }
                ]
        }
}
```

To test, currently using a local script (copied from CNI scripts):
https://github.com/containernetworking/cni/blob/master/scripts/docker-run.sh).
To run script:
```
   cd $GOPATH/src/github.com/intel/userspace-cni-network-plugin/
   sudo CNI_PATH=$CNI_PATH GOPATH=$GOPATH ./scripts/usrsp-docker-run.sh -it --privileged vpp-centos-userspace-cni
```

**NOTE:** The *usrsp-docker-run.sh* script mounts some volumes in the container. Change as needed:
* *-v /var/lib/cni/usrspcni/shared:/var/lib/cni/usrspcni/shared:rw*
  * Socket files (memif or vhost-user) are passed to the container through a subdirectory of this base directory..
* *-v /var/lib/cni/usrspcni/$contid:/var/lib/cni/usrspcni/data:rw*
  * Current implementation is to write the remote configuration into a file and share the directory
with the container, which is the volume mapping. Directory is currently hard coded.
* *--device=/dev/hugepages:/dev/hugepages*
  * VPP requires hugepages, so need to map hugepoages into container.

In the container, you should see the usrsp-app ouput the message sequence of
its communication with local VPP (VPP in the container) and some database
dumps interleaved.

### Verify Host
To verify the local config (on host) in another window:

**Before Container Started:**
```
vppctl show interface
              Name               Idx       State          Counter          Count     
GigabitEthernet0/8/0              1        down      
GigabitEthernet0/9/0              2        down      
local0                            0        down      
   
vppctl show mode
l3 local0  
l3 GigabitEthernet0/8/0  
l3 GigabitEthernet0/9/0  
   
vppctl show memif
sockets
  id  listener    filename
  0   no          /run/vpp/memif.sock
```

After the container is started, on the host there should be an additional memif
interface created and added to a new L2 bridge, all created by the Userspace
CNI.

**After Container Started:**
```
vppctl show interface
              Name               Idx       State          Counter          Count
GigabitEthernet0/8/0              1        down
GigabitEthernet0/9/0              2        down
local0                            0        down
memif1/0                          3         up
   
vppctl show mode
l3 local0
l3 GigabitEthernet0/8/0
l3 GigabitEthernet0/9/0
l2 bridge memif1/0 bd_id 4 shg 0
   
vppctl show memif
sockets
  id  listener    filename
  0   no          /run/vpp/memif.sock
  1   yes (1)     /var/run/vpp/cni/shared/memif-79b661b189b2-net0.sock
   
interface memif1/0
  remote-name "VPP 18.07-16~gca7a68e~b66"
  remote-interface "memif1/0"
  socket-id 1 id 0 mode ethernet
  flags admin-up connected
  listener-fd 22 conn-fd 23
  num-s2m-rings 1 num-m2s-rings 1 buffer-size 0 num-regions 1
  region 0 size 4227328 fd 24
    master-to-slave ring 0:
      region 0 offset 16512 ring-size 1024 int-fd 26
      head 1024 tail 0 flags 0x0001 interrupts 0
    slave-to-master ring 0:
      region 0 offset 0 ring-size 1024 int-fd 25
      head 0 tail 0 flags 0x0001 interrupts 0
```

### Verify Container
The container is setup to start VPP, read the config pushed by the User Space
CNI, apply the data and then exit to bash. To verify the container config, in
the container, run the following:
```
vppctl show interface
              Name               Idx       State          Counter          Count     
local0                            0        down      
memif1/0                          1         up       
   
vppctl show interface addr
local0 (dn):
memif1/0 (up):
  L3 192.168.210.45/24
   
vppctl show mode          
l3 local0  
l3 memif1/0  
   
vppctl show memif
sockets
  id  listener    filename
  0   no          /run/vpp/memif.sock
  1   no          /var/run/vpp/cni/shared/memif-05138381d803-net0.sock
   
interface memif1/0
  remote-name "VPP 18.07-rc2~11-g18bde8a"
  remote-interface "memif1/0"
  socket-id 1 id 0 mode ethernet
  flags admin-up slave connected
  listener-fd 0 conn-fd 17
  num-s2m-rings 1 num-m2s-rings 1 buffer-size 2048 num-regions 1
  region 0 size 4227328 fd 18
    slave-to-master ring 0:
      region 0 offset 0 ring-size 1024 int-fd 19
      head 0 tail 0 flags 0x0001 interrupts 0
    master-to-slave ring 0:
      region 0 offset 16512 ring-size 1024 int-fd 20
      head 1024 tail 0 flags 0x0001 interrupts 0
```

### Ping
If a second container is started on the same host, the two containers will have
L3 connectivity through an L2 bridge on the host. Because the interfaces in the
host are owned by VPP, use VPP in one of the containers to ping between the two
containers.
```
vppctl ping 192.168.210.46   
64 bytes from 192.168.210.46: icmp_seq=2 ttl=64 time=39.0119 ms
64 bytes from 192.168.210.46: icmp_seq=3 ttl=64 time=37.9991 ms
64 bytes from 192.168.210.46: icmp_seq=4 ttl=64 time=57.0304 ms
64 bytes from 192.168.210.46: icmp_seq=5 ttl=64 time=40.0044 ms
   
Statistics: 5 sent, 4 received, 20% packet loss
```

### Debug
The *vpp-centos-userspace-cni* container runs a script at startup (in Dockefile
CMD command) which starts VPP and then runs *usrsp-app*. Assuming the same notes
above, to see what is happening in the container, cause
*vpp-centos-userspace-cni* container to start in bash and skip the script, then
run VPP and *usrsp-app* manually: 
```
   cd $GOPATH/src/github.com/containernetworking/cni/scripts
   sudo CNI_PATH=$CNI_PATH GOPATH=$GOPATH ./scripts/usrsp-docker-run.sh -it --privileged bmcfall/vpp-centos-userspace-cni:0.2.0 bash
   
   /* Within Container: */
   vpp -c /etc/vpp/startup.conf &
   usrsp-app
```


## Testing with DPDK Testpmd Application

To follow this example you should have a system with kubernetes available and
configured to support native 1 GB hugepages. You should also have multus-cni and
userspace-cni-network-plugin up and running. See `examples/crd-userspace-net-ovs-no-ipam.yaml` for
example config to use with multus. If using OVS,
check that you have bridge named `br0` in your OVS with `ovs-vsctl show` and if
not, create it with
`ovs-vsctl add-br br0 -- set bridge br0 datapath_type=netdev`.

### 1. Build the image to be used

Build container image from

```Dockerfile
FROM ubuntu:bionic

RUN apt-get update && apt-get install -y dpdk;

ENTRYPOINT ["bash"]
```

Dockerfile and tag it as `ubuntu-dpdk`:

```bash
docker build . -t ubuntu-dpdk
```

### 2. Create pod with multiple vhostuser interfaces

Copy `get-prefix.sh` script from userspace-cni-network-plugin repo to
`/var/lib/cni/vhostuser/`. See `examples/pod-multi-vhost.yaml`and start the
pod:

```bash
kubectl create -f examples/pod-multi-vhost.yaml
```

### 3. Open terminal to pod and start testpmd

Open terminal to the created pod once it is running:

```bash
kubectl exec -it multi-vhost-example bash
```

Launch testpmd and automatically start forwarding packets after sending first
burst:

```bash
# Get container ID
export ID=$(/vhu/get-prefix.sh)

# Run testpmd with ports created by vhostplugin
# Note: change coremask to suit your system
testpmd \
    -d librte_pmd_virtio.so.17.11 \
    -m 1024 \
    -c 0xC \
    --file-prefix=testpmd_ \
    --vdev=net_virtio_user0,path=/vhu/${ID}/${ID:0:12}-net1 \
    --vdev=net_virtio_user1,path=/vhu/${ID}/${ID:0:12}-net2 \
    --no-pci \
    -- \
    --no-lsc-interrupt \
    --auto-start \
    --tx-first \
    --stats-period 1 \
    --disable-hw-vlan;
```

If packets are not going through, you may need to configure direct flows to your
switch between the used ports. For example, with OVS as the switch, this is done
by getting the port numbers with `ovs-ofctl dump-ports br0` and configuring
flow, for example, from port 1 to port 2 with
`ovs-ofctl add-flow br0 in_port=1,action=output:2` and vice versa.

## Testing with DPDK L3FWD Application

To follow this example you should have a system with kubernetes available and configured to support native 1 GB hugepages. You should also have multus-cni and userspace-cni-network-plugin up and running. See `examples/crd-userspace-net-ovs-no-ipam.yaml` for example config to use with multus. If using OVS, check that you have bridge named `br0` in your OVS with `ovs-vsctl show` and if not, create it with `ovs-vsctl add-br br0 -- set bridge br0 datapath_type=netdev`.

### 1. Build the images to be used

Create the l3fwd Dockerfile:
```bash
$ cat <<EOF > l3fwd-dockerfile
FROM ubuntu:bionic

RUN apt-get update && apt-get install -y --no-install-recommends \\
  build-essential make wget vim dpdk libnuma-dev \\
  && apt-get clean && rm -rf /var/lib/apt/lists/*

ENV DPDK_VERSION=17.11.3 \\
  RTE_SDK=/usr/src/dpdk \\
  RTE_TARGET=x86_64-native-linuxapp-gcc

RUN wget http://fast.dpdk.org/rel/dpdk-\${DPDK_VERSION}.tar.xz && tar xf dpdk-\${DPDK_VERSION}.tar.xz && \\
  mv dpdk-stable-\${DPDK_VERSION} \${RTE_SDK}

RUN sed -i s/CONFIG_RTE_EAL_IGB_UIO=y/CONFIG_RTE_EAL_IGB_UIO=n/ \${RTE_SDK}/config/common_linuxapp \\
  && sed -i s/CONFIG_RTE_LIBRTE_KNI=y/CONFIG_RTE_LIBRTE_KNI=n/ \${RTE_SDK}/config/common_linuxapp \\
  && sed -i s/CONFIG_RTE_KNI_KMOD=y/CONFIG_RTE_KNI_KMOD=n/ \${RTE_SDK}/config/common_linuxapp

RUN cd \${RTE_SDK} && make install T=\${RTE_TARGET} && make -C examples

WORKDIR \${RTE_SDK}/examples/l3fwd/\${RTE_TARGET}/app/
EOF
```

Build the l3fwd Docker image, tag it as `dpdk-l3fwd`:
```bash
docker build -t dpdk-l3fwd -f l3fwd-dockerfile .
```

Create the pktgen Dockerfile:
```bash
$ cat <<EOF > pktgen-dockerfile
FROM debian:jessie

RUN apt-get update && apt-get install -y --no-install-recommends \\
  gcc build-essential make wget curl git liblua5.2-dev libedit-dev libpcap-dev libncurses5-dev libncursesw5-dev pkg-config vim libnuma-dev ca-certificates \\
  && apt-get clean && rm -rf /var/lib/apt/lists/*

ENV DPDK_VERSION=17.11.3 \\
  RTE_SDK=/usr/src/dpdk \\
  RTE_TARGET=x86_64-native-linuxapp-gcc \\
  PKTGEN_COMMIT=pktgen-3.4.8 \\
  PKTGEN_DIR=/usr/src/pktgen

RUN wget http://fast.dpdk.org/rel/dpdk-\${DPDK_VERSION}.tar.xz && tar xf dpdk-\${DPDK_VERSION}.tar.xz && \\
  mv dpdk-stable-\${DPDK_VERSION} \${RTE_SDK}

RUN sed -i s/CONFIG_RTE_EAL_IGB_UIO=y/CONFIG_RTE_EAL_IGB_UIO=n/ \${RTE_SDK}/config/common_linuxapp \\
  && sed -i s/CONFIG_RTE_LIBRTE_KNI=y/CONFIG_RTE_LIBRTE_KNI=n/ \${RTE_SDK}/config/common_linuxapp \\
  && sed -i s/CONFIG_RTE_KNI_KMOD=y/CONFIG_RTE_KNI_KMOD=n/ \${RTE_SDK}/config/common_linuxapp

RUN sed -i s/CONFIG_RTE_APP_TEST=y/CONFIG_RTE_APP_TEST=n/ \${RTE_SDK}/config/common_linuxapp \\
  && sed -i s/CONFIG_RTE_TEST_PMD=y/CONFIG_RTE_TEST_PMD=n/ \${RTE_SDK}/config/common_linuxapp

RUN cd \${RTE_SDK} \\
  && make install T=\${RTE_TARGET} DESTDIR=install -j

RUN git config --global user.email "root@container" \\
  && git config --global user.name "root"

RUN git clone http://dpdk.org/git/apps/pktgen-dpdk \\
  && cd pktgen-dpdk \\
  && git checkout \${PKTGEN_COMMIT} \\
  && cd .. \\
  && mv pktgen-dpdk \${PKTGEN_DIR}

RUN cd \${PKTGEN_DIR} \\
  && make -j

WORKDIR \${PKTGEN_DIR}/
EOF
```

Build the pktgen Docker image, tag it as`dpdk-pktgen`:
```bash
docker build -t dpdk-pktgen -f pktgen-dockerfile .
```

### 2. Create the pods to be used

Create the l3fwd Pod Spec:
```bash
$ cat <<EOF > l3fwd-pod.yaml
apiVersion: v1
kind: Pod
metadata:
  generateName: dpdk-l3fwd-
  annotations:
    k8s.v1.cni.cncf.io/networks: userspace-networkobj
spec:
  containers:
  - name: dpdk-l3fwd
    image: dpdk-l3fwd
    imagePullPolicy: IfNotPresent
    securityContext:
      privileged: true
      runAsUser: 0
    volumeMounts:
    - mountPath: /vhu/
      name: socket
    - mountPath: /dev/hugepages
      name: hugepage
    resources:
      requests:
        memory: 2Gi
      limits:
        hugepages-1Gi: 2Gi
    command: ["sleep", "infinity"]
  volumes:
  - name: socket
    hostPath:
      path: /var/lib/cni/vhostuser/
  - name: hugepage
    emptyDir:
      medium: HugePages
  securityContext:
    runAsUser: 0
  restartPolicy: Never
EOF
```

Create the pktgen Pod Spec:
```bash
$ cat <<EOF > pktgen-pod.yaml
apiVersion: v1
kind: Pod
metadata:
  generateName: dpdk-pktgen-
  annotations:
    k8s.v1.cni.cncf.io/networks: userspace-networkobj
spec:
  containers:
  - name: dpdk-pktgen
    image: dpdk-pktgen
    imagePullPolicy: IfNotPresent
    securityContext:
      privileged: true
      runAsUser: 0
    volumeMounts:
    - mountPath: /vhu/
      name: socket
    - mountPath: /dev/hugepages
      name: hugepage
    resources:
      requests:
        memory: 2Gi
      limits:
        hugepages-1Gi: 2Gi
    command: ["sleep", "infinity"]
  volumes:
  - name: socket
    hostPath:
      path: /var/lib/cni/vhostuser/
  - name: hugepage
    emptyDir:
      medium: HugePages
  securityContext:
    runAsUser: 0
  restartPolicy: Never
EOF
```

Deploy both pods:
```bash
$ kubectl create -f l3fwd-pod.yaml
$ kubectl create -f pktgen-pod.yaml
```

Verify your pods are running and note their unique name IDs:
```bash
$ kubectl get pods
NAME                       READY   STATUS    RESTARTS   AGE
dpdk-l3fwd-vj4hj      1/1        Running   0                 51s
dpdk-pktgen-xrsz9   1/1        Running   0                 2s
```

### 3. Run L3FWD and Pktgen

Using your pod ID, open a bash shell in the pktgen pod:
```bash
$ kubectl exec -it dpdk-pktgen-<ID> bash
```

Export the port ID prefex and start the pktgen application (adjust cores, memory, etc. to suit your system):
```bash
$ export ID=$(/vhu/get-prefix.sh)
$ ./app/x86_64-native-linuxapp-gcc/pktgen -l 10,11,12 --vdev=virtio_user0,path=/vhu/${ID}/${ID:0:12}-net1 --no-pci --socket-mem=1024,1024 --master-lcore 10 -- -m 11:12.0 -P
```

The pktgen application should launch. Among the statistics, make note of the pktgen source MAC address listed as ```Src MAC Address```. For example:
```bash
Src MAC Address   :   f2:89:22:4e:28:3b
```

In another terminal, open a bash shell in the l3fwd pod:
```bash
kubectl exec -it dpdk-l3fwd-<ID> bash
```

Export the port ID prefex and start the l3fwd application. Set the destination MAC address using the ```--eth-dest``` argument. This should be the  ```Src MAC Address``` previously noted from the pktgen pod (adjust cores, memory, etc. to suit your system):
```bash
$ export ID=$(/vhu/get-prefix.sh)
$ ./l3fwd -c 0x10 --vdev=virtio_user0,path=/vhu/${ID}/${ID:0:12}-net1 --no-pci --socket-mem=1024,1024 -- -p 0x1 -P --config "(0,0,4)" --eth-dest=0,<pktgen-source-mac-add> --parse-ptype
```

The l3fwd app should start up. Among the information printed to the screen will be the ```Address```. This is the MAC address of the l3fwd port, make note of it.

Back on the pktgen pod, set the destination MAC address to that of the l3fwd port:
```bash
Pktgen:/> set 0 dst mac <l3fwd-mac-address>
```

Start traffic generation:
```bash
Pktgen:/> start 0
```

You should see the packet counts for Tx and Rx increase, verifying that packets are being transmitted by pktgen and are being sent back via l3fwd running in the other pod.

To exit:
```bash
Pktgen:/> stop 0
Pktgen:/> quit
```

# Unit Testing

## Unit Tests Overview

The Userspace CNI networking plugin is written in [GO](https://golang.org/) and
it uses [GO Testing](https://golang.org/pkg/testing) package for unit testing.
In order to simplify unit test code, a [testify](https://github.com/stretchr/testify)
toolkit is also used for [assertions](https://godoc.org/github.com/stretchr/testify/assert).
Unit test files have suffix *_test.go* and they are located at the same directory
as the production code.

For example *userspace* package have one source file and one file with unit tests:

```bash
   userspace/userspace.go
   userspace/userspace_test.go
```

In the file `userspace_test.go` you can find a set of test functions with the same
name as functions from `userspace.go` with prefix `Test`. Each test function
implements a set of tests to test production code thoroughly with different
input data, etc.

Testing binary is built and tests executed by `go test` command at directory
with test files.

For example, unit tests for userspace package with verbose output can be executed
as follows:

```bash
cd userspace
go test -v
```

An example of verbose output of userspace unit tests:

```bash
=== RUN   TestPrintVersionString
=== RUN   TestPrintVersionString/verify_version_string
--- PASS: TestPrintVersionString (0.00s)
    --- PASS: TestPrintVersionString/verify_version_string (0.00s)
=== RUN   TestLoadNetConf
=== RUN   TestLoadNetConf/fail_to_parse_netConf_1
=== RUN   TestLoadNetConf/fail_to_parse_netConf_2
=== RUN   TestLoadNetConf/fail_to_parse_netConf_3
=== RUN   TestLoadNetConf/fail_to_parse_netConf_4
=== RUN   TestLoadNetConf/fail_to_parse_netConf_5
=== RUN   TestLoadNetConf/fail_to_parse_netConf_6
=== RUN   TestLoadNetConf/fail_to_set_default_logging_level
=== RUN   TestLoadNetConf/fail_to_set_log_file
=== RUN   TestLoadNetConf/load_correct_netConf
--- PASS: TestLoadNetConf (0.00s)
    --- PASS: TestLoadNetConf/fail_to_parse_netConf_1 (0.00s)
    --- PASS: TestLoadNetConf/fail_to_parse_netConf_2 (0.00s)
    --- PASS: TestLoadNetConf/fail_to_parse_netConf_3 (0.00s)
    --- PASS: TestLoadNetConf/fail_to_parse_netConf_4 (0.00s)
    --- PASS: TestLoadNetConf/fail_to_parse_netConf_5 (0.00s)
    --- PASS: TestLoadNetConf/fail_to_parse_netConf_6 (0.00s)
    --- PASS: TestLoadNetConf/fail_to_set_default_logging_level (0.00s)
    --- PASS: TestLoadNetConf/fail_to_set_log_file (0.00s)
    --- PASS: TestLoadNetConf/load_correct_netConf (0.00s)
...
```

It is possible to execute unit tests directly from the host where the code is
being developed. However it is strongly recommended to execute them from within
a container. It will assure that unit tests will be isolated from the host
and thus avoid any collision with the host environment (e.g. k8s or OVS).
Another advantage is a possibility to easily execute unit tests at various Linux
OS distributions. Support of unit test execution inside containers is implemented
by project Makefile and described in following paragraphs.

## Unit Tests Inside Container

Project `Makefile` defines a set of targets suitable for unit testing inside
containers. In order to build and use testing containers, both `docker` and
`gcc` (C preprocessor is used to assemble Dockerfiles) have to be installed
at the host.

Example of tools installation at Ubuntu:

```bash
   apt install docker.io gcc
```

After the installation is completed, then any of unit test specific targets
can be executed in order to:

* build test containers and update them in case that new commit is detected
* execute unit tests
* calculate code coverage
* cleanup unit test container images and generated docker files

Targets for container building, unit testing and code coverage calculation
are prepared in three different versions to be executed at:

* Linux OS distribution defined by `UT_OS` environment variable

  e.g. execute build of container image for CentOS8

  ```bash
     export UT_OS=centos8
     make test-build
  ```

* Linux OS distribution specified by target suffix

  e.g. calculate code coverage at Ubuntu 20.04

  ```bash
     make coverage-ubuntu20.04
  ```

* all supported OS distributions

  e.g. execute unit tests at all supported OS distros:

  ```bash
     make test-all
  ```

The list of supported `make` targets is also documented as part of `make help`
as follows:

```
Make Targets for unit testing inside containers:
 make test-clean      - Remove test container images and generated Dockerfiles.
 make test-build      - Build container image for unit tests with OS defined by UT_OS: UT_OS=ubuntu20.04
 make test            - Run unit tests inside container with OS defined by UT_OS: UT_OS=ubuntu20.04
 make coverage        - Calculate code coverage in container with OS defined by UT_OS: UT_OS=ubuntu20.04
 make test-build-<os> - Build container image for unit tests with <os>, e.g. make test-build-centos8
 make test-<os>       - Run unit tests inside container with <os>, e.g. make test-centos8
 make coverage-<os>   - Calculate code coverage inside container with <os>, e.g. make coverage-centos8
 make test-build-all  - Build container images for unit tests for all supported OS distributions
                        e.g. make -j 5 test-build-all
 make test-all        - Run unit tests inside container for all supported OS distributions
                        e.g. make -j 5 test-all
 make coverage-all    - Calculate code coverage inside container for all supported OS distributions.
                        e.g. make -j 5 coverage-all
```

Following Linux OS distributions are supported for unit testing inside containers:

* CentOS 7, 8
* Fedora 31, 32
* Ubuntu 16.04, 18.04, 20.04
