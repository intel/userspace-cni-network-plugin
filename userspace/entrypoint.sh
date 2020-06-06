#!/bin/sh

# Always exit on errors.
set -e

# Check if /opt/cni/bin directory exists
if [ ! -d "/host/opt/cni/bin" ] 
then
    echo "Directory /opt/cni/bin/ does not exists." 
    exit 1;
fi

# Copy cni-plugin on host machine
cp -f /userspace /host/opt/cni/bin/

# Sleep for 50 years. 
# sleep infinity is not available in alpine; 
sleep 2147483647
