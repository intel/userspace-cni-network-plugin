#!/bin/bash

# Run a docker container with network namespace set up by the
# CNI plugins.

# Example usage: ./docker-run.sh --rm busybox /sbin/ifconfig
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

docker run -v /var/run/vpp/cni/shared:/var/run/vpp/cni/shared:rw -v /var/run/usrspcni/$contid:/var/run/usrspcni/data:rw --device=/dev/hugepages:/dev/hugepages --net=container:$contid $@

