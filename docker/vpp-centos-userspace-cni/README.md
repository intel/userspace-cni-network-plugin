#  DockerHub Image: bmcfall/vpp-centos-userspace-cni
This directory contains the files needed to build the docker image located in:
   https://hub.docker.com/r/bmcfall/vpp-centos-userspace-cni/

This image is based on CentOS (latest) base image built with VPP 19.04.1 and a
User Space CNI application (usrsp-app). Source code for the usrsp-app is in this
same repo:
   https://github.com/intel/userspace-cni-network-plugin

The User Space CNI inconjunction with the VPP CNI Library (cnivpp) creates
interfaces on the host, like memif or vhostuser, adds the host side of the
interface to a local network, then copies information needed in the container
into a DB. The container, like this one, boots up, starts a local instance of
VPP, then runs the usrsp-app to poll the DB looking for the needed data. Once
found, usrsp-app consumes the data and writes to the local VPP instance via the
VPP GO API. This container then drops into bash for additional testing and
debugging.


# Build Instructions for vpp-centos-userspace-cni Docker Image
Get the **user-space-net-plugin** repo:
```
   cd $GOPATH/src/
   go get github.com/intel/userspace-cni-network-plugin
```

Build the docker image:
```
   cd $GOPATH/src/github.com/intel/userspace-cni-network-plugin/docker/vpp-centos-userspace-cni/
   docker build --rm -t vpp-centos-userspace-cni .
```


## Development Image
The above process pulls the **usrsp-app** from the upstream source. If there are
local changes that need to be tested, then build **user-space-net-plugin** to
get the **usrsp-app** binary and copy the **usrsp-app** into the image directory:
```
   cd $GOPATH/src/github.com/intel/userspace-cni-network-plugin/
   make extras
   cp docker/usrsp-app/usrsp-app docker/vpp-centos-userspace-cni/.
```

Update the **Dockerfile** to use the local binary by uncommenting the COPY
command. **Make sure not to check this change in.**
```
   vi docker/vpp-centos-userspace-cni/Dockerfile
   :
   
   # For Development, overwrite repo generated usrsp-app with local development binary.
   # Needs to be commented out before each merge.
   COPY usrsp-app /usr/sbin/usrsp-app                   <-- Uncomment
```

Depending on the state of the **usrsp-app** and the number of changes, the
**usrsp-app** may not build from upstream, so building of **usrsp-app** in
the container image may also need to be commented out. **Make sure not to
check this change in.**
```
# Build the usrsp-app
WORKDIR /root/go/src/github.com/intel/
RUN git clone https://github.com/intel/userspace-cni-network-plugin
WORKDIR /root/go/src/github.com/intel/userspace-cni-network-plugin
#RUN make extras                                        <-- Comment out
#RUN cp docker/usrsp-app/usrsp-app /usr/sbin/usrsp-app  <-- Comment out
```

Build the docker image as described above.

# To run
The follwing directory contains some sample yaml files that create
two networks and creates two pods with two addtional interfaces (one
for each network). Once created, the pods can ping each other over
these two networks.
'github.com/intel/userspace-cni-network-plugin/examples/vpp-memif-ping/'

# vpp-centos-userspace-cni Docker Image Nuances
Below are a couple points about the image that will probably need to change:


## VPP Version
This image is based on VPP 19.04.1, which is taken from the upstream
https://packagecloud.io/fdio/ repository.

**NOTE:** Care must be taken to ensure the same version of VPP is used to build
the cnivpp library, the usrsp-app and the Docker Image. Otherwise there may be an
API version mismatch.


## Volumes and Devices
Inside the sample yaml files the pod is started with the following volume mounts:
```
vi github.com/intel/userspace-cni-network-plugin/examples/vpp-memif-ping/userspace-vpp-pod-1.yaml
:
    volumeMounts:
    - mountPath: /var/lib/cni/usrspcni/shared/
      name: socket
    - mountPath: /var/lib/cni/usrspcni/data/
      name: configdata
    - mountPath: /dev/hugepages
      name: hugepage
:
  volumes:
  - name: socket
    hostPath:
      path: /var/lib/cni/usrspcni/shared/
  - name: configdata
    hostPath:
      path: /var/lib/cni/usrspcni/container/
  - name: hugepage
    emptyDir:
      medium: HugePages
```
Where:
* **/var/lib/cni/usrspcni/shared** is mapped to **/var/lib/cni/usrspcni/shared**
** This directory contains the socketfiles shared between the host and
the container.
* **/var/lib/cni/usrspcni/container** is mapped to **/var/lib/cni/usrspcni/data**
** This directory is used by usrspdb to pass configuration data into the container.
Longer term, this may move to some etcd DB and this mapping can be removed.
* **device=/dev/hugepages** is mapped to **/dev/hugepages**
** Mapping hugepages into the Container, needed by VPP/DPDK.

