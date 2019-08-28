# VPP CNI Library Intro
VPP CNI Library is written in GO and used by UserSpace CNI to interface with the
VPP GO-API. The UserSpace CNI is a CNI implementation designed to implement
User Space networking (as apposed to kernel space networking), like DPDK based
applications. For example VPP and OVS-DPDK.

The UserSpace CNI, based on the input config data, uses this library to add
interfaces (memif and vhost-user interface) to a local VPP instance running on
the host and add that interface to a local network, like a L2 Bridge. The
UserSpace CNI then processes config data intended for the container and uses
this library to add that data to a Database the container can consume.

UserSpace CNI README.md describes how to test with VPP CNI. Below are a
few notes regarding packages in this sub-folder.

## usrsp-app
The **usrsp-app** is intended to run in a container. It leverages the VPP CNI code
to consume interfaces in the container.

## docker/vpp-centos-userspace-cni/
The docker image **vpp-centos-userspace-cni** runs a VPP instance and the
usrsp-app at startup. 

## cnivpp/localdb.go
**localdb** is use to store data in a DB. For the local VPP instance, the localdb
is used to store the swIndex generated when the interface is created. It is used
later to delete the interface. The usrspdb is used to pass configuration
data to the container so the container can consume the interface.

The initial implementation of the DB is just json data written to a file.
This can be expanded at a later date to write to something like an etcd DB.


The following filenames and directory structure is used to store the data:
* **Host**:
  * **/var/lib/cni/usrspcni/data/**:
    * **local-<ContainerId:12>-<ifname>.json**: Used to store local data
needed to delete and cleanup.

  * **/var/lib/cni/usrspcni/shared/**: Not a database directory, but this directory
is used for interface socket files, for example: **memif-<ContainerId:12>-<ifname>.sock**
This directory is mapped into that container as the same directory in the container.

  * **/var/lib/cni/usrspcni/<ContainerId>/**: This directory is mapped into that container
as **/var/lib/cni/usrspcni/data/**, so appears to the container as its local data
directory. This is where the container writes its
**local-<ContainerId:12>-<ifname>.json** file described above.
    * **remote-<ContainerId:12>-<ifname>.json**: This file contains the configuration
to apply the interface in the container. The data is the same json data passed into
the UserSpace CNI (define in **user-space-net-plugin/pkg/types/types.go**), but
the Container data has been copied into the Host data label. The usrsp-app processes the
data as local data. Once this data is read in the container, the usrsp-app deletes the
file.
    * **addData-<ContainerId:12>-<ifname>.json**: This file is used to pass
additional data into the the container, which is not defined by **pkg/types/types.go**.
This includes the ContainerId itself, and the results from the IPAM plugin that
were processed locally. Once this data is read in the container, the usrsp-app deletes
the file.

* **Container**:
  * **/var/lib/cni/usrspcni/data/**: Mapped from **/var/lib/cni/usrspcni/container/**
on the host.
  * **/var/lib/cni/usrspcni/shared/**: Mapped from **/var/lib/cni/usrspcni/shared/** on
the host.

