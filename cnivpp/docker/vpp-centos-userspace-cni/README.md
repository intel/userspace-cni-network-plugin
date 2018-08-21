#  DockerHub Image: bmcfall/vpp-centos-userspace-cni
This directory contains the files needed to build the docker image located in:
   https://hub.docker.com/r/bmcfall/vpp-centos-userspace-cni/

This image is based on CentOS (latest) base image built with VPP 18.04 and a
VPP User Space CNI application (vpp-app). Source code for the vpp-app is in this
same repo:
   https://github.com/intel/vhost-user-net-plugin

The User Space CNI inconjunction with the VPP CNI Library (cnivpp) creates
interfaces on the host, like memif or vhostuser, adds the host side of the
interface to a local network, then copies information needed in the container
into a DB. The container, like this one, boots up, starts a local instance of
VPP, then runs the vpp-app to poll the DB looking for the needed data. Once
found, vpp-app consumes the data and writes to the local VPP instance via the
VPP GO API. This container then drops into bash for additional testing and
debugging.


# Build Instructions for vpp-centos-userspace-cni Docker Image
Get the **user-space-net-plugin** repo:
```
   cd $GOPATH/src/
   go get github.com/intel/vhost-user-net-plugin
```

Build the docker image:
```
   cd $GOPATH/src/github.com/intel/vhost-user-net-plugin/cnivpp/docker/vpp-centos-userspace-cni/
   docker build --rm -t vpp-centos-userspace-cni .
```

## Development Image
The above process pulls the **vpp-app** from the upstream source. If there are
local changes that need to be tested, then build **user-space-net-plugin** to
get the **vpp-app** binary and copy the **vpp-app** into the image directory:
```
   cd $GOPATH/src/github.com/intel/vhost-user-net-plugin/
   ./build
   cp cnivpp/vpp-app/vpp-app cnivpp/docker/vpp-centos-userspace-cni/.
```

Update the **Dockerfile** to use the local binary by uncommenting the COPY
command. Make sure not to check this change in.
```
   vi cnivpp/docker/vpp-centos-userspace-cni/Dockerfile
   :
   
   # For Development, overwrite repo generated vpp-app with local development binary.
   # Needs to be commented out before each merge.
   #COPY vpp-app /usr/sbin/vpp-app
```

Build the docker image as described above.

# To run
Up to this point, all my testing with this container has been with the
script from the User Space CNI:
   github.com/intel/vhost-user-net-plugin/scripts/vpp-docker-run.sh
This is a local copy of the CNI test script
(https://github.com/containernetworking/cni/blob/master/scripts/docker-run.sh),
with a few local changes to easy deployment
(see [Volumes and Devices](#Volumes and Devices) below). To run:
* Create a JSON config file as described in
github.com/intel/vhost-user-net-plugin/README.md.
* Make sure same version of VPP is running on the host.
* user-space-net-plugin is built and copied to $CNI_PATH
(see user-space-net-plugin).
* Then run:
```
sudo CNI_PATH=$CNI_PATH GOPATH=$GOPATH ./scripts/vpp-docker-run.sh -it --privileged vpp-centos-userspace-cni
```

# vpp-centos-userspace-cni Docker Image Nuances
Below are a couple points about the image that will probably need to change:


## VPP Version
This image is based on VPP 18.04, using the *fdio-release.repo* file. The
*Dockerfile* just copies *.repo into the */etc/yum.repos.d/*. To change the
VPP version in the Docker image, update the *fdio-release.repo* file.

**NOTE:** Care must be taken to ensure the same version of VPP is used to build
the cnivpp library, the vpp-app and the Docker Image. Otherwise there may be an
API version mismatch.

As an example, to update the version of VPP used in the Docker image to the latest
version, update the *fdio-release.repo* file as follows:
```
[fdio-release]
name=fd.io release branch latest merge
baseurl=https://nexus.fd.io/content/repositories/fd.io.centos7/
enabled=1
gpgcheck=0
```

For more examples, see: https://wiki.fd.io/view/VPP/Installing_VPP_binaries_from_packages


## Volumes and Devices
Inside vpp-docker-run.sh, the script starts this image as follows:
```
docker run \
 -v /var/run/vpp/cni/shared:/var/run/vpp/cni/shared:rw \
 -v /var/run/vpp/cni/$contid:/var/run/vpp/cni/data:rw  \
 --device=/dev/hugepages:/dev/hugepages \
 --net=container:$contid $@
```
Where:
* **/var/run/vpp/cni/shared** mapped to **/var/run/vpp/cni/shared**
** This directory contains the socketfiles shared between the host and
the container.
* **/var/run/vpp/cni/$contid** mapped to **/var/run/vpp/cni/data**
** This directory is used by vppdb to pass configuration data into the container.
Longer term, this may move to some etcd DB and this mapping can be removed.
* **device=/dev/hugepages:/dev/hugepages**
** Mapping hugepages into the Container, needed by VPP/DPDK.

