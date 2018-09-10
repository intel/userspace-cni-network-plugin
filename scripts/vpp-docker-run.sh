#!/bin/bash

# Run a docker container with network namespace set up by the
# CNI plugins.

# Example usage: ./docker-run.sh --rm busybox /sbin/ifconfig
scriptpath=$GOPATH/src/github.com/containernetworking/cni/scripts
echo $scriptpath

contid=$(docker run -d --net=none bmcfall/vpp-centos-userspace-cni:0.2.0 /bin/sleep 10000000)
pid=$(docker inspect -f '{{ .State.Pid }}' $contid)
netnspath=/proc/$pid/ns/net

$scriptpath/exec-plugins.sh add $contid $netnspath

function cleanup() {
	$scriptpath/exec-plugins.sh del $contid $netnspath
	docker rm -f $contid >/dev/null
}
trap cleanup EXIT

#docker run -v /var/run/vpp/:/var/run/vpp:rw --device=/dev/hugepages:/dev/hugepages --net=container:$contid $@
docker run -v /var/run/vpp/cni/shared:/var/run/vpp/cni/shared:rw -v /var/run/vpp/cni/$contid:/var/run/vpp/cni/data:rw --device=/dev/hugepages:/dev/hugepages --net=container:$contid $@

