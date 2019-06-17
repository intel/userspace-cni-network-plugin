#!/bin/bash

# Start the first process
/usr/bin/vpp -c /etc/vpp/startup.conf &> vppBoot.log &
status=$?
if [ $status -ne 0 ]; then
  echo "Failed to start VPP: $status"
  exit $status
fi

sleep 2

# Start the second process
/usr/sbin/usrsp-app &
status=$?
if [ $status -ne 0 ]; then
  echo "Failed to start usrsp-app for VPP: $status"
  exit $status
fi

while :
do
  sleep 1000
done

