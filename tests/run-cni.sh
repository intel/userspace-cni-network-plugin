# Copyright (c) 2023 Intel Corp
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
# implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#!/bin/sh
# https://github.com/containernetworking/cni/blob/main/SPEC.md#section-2-execution-protocol
# https://github.com/containernetworking/cni/blob/spec-v1.0.0/plugins/debug/README.md

unset CNI_COMMAND CNI_IFNAME CNI_NETNS CNI_CONTAINERID
export CNI_PATH=/opt/cni/bin
mkdir -p {$CNI_PATH}
#create a random namespace
apt install -y uuidgen
namespace=uuidgen -r
containerid=uuidgen -r
ip netns add ${namespace}||true

set CNI_COMMAND=ADD
set CNI_CONTAINERID=${containerid}
set CNI_NETNS=/var/run/${namespace}
set CNI_IFNAME=eth0

cat /etc/cni/net.d/90-userspace.conf | ${CNI_PATH}/userspace

# cleanup
sleep 5
set CNI_COMMAND=DEL
cat /etc/cni/net.d/90-userspace.conf | ${CNI_PATH}/userspace
sleep 5
ip netns delete ${namespace}||true
unset CNI_COMMAND CNI_IFNAME CNI_NETNS CNI_CONTAINERID



