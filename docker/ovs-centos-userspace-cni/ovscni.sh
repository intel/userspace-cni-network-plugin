#!/bin/bash

# Allocate Hugepages
sysctl -w vm.nr_hugepages=1024; sysctl -w kernel.shmmax=2147483648

# Start the first process
#systemctl start openvswitch.service &> ovsBoot.log &
/usr/share/openvswitch/scripts/ovs-ctl start
status=$?
if [ $status -ne 0 ]; then
  echo "Failed to start OvS: $status"
  exit $status
fi

sleep 2

# Start the second process
/usr/sbin/usrsp-app &
status=$?
if [ $status -ne 0 ]; then
  echo "Failed to start usrsp-app for OvS: $status"
  exit $status
fi

