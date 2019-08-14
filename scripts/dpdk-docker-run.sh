#!/bin/bash

# Run a docker container with network namespace set up by the
# CNI plugins. This script is a copy of the following CNI script
# with Userspace CNI specific modifications:
#   go/src/github.com/containernetworking/cni/scripts/docker-run.sh
#
# DPDK Application Example usage:
#   cd $GOPATH/src/github.com/intel/userspace-cni-network-plugin
#   sudo env "PATH=$PATH" CNI_PATH=$CNI_PATH GOPATH=$GOPATH ./scripts/dpdk-docker-run.sh -it --privileged dpdk-app-testpmd
#
# Add DEBUG=1 for additional output.
#

scriptpath=$GOPATH/src/github.com/containernetworking/cni/scripts
echo $scriptpath

contid=$(docker run -d --net=none $@ /bin/sleep 10000000)
pid=$(docker inspect -f '{{ .State.Pid }}' $contid)
netnspath=/proc/$pid/ns/net

$scriptpath/exec-plugins.sh add $contid $netnspath

function cleanup() {
	$scriptpath/exec-plugins.sh del $contid $netnspath
	docker rm -f $contid >/dev/null
}
trap cleanup EXIT

#
# Temporary - Hardcode the vhost socket file: <ContId:12>-eth0
# ToDo: Need to read the DB files to pull out the <ContId> and <IfName>
#
trucContid=${contid:0:12}
docker run -i -t -v /var/lib/cni/usrspcni/shared:/var/lib/cni/usrspcni/shared:rw \
    -v /dev/hugepages:/dev/hugepages \
    dpdk-app-testpmd testpmd -l 0-1 -n 4 -m 1024 --no-pci \
    --vdev=virtio_user0,path=/var/lib/cni/usrspcni/shared/$trucContid-eth0 \
    --file-prefix=container \
    -- -i --txqflags=0xf00 --disable-hw-vlan --port-topology=chained

