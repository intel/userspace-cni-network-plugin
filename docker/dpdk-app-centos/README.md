#  Docker Image: dpdk-app-centos
This directory contains the files needed to build a DPDK based test image.
This image is based on CentOS (latest) base image built with DPDK 19.08.

The User Space CNI inconjunction with the OVS CNI Library (cniovs) or VPP
CNI Library (cnivpp) creates interfaces on the host, like a vhost-user or
a memif interface, adds the host side of the interface to a local network,
like a L2 bridge, then copies information needed in the container into
annotations. The container, like this one, boots up and reads the
annotatations and runs a DPDK application.


# Build Instructions for dpdk-app-centos Docker Image
Get the **user-space-net-plugin** repo:
```
   cd $GOPATH/src/
   go get github.com/intel/userspace-cni-network-plugin
```

Build the docker image:
```
   cd $GOPATH/src/github.com/intel/userspace-cni-network-plugin/docker/dpdk-app-centos/
   docker build --rm -t dpdk-app-centos .
```

## Reduce Image Size
Multi-stage builds are a new feature requiring **Docker 17.05** or higher on
the daemon and client. If multi-stage builds are supported on your system,
then uncomment the following lines (those with **##**) in the Dockerfile:
```
:

# -------- Import stage.
# Docker 17.05 or higher
##FROM centos

# Install UserSpace CNI
##COPY --from=0 /usr/bin/dpdk-app /usr/bin/dpdk-app

:
```

**NOTE:** Need to verify this works as designed. System currently has older
version of Docker and haven't been able to verify. May need to add an
additional package or two to the image after the import stage.


# Docker Image Details
This Docker Image is downloading DPDK (version 19.08 to get memif PMD)
and building it. Once built, changing into the DPDK `testpmd`
directory (${DPDK_DIR}/app/test-pmd) and building it.

`testpmd` is a sample DPDK application that comes with DPDK. Typically,
`testpmd` is started with a set of input parameters that initializes DPDK.
For example:
```
$ testpmd \
-m 1024 \
-l 2-3 \
-n 4 \
--vdev=virtio_user0,path=/var/lib/cni/usrspcni/558cbb292167-net1 \
--vdev=virtio_user1,path=/var/lib/cni/usrspcni/558cbb292167-net2 \
--no-pci \
-- \
--auto-start \
--tx-first \
--no-lsc-interrupt \
--stats-period 10
```

This Docker image is tweaking this a little. Before `testpmd` is built, the
testpmd.c file (contains main()) is updated using 'sed'. See
'testpmd_substitute.sh'.

**NOTE:** If a different version of DPDK is needed or used, this local file
may need to be synchronized with the updated version. 

An additional file, dpdk-args.c, is also added to the directory and Makefile.
The changes to testpmd.c are simply to call a function in dpdk-args.c which
will generate this list of input parameters, and then pass this private set
of parameters to DPDK functions instead of the inpupt `argc` and `argv`. When
the generated binary is copied to `/usr/bin/`, it is renamed to `dpdk-app`.

The code is leveraging another project, app-netutil
(https://github.com/openshift/app-netutil), which is a library design to be
called within a container to collect all the configuration data, like that
stored in annotations by Userspace CNI, and expose it to a DPDK application
in a clean API.

**NOTE:** For debugging, if `dpdk-app` is called with a set of input parameters,
it will skip the dpdk-args.c code and behave exactly as `testpmd`. Just add
the `sleep` to the pod spec:
```
:
    resources:
      requests:
        memory: 2Mi
      limits:
        hugepages-2Mi: 1024Mi
    command: ["sleep", "infinity"]    <-- ADD
  nodeSelector:
    vswitch: ovs
:
```

Then get a pod shell:
```
   kubectl exec -it userspace-ovs-pod-1 -- sh
```

Run `dpdk-app` with no parameters, and it will be as if it is called
as the container is started. It also prints out the generated parameter
list, which include the dynamic socketfile path:
```
sh-4.2# dpdk-app 
COLLECT Data:
  cpuRsp.CPUSet = 0-63
  Interface[0]:
    IfName="eth0"  Name="cbr0"  Type=unknown
    MAC="5e:6b:7e:19:5b:94"  IP="10.244.0.197"
  Interface[1]:
    IfName="net1"  Name="sriov-network-a"  Type=SR-IOV
    MAC=""
    PCIAddress=0000:01:0a.4
  Interface[2]:
    IfName="net2"  Name="sriov-network-b"  Type=SR-IOV
    MAC=""
    PCIAddress=0000:01:02.4
ENTER dpdk-app (testpmd):
  myArgc=15
  dpdk-app -n 4 -l 1-3 --master-lcore 1 --vdev=virtio_user0,path=/var/lib/cni/usrspcni/34c8ba49b767-net1 --vdev=virtio_user1,path=/var/lib/cni/usrspcni/34c8ba49b767-net2 --no-pci -- --auto-start --tx-first --no-lsc-interrupt --stats-period 60
EAL: Detected 64 lcore(s)
EAL: Detected 2 NUMA nodes
EAL: Multi-process socket /var/run/dpdk/rte/mp_socket
:
```

Then 'CTRL-C' to exit and re-run `dpdk-app` with input parameters
modified as needed:
```
dpdk-app \
-l 1-3 \
-master-lcore 1 \
-n 4 \
--vdev=virtio_user0,path=/var/lib/cni/usrspcni/34c8ba49b767-net1 \
--vdev=virtio_user1,path=/var/lib/cni/usrspcni/34c8ba49b767-net2 \
--no-pci \
-- \
--auto-start \
--tx-first \
--no-lsc-interrupt
```


# Deploy Image
An example of using this Docker image can be found in this same repo under:
```
   $GOPATH/src/github.com/intel/userspace-cni-network-plugin/examples/ovs-host/
```

This example assumes that kubernetes has already been deployed with `multus`
and some CNI for the default network (like flannel). It also assumes that OvS
is running on one or more of the worker nodes and the worker nodes running
OvS have the label `vswitch=ovs` applied. For example, for the node 
`k8s-work-1-f29-ovs1`, run:
```
   kubectl label nodes k8s-work-1-f29-ovs1 vswitch=ovs
```

The files in this subdirectory are used to create two networks:
* userspace-ovs-net-1
  * Creates bridge `br-4` on the local OvS instance
    * Adds vhost interface [ContainerId:12]-net[instance] (i.e. 8a0dd2a77c59-net1)
    	to the bridge on the local OvS instance.
    * Adds IP in the 10.56.217.0/24 subnet to the vhost interface in the container.
* userspace-ovs-net-2
  * Creates bridge `br-5` on the local OvS instance
    * Adds vhost interface [ContainerId:12]-net[instance] (i.e. 8a0dd2a77c59-net2)
    	to the bridge on the local OvS instance.
    * Adds IP in the 10.77.217.0/24 subnet to the vhost interface in container.

To apply the CRD for these networks:
```
   kubectl create -f examples/ovs-vhost/userspace-ovs-netAttach-1.yaml
   kubectl create -f examples/ovs-vhost/userspace-ovs-netAttach-2.yaml
```

Create two pods, each with 3 interfaces:
* 1 on the default network (i.e. kernel interface using flannel)
* 1 on userspace-ovs-net-1
* 1 on userspace-ovs-net-2

**NOTE:** Userspace CNI does not manage node to node networking. So
in this example, the two pods must be created on the same node (the
reason for the `vswitch=ovs` label), or OvS on the different nodes
must have connectivity setup between them.

To create the two pods:
```
   kubectl create -f examples/ovs-vhost/userspace-ovs-pod-1.yaml
   kubectl create -f examples/ovs-vhost/userspace-ovs-pod-2.yaml
```

To verify setup, on host, inspect OvS configuration:
```
sudo ovs-vsctl show
26373aec-1484-4980-8853-9db2183cfafc
    Bridge "br-5"
        Port "8a0dd2a77c59-net2"
            Interface "8a0dd2a77c59-net2"
                type: dpdkvhostuser
        Port "e5b2972a8210-net2"
            Interface "e5b2972a8210-net2"
                type: dpdkvhostuser
        Port "br-5"
            Interface "br-5"
                type: internal
    Bridge "br-4"
        Port "br-4"
            Interface "br-4"
                type: internal
        Port "8a0dd2a77c59-net1"
            Interface "8a0dd2a77c59-net1"
                type: dpdkvhostuser
        Port "e5b2972a8210-net1"
            Interface "e5b2972a8210-net1"
                type: dpdkvhostuser
    ovs_version: "2.9.2"
```

To show statistics (choose an interface form above output):
```
sudo ovs-vsctl list interface 8a0dd2a77c59-net2
[sudo] password for bmcfall: 
_uuid               : dde9399d-88fc-43c2-b3a1-8073fa3f9cef
admin_state         : up
bfd                 : {}
bfd_status          : {}
cfm_fault           : []
cfm_fault_status    : []
cfm_flap_count      : []
cfm_health          : []
cfm_mpid            : []
cfm_remote_mpids    : []
cfm_remote_opstate  : []
duplex              : []
error               : []
external_ids        : {}
ifindex             : 15730733
ingress_policing_burst: 0
ingress_policing_rate: 0
lacp_current        : []
link_resets         : 0
link_speed          : []
link_state          : up
lldp                : {}
mac                 : []
mac_in_use          : "00:00:00:00:00:00"
mtu                 : 1500
mtu_request         : []
name                : "8a0dd2a77c59-net2"
ofport              : 2
ofport_request      : []
options             : {}
other_config        : {}
statistics          : {"rx_1024_to_1522_packets"=0, "rx_128_to_255_packets"=0, "rx_1523_to_max_packets"=0, "rx_1_to_64_packets"=4226816, "rx_256_to_511_packets"=0, "rx_512_to_1023_packets"=0, "rx_65_to_127_packets"=0, rx_bytes=270516224, rx_dropped=0, rx_errors=0, rx_packets=4226816, tx_bytes=270514176, tx_dropped=0, tx_packets=4226784}
status              : {features="0x0000000110008000", mode=server, num_of_vrings="2", numa="0", socket="/usr/local/var/run/openvswitch/8a0dd2a77c59-net2", status=connected, "vring_0_size"="256", "vring_1_size"="256"}
type                : dpdkvhostuser
```