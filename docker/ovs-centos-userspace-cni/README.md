#  DockerHub Image: bmcfall/ovs-centos-userspace-cni
This directory contains the files needed to build the docker image located in:
   https://hub.docker.com/r/bmcfall/ovs-centos-userspace-cni/

This image is based on CentOS (latest) base image built with OvS-DPDK 2.9.2,
DPDK 17.11.4 and a User Space CNI application (usrsp-app). Source code for
the usrsp-app is in this same repo:
   https://github.com/intel/userspace-cni-network-plugin

The User Space CNI inconjunction with the OVS CNI Library (cniovs) creates
interfaces on the host, like vhostuser, adds the host side of the
interface to a local network, then copies information needed in the container
into a DB. The container, like this one, boots up, starts a local instance of
OvS-DPDK, then runs the usrsp-app to poll the DB looking for the needed data. Once
found, usrsp-app consumes the data and writes to the local OVS instance via the
OVS commandline (ovs-vsctl). This container then drops into bash for additional
testing and debugging.

# Status
This is a **TEST** container. Currently EAL Init in DPDK panics because there
is an issue with the hugepages. Still debugging. Move to use the
dpdk-centos-userspace-cni image, which runs DPDK testpmd or l3fwd.

**Don't use this container at this time.**


# Build Instructions for ovs-centos-userspace-cni Docker Image
Get the **user-space-net-plugin** repo:
```
   cd $GOPATH/src/
   go get github.com/intel/userspace-cni-network-plugin
```

Build the docker image:
```
   cd $GOPATH/src/github.com/intel/userspace-cni-network-plugin/docker/ovs-centos-userspace-cni/
   docker build --rm -t ovs-centos-userspace-cni .
```

## Development Image
The above process pulls the **usrsp-app** from the upstream source. If there are
local changes that need to be tested, then build **user-space-net-plugin** to
get the **usrsp-app** binary and copy the **usrsp-app** into the image directory:
```
   cd $GOPATH/src/github.com/intel/userspace-cni-network-plugin/
   make extras
   cp docker/usrsp-app/usrsp-app docker/ovs-centos-userspace-cni/.
```

Update the **Dockerfile** to use the local binary by uncommenting the COPY
command. Make sure not to check this change in.
```
   vi docker/ovs-centos-userspace-cni/Dockerfile
   :
   
   # For Development, overwrite repo generated usrsp-app with local development binary.
   # Needs to be commented out before each merge.
   #COPY usrsp-app /usr/sbin/usrsp-app
```

Build the docker image as described above.

# To run
Up to this point, all my testing with this container has been with the
script from the User Space CNI:
   github.com/intel/userspace-cni-network-plugin/scripts/usrsp-docker-run.sh
This is a local copy of the CNI test script
(https://github.com/containernetworking/cni/blob/master/scripts/docker-run.sh),
with a few local changes to easy deployment
(see [Volumes and Devices](#Volumes and Devices) below). To run:
* Create a JSON config file as described in
github.com/intel/userspace-cni-network-plugin/README.md.
* Make sure same version of OVS is running on the host.
* user-space-net-plugin is built and copied to $CNI_PATH
(see user-space-net-plugin).
* Then run:
```
sudo CNI_PATH=$CNI_PATH GOPATH=$GOPATH ./scripts/usrsp-docker-run.sh -it --privileged ovs-centos-userspace-cni
```

# ovs-centos-userspace-cni Docker Image Nuances
Below are a couple points about the image that will probably need to change:


## Volumes and Devices
Inside usrsp-docker-run.sh, the script starts this image as follows:
```
docker run \
 -v /var/lib/cni/usrspcni/shared:/var/lib/cni/usrspcni/shared:rw \
 -v /var/lib/cni/usrspcni/$contid:/var/lib/cni/usrspcni/data:rw  \
 --device=/dev/hugepages:/dev/hugepages \
 --net=container:$contid $@
```
Where:
* **/var/lib/cni/usrspcni/shared** mapped to **/var/lib/cni/usrspcni/shared**
** This directory contains the socketfiles shared between the host and
the container.
* **/var/lib/cni/usrspcni/$contid** mapped to **/var/lib/cni/usrspcni/data**
** This directory is used by usrspdb to pass configuration data into the container.
Longer term, this may move to some etcd DB and this mapping can be removed.
* **device=/dev/hugepages:/dev/hugepages**
** Mapping hugepages into the Container, needed by OVS/DPDK.

