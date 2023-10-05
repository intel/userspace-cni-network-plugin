#!/bin/bash
socket=$(ls /var/lib/cni/usrspcni/)

containerid=$(grep containerID /etc/podinfo/annotations |cut -d = -f 2 | sed 's/"//g')
echo "${containerid:0:12}"



echo "sh int addr"
vppctl "sh int addr"
echo "create memif socket id 1 filename /var/lib/cni/usrspcni/$socket"
vppctl "create memif socket id 1 filename /var/lib/cni/usrspcni/$socket"
echo "create interface memif id 0 socket-id 1 client no-zero-copy"
vppctl "create interface memif id 0 socket-id 1 client no-zero-copy"
echo "set int state memif1/0 up"
vppctl "set int state memif1/0 up"
if hostname | grep -q app1;  then
    echo app1
    echo "set int ip address memif1/0 192.168.1.3/24"
    vppctl "set int ip address memif1/0 192.168.1.3/24"
else
    echo app2
    echo "set int ip address memif1/0 192.168.1.4/24"
    vppctl "set int ip address memif1/0 192.168.1.4/24"

fi
echo "sh int addr"
vppctl "sh int addr"
echo "sh memif"
vppctl "sh memif"
exit 0
