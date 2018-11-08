package cniovs

import (
	"os"
	"os/exec"
	"strings"
)

const defaultOvSSocketDir = "/usr/local/var/run/openvswitch/"

func execCommand(cmd string, args []string) ([]byte, error) {
	return exec.Command(cmd, args...).Output()
}

/*
Functions to control OVS by using the ovs-vsctl cmdline client.
*/

func createVhostPort(sock_dir string, sock_name string, client bool, bridge_name string) (string, error) {
	var err error

	type_str := "type=dpdkvhostuser"
	if client {
		type_str = "type=dpdkvhostuserclient"
	}

	// COMMAND: ovs-vsctl add-port <bridge_name> <sock_name> -- set Interface <sock_name> type=<dpdkvhostuser|dpdkvhostuserclient>
	cmd := "ovs-vsctl"
	args := []string{"add-port", bridge_name, sock_name, "--", "set", "Interface", sock_name, type_str}
	if _, err = execCommand(cmd, args); err != nil {
		return "", err
	}

	// Determine the location OvS uses for Sockets. Default location can be
	// overwritten with environmental variable: OVS_SOCKDIR
	ovs_socket_dir, ok := os.LookupEnv("OVS_SOCKDIR")
	if ok == false {
		ovs_socket_dir = defaultOvSSocketDir
	}

	// Move socket to defined dir for easier mounting
	return sock_name, os.Rename(
		ovs_socket_dir+sock_name,
		sock_dir+"/"+sock_name)
}

func deleteVhostPort(sock_name string, bridge_name string) error {
	// COMMAND: ovs-vsctl del-port <bridge_name> <sock_name>
	cmd := "ovs-vsctl"
	args := []string{"--if-exists", "del-port", bridge_name, sock_name}
	_, err := execCommand(cmd, args)
	return err
}

func createBridge(bridge_name string) error {
	var err error

	// COMMAND: ovs-vsctl add-br <bridge_name> -- set bridge <bridge_name> datapath_type=netdev
	cmd := "ovs-vsctl"
	args := []string{"add-br", bridge_name, "--", "set", "bridge", bridge_name, "datapath_type=netdev"}
	if _, err = execCommand(cmd, args); err != nil {
		return err
	}

	return err
}

func deleteBridge(bridge_name string) error {
	// COMMAND: ovs-vsctl del-br <bridge_name>
	cmd := "ovs-vsctl"
	args := []string{"del-br", bridge_name}

	_, err := execCommand(cmd, args)
	return err
}

func getVhostPortMac(sock_name string) (string, error) {
	// COMMAND: ovs-vsctl --bare --columns=mac find port name=<sock_name>
	cmd := "ovs-vsctl"
	args := []string{"--bare", "--columns=mac", "find", "port", "name=" + sock_name}
	if mac_b, err := execCommand(cmd, args); err != nil {
		return "", err
	} else {
		return strings.Replace(string(mac_b), "\n", "", -1), nil
	}
}

func findBridge(bridge_name string) bool {
	found := false

	// COMMAND: ovs-vsctl --bare --columns=name find bridge name=<bridge_name>
	cmd := "ovs-vsctl"
	args := []string{"--bare", "--columns=name", "find", "bridge", "name=" + bridge_name}
	if name, err := execCommand(cmd, args); err != nil {
		if name != nil && len(name) != 0 {
			found = true
		}
	}

	return found
}

func doesBridgeContainInterfaces(bridge_name string) bool {
	found := false

	// ovs-vsctl list-ports <bridge_name>
	cmd := "ovs-vsctl"
	args := []string{"list-ports", bridge_name}
	if name, err := execCommand(cmd, args); err != nil {
		if name != nil && len(name) != 0 {
			found = true
		}
	}

	return found
}
