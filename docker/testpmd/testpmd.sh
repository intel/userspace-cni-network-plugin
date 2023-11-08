#!/bin/bash

cd /dpdk || exit 1

#set container id  as env
LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/local/lib/x86_64-linux-gnu/
while ! [ -s /etc/podinfo/annotations ]; do
    echo "annotations not found yet"
    sleep 1 # throttle the check
done
cat /etc/podinfo/annotations
sleep 10
containerid=$(grep containerId /etc/podinfo/annotations |cut -d '"' -f 5 |sed 's/\\//')
echo "${containerid:0:12}"

if grep -q app1 /etc/podinfo/labels; then
        fordwardmode="txonly"
	cpu="2,3,4,5"
else 
	fordwardmode="rxonly"
	cpu="6,7,8,9"
fi

#--stats-period 1 is needed to avoid testpmd exiting
commands="./build/app/dpdk-testpmd -l $cpu --vdev net_virtio_user0,path=/var/lib/cni/usrspcni/${containerid:0:12}-net1,server=1,queue_size=2048 --in-memory --single-file-segments -- --tx-ip 192.168.1.1,192.168.1.2 --tx-udp=4000,4000 --forward-mode=$fordwardmode --stats-period 1" #-i
echo "$commands"
$commands
