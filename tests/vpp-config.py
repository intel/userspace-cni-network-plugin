#!/usr/bin/python

# Copyright (c) 2017 Intel Corp
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

import os
import sys
import subprocess
import re
import string

def execCommand(command):
	''' Execute the shell command and return the output'''
	data = subprocess.Popen(command, stdout=subprocess.PIPE, shell=True).communicate()[0]
	if 'rpc error' not in data:
		return data
	return None

def createVhostPort(sock):
	'''Create the Vhost User port, VPP works as Vhost User server'''
	cmd = 'vppctl create vhost socket {} server'.format(sock)
	vport = re.sub("\n\s*\n*", "", execCommand(cmd))

	cmd = 'vppctl set interface state {} up'.format(vport)
	execCommand(cmd)

	return vport

def deleteVhostPort(port):
	'''Remove the DPDK Vhost User port from the VPP bridge'''
	cmd = 'vppctl delete vhost-user {}'.format(port)
	return re.sub("\n\s*\n*", "", execCommand(cmd))

def getVhostPortMac(port):
	'''Get MAC address of the specified Vhost User Port'''
	cmd = 'vppctl show hardware {}'.format(port)
	return string.splitfields(execCommand(cmd))[10]

def configVhostPortRoute(port, containerIP, containerMAC):
	'''Setup Routing rules for the Vhost User port's client'''
	cmd = 'vppctl set int unnum {} use loop0'.format(port)
	execCommand(cmd)

	cmd = 'vppctl ip route add {}/32 via {}'.format(containerIP, port)
	execCommand(cmd)

	cmd = 'vppctl set ip arp {} {} {}'.format(port, containerIP, containerMAC)
	execCommand(cmd)

	tap = ''
	cmd = 'vppctl show tap'
	output = execCommand(cmd).split('\n')
	for line in output:
		entry = string.splitfields(line)
		if len(entry) == 3 and entry[0] == port:
			tap = entry[2]
			cmd = 'ip link set {} up'.format(tap)
			execCommand(cmd)
			cmd = 'ip route add {}/32 dev {}'.format(containerIP, tap)
			execCommand(cmd)
			break
	return tap

def main():
	if (len(sys.argv) == 1):
		print "Usage: ", sys.argv[0], "command [options]"
		exit(1)

	if sys.argv[1] == 'create':
		print createVhostPort(sys.argv[2])
	elif sys.argv[1] == 'delete':
		print deleteVhostPort(sys.argv[2])
	elif sys.argv[1] == 'getmac':
		print getVhostPortMac(sys.argv[2])
	elif sys.argv[1] == 'config':
		print configVhostPortRoute(sys.argv[2], sys.argv[3], sys.argv[4])
	else:
		print "Not supported yet!"

if __name__ == "__main__":
	try:
		main()
	except KeyboardInterrupt:
		pass
