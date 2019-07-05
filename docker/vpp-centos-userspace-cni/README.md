#  DockerHub Image: bmcfall/vpp-centos-userspace-cni
This directory contains the files needed to build the docker image located in:
   https://hub.docker.com/r/bmcfall/vpp-centos-userspace-cni/

This image is based on CentOS (latest) base image built with VPP 19.04.1 and a
User Space CNI application (usrsp-app). Source code for the usrsp-app is in this
same repo:
   https://github.com/intel/userspace-cni-network-plugin

The User Space CNI inconjunction with the VPP CNI Library (cnivpp) creates
interfaces on the host, like memif interface, adds the host side of the
interface to a local network, then copies information needed in the container
into annotations. The container, like this one, boots up, starts a local instance
of VPP, then runs the usrsp-app to read the annotataions, looking for the needed
data. When found, usrsp-app consumes the data and writes to the local VPP instance
via the VPP GO API. This container then drops into bash for additional testing and
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
The following directory contains some sample yaml files that create
two networks and creates two pods with two addtional interfaces (one
for each network). Once created, the pods can ping each other over
these two networks.
'github.com/intel/userspace-cni-network-plugin/examples/vpp-memif-ping/'

Example:
Currently, Userspace CNI does not manage networks between nodes. For testing,
**userspace-vpp-pod-1.yaml** and **userspace-vpp-pod-2.yaml** have the 
**nodeSelector** of **vswitch: vpp** included so both pods end up on the same
node. Either remove this **nodeSelector** and ensure the VPP instaneces on
different nodes can pass traffic between nodes on the bridges defined in
**userspace-vpp-netAttach-1.yaml** and **userspace-vpp-netAttach-2.yaml**,
or add a label as follows, where **<node1>** is a valid node from
**kubectl get nodes**: 
```
kubectl label nodes <node1> vswitch=vpp
```

Create the two networks and then the two pods:
```
kubectl create -f examples/vpp-memif-ping/userspace-vpp-netAttach-1.yaml
kubectl create -f examples/vpp-memif-ping/userspace-vpp-netAttach-2.yaml
kubectl create -f examples/vpp-memif-ping/userspace-vpp-pod-1.yaml
kubectl create -f examples/vpp-memif-ping/userspace-vpp-pod-2.yaml
```

Log into the two pods and retrieve the IP addresses, and run a ping between them:
```
kubectl exec -it userspace-vpp-pod-1 -- sh
sh-4.2# vppctl show interface addr
local0 (dn):
memif1/0 (up):
  L3 10.56.217.161/24
memif2/0 (up):
  L3 10.77.217.161/24
sh-4.2#
```
```
kubectl exec -it userspace-vpp-pod-2 -- sh
sh-4.2# vppctl show interface addr
local0 (dn):
memif1/0 (up):
  L3 10.56.217.162/24
memif2/0 (up):
  L3 10.77.217.162/24
sh-4.2# vppctl ping 10.56.217.161
116 bytes from 10.56.217.161: icmp_seq=2 ttl=64 time=79.4484 ms
116 bytes from 10.56.217.161: icmp_seq=3 ttl=64 time=127.9922 ms
116 bytes from 10.56.217.161: icmp_seq=4 ttl=64 time=82.9510 ms
116 bytes from 10.56.217.161: icmp_seq=5 ttl=64 time=77.0007 ms

Statistics: 5 sent, 4 received, 20% packet loss
sh-4.2# vppctl ping 10.77.217.161
116 bytes from 10.77.217.161: icmp_seq=2 ttl=64 time=148.0164 ms
116 bytes from 10.77.217.161: icmp_seq=3 ttl=64 time=71.9887 ms
116 bytes from 10.77.217.161: icmp_seq=4 ttl=64 time=151.0084 ms
116 bytes from 10.77.217.161: icmp_seq=5 ttl=64 time=73.5451 ms

Statistics: 5 sent, 4 received, 20% packet loss
sh-4.2# 
```

# vpp-centos-userspace-cni Docker Image Nuances
Below are a couple points about the image that will probably need to change:

## VPP Version
This image is based on VPP 19.04.1, which is taken from the upstream
https://packagecloud.io/fdio/ repository.

**NOTE:** Care must be taken to ensure the same version of VPP is used to build
the cnivpp library, the usrsp-app, the Docker Image and running on the host.
Otherwise there may be an API version mismatch.

## Volumes and Devices
Inside the sample yaml files the pod is started with the following volume mounts:
```
vi github.com/intel/userspace-cni-network-plugin/examples/vpp-memif-ping/userspace-vpp-pod-1.yaml
:
    volumeMounts:
    - mountPath: /etc/podinfo
      name: podinfo
      readOnly: false
    - mountPath: /var/lib/cni/usrspcni/
      name: shared-dir
    - mountPath: /dev/hugepages
      name: hugepage
:
  volumes:
  - name: podinfo
    downwardAPI:
      items:
        - path: "labels"
          fieldRef:
            fieldPath: metadata.labels
        - path: "annotations"
          fieldRef:
            fieldPath: metadata.annotations
  - name: shared-dir
    hostPath:
      path: /var/lib/cni/usrspcni/023bcd123/
  - name: hugepage
    emptyDir:
      medium: HugePages
```
Where:
* **shared-dir** maps **/var/lib/cni/usrspcni/023bcd123** on host to
**/var/lib/cni/usrspcni** in the container.
  * This directory contains any files, like the virtio socket files, shared
between the host and the container.
  * The **023bcd123** sub-directory is intended to a be a unique folder
per pod on the host.
  * Work is in progress for an Addmission Controller to automatically insert
this volume mount into the Pod Spec and generate a random sub-directory.
* **podinfo** is a Downward API definition which maps **/etc/podinfo** into the
container.
  * All labels are inserted into a file named **labels** into this directory by
kubernetes.
  * All annotations are inserted into a file named **annotations** into this directory
by kubernetes.
  * Work is in progress for an Addmission Controller to automatically insert
this Downward API mount into the Pod Spec.
* **hugepage** maps **/dev/hugepages** on the host to **/dev/hugepages** in the
container.
  * Mapping hugepages into the Container, needed by DPDK application.
