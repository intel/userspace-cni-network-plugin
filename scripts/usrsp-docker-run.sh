#!/bin/bash

# Run a docker container with network namespace set up by the
# CNI plugins. This script is a copy of the following CNI script
# with Userspace CNI specific modifications:
#   go/src/github.com/containernetworking/cni/scripts/docker-run.sh
#
# VPP Example usage:
#   cd $GOPATH/src/github.com/intel/userspace-cni-network-plugin
#   sudo CNI_PATH=$CNI_PATH GOPATH=$GOPATH ./scripts/usrsp-docker-run.sh -it --privileged vpp-centos-userspace-cni
#
# OvS Example usage:
#   cd $GOPATH/src/github.com/intel/userspace-cni-network-plugin
#   sudo CNI_PATH=$CNI_PATH GOPATH=$GOPATH ./scripts/usrsp-docker-run.sh -it --privileged ovs-centos-userspace-cni
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

docker run \
 -v /var/lib/cni/usrspcni/shared:/var/lib/cni/usrspcni/shared:rw \
 -v /var/lib/cni/usrspcni/$contid:/var/lib/cni/usrspcni/data:rw \
 --device=/dev/hugepages:/dev/hugepages \
 --net=container:$contid $@

