package cniovs

import (
	"os"
	"os/exec"
	"strings"
)

func execCommand(cmd string, args []string) ([]byte, error) {
	return exec.Command(cmd, args...).Output()
}

/*
Functions to control OVS by using the ovs-vsctl cmdline client.
*/

func createVhostPort(sock_dir string, sock_name string) (string, error) {
	// Create socket
	cmd := "ovs-vsctl"
	args := []string{"add-port", "br0", sock_name, "--", "set", "Interface", sock_name, "type=dpdkvhostuser"}
	if _, err := execCommand(cmd, args); err != nil {
		return "", err
	}

	// Move socket to defined dir for easier mounting
	return sock_name, os.Rename(
		"/usr/local/var/run/openvswitch/"+sock_name,
		sock_dir+"/"+sock_name)
}

func deleteVhostPort(sock_name string) error {
	cmd := "ovs-vsctl"
	args := []string{"--if-exists", "del-port", "br0", sock_name}
	_, err := execCommand(cmd, args)
	return err
}

func getVhostPortMac(sock_name string) (string, error) {
	cmd := "ovs-vsctl"
	args := []string{"--bare", "--columns=mac", "find", "port", "name=" + sock_name}
	if mac_b, err := execCommand(cmd, args); err != nil {
		return "", err
	} else {
		return strings.Replace(string(mac_b), "\n", "", -1), nil
	}
}
