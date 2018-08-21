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
	'''Create the Vhost User port, OVS works as Vhost User server'''
	tmp = sock.rsplit('/', 1)
	sock_dir, sock_file = tmp[0], tmp[1]

	try:
		# Add the DPDK Vhost User Port, OVS works as the server
		cmd = 'ovs-vsctl add-port br0 {} -- set Interface {} type=dpdkvhostuser'.format(sock_file, sock_file)
		execCommand(cmd)

		# Move the socket to desired location
		cmd = 'mv /usr/local/var/run/openvswitch/{} {}/{}'.format(sock_file, sock_dir, sock_file)
		execCommand(cmd)
	except:
		print "Some errors occured, please have a check..."

	# For the OVS, the socket file name is composed with the socket dir and the Interface name	
	return sock_file

def deleteVhostPort(port):
	'''Remove the DPDK Vhost User port from the OVS bridge'''
	cmd = 'ovs-vsctl --if-exists del-port br0 {}'.format(port)
	return re.sub("\n\s*\n*", "", execCommand(cmd))

def getVhostPortMac(port):
	'''Get MAC address of the specified Vhost User Port'''
	cmd = 'ovs-ofctl show br0'
	output = execCommand(cmd).split('\n')

	reg_str = ' [0-9]+({})*'.format(port)
	pattern = re.compile(reg_str)
	for line in output:
		if pattern.match(line):
			return string.splitfields(line)[1].split(':', 1)[1]

	return None

def configVhostPortRoute(port, containerIP, containerMAC):
	'''Setup Routing rules for the Vhost User port's client'''
	# TODO

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
