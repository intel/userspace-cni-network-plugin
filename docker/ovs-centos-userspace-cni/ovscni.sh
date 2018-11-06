#!/bin/bash

# Allocate Hugepages
sysctl -w vm.nr_hugepages=1024; sysctl -w kernel.shmmax=2147483648
mount -t hugetlbfs none /dev/hugepages
export PATH=$PATH:/usr/local/share/openvswitch/scripts:/usr/local/bin

# Start the first process
/usr/local/share/openvswitch/scripts/ovs-ctl --no-ovs-vswitchd start
/usr/local/bin/ovs-vsctl --no-wait set Open_vSwitch . other_config:dpdk-init=true
/usr/local/share/openvswitch/scripts/ovs-ctl --no-ovsdb-server start
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

