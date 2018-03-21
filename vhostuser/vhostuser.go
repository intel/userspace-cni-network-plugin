// Copyright 2017 Intel Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containernetworking/cni/pkg/ipam"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
)

const defaultCNIDir = "/var/lib/cni/vhostuser"

type VhostConf struct {
	Vhostname string `json:"vhostname"`  // Vhost Port name
	VhostMac  string `json:"vhostmac"`   // Vhost port MAC address
	Ifname    string `json:"ifname"`     // Interface name
	IfMac     string `json:"ifmac"`      // Interface Mac address
	Vhosttool string `json:"vhost_tool"` // Scripts for configuration
}

type NetConf struct {
	types.NetConf
	VhostConf VhostConf `json:"vhost,omitempty"`
	If0name   string    `json:"if0name"`
	CNIDir    string    `json:"cniDir"`
}

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

// ExecCommand Execute shell commands and return the output.
func ExecCommand(cmd string, args []string) ([]byte, error) {
	return exec.Command(cmd, args...).Output()
}

func loadConf(bytes []byte) (*NetConf, error) {
	n := &NetConf{}
	if err := json.Unmarshal(bytes, n); err != nil {
		return nil, fmt.Errorf("failed to load netconf: %v", err)
	}

	if n.CNIDir == "" {
		n.CNIDir = defaultCNIDir
	}

	return n, nil
}

// saveVhostConf Save the rendered netconf for cmdDel
func saveVhostConf(conf *NetConf, containerID string) error {
	fileName := fmt.Sprintf("%s-%s.json", containerID[:12], conf.If0name)
	if vhostConfBytes, err := json.Marshal(conf.VhostConf); err == nil {
		sockDir := filepath.Join(conf.CNIDir, containerID)
		path := filepath.Join(sockDir, fileName)

		return ioutil.WriteFile(path, vhostConfBytes, 0644)
	} else {
		return fmt.Errorf("error serializing delegate netconf: %v", err)
	}
}

func (vc *VhostConf) loadVhostConf(conf *NetConf, containerID string) error {
	fileName := fmt.Sprintf("%s-%s.json", containerID[:12], conf.If0name)
	sockDir := filepath.Join(conf.CNIDir, containerID)
	path := filepath.Join(sockDir, fileName)

	if data, err := ioutil.ReadFile(path); err == nil {
		if err = json.Unmarshal(data, vc); err != nil {
			return fmt.Errorf("failed to parse VhostConf: %v", err)
		}
	} else {
		return fmt.Errorf("failed to read config: %v", err)
	}

	return nil
}

func createVhostPort(conf *NetConf, containerID string) error {
	s := []string{containerID[:12], conf.If0name}
	sockRef := strings.Join(s, "-")

	sockDir := filepath.Join(conf.CNIDir, containerID)
	if _, err := os.Stat(sockDir); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(sockDir, 0700); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	sockPath := filepath.Join(sockDir, sockRef)

	// vppctl create vhost socket /tmp/sock0 server
	args := []string{"create", sockPath}
	if output, err := ExecCommand(conf.VhostConf.Vhosttool, args); err == nil {
		vhostName := strings.Replace(string(output), "\n", "", -1)

		args = []string{"getmac", vhostName}
		if output, err := ExecCommand(conf.VhostConf.Vhosttool, args); err == nil {
			conf.VhostConf.VhostMac = strings.Replace(string(output), "\n", "", -1)
		}

		conf.VhostConf.Vhostname = vhostName
		conf.VhostConf.Ifname = conf.If0name
		conf.VhostConf.IfMac = GenerateRandomMacAddress()
		return saveVhostConf(conf, containerID)
	}

	return nil
}

func destroyVhostPort(conf *NetConf, containerID string) error {
	vc := &VhostConf{}
	if err := vc.loadVhostConf(conf, containerID); err != nil {
		return err
	}

	//vppctl delete vhost-user VirtualEthernet0/0/0
	args := []string{"delete", vc.Vhostname}
	if _, err := ExecCommand(conf.VhostConf.Vhosttool, args); err == nil {
		path := filepath.Join(conf.CNIDir, containerID)
		return os.RemoveAll(path)
	}

	return nil
}

const NET_CONFIG_TEMPLATE = `{
	"ipAddr": "%s/32",
	"macAddr": "%s",
	"gateway": "169.254.1.1",
	"gwMac": "%s"
}
`

func GenerateRandomMacAddress() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}

	// Set the local bit and make sure not MC address
	macAddr := fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		(buf[0]|0x2)&0xfe, buf[1], buf[2],
		buf[3], buf[4], buf[5])
	return macAddr
}

// SetupContainerNetwork Write the configuration to file
func SetupContainerNetwork(conf *NetConf, containerID, containerIP string) {
	args := []string{"config", conf.VhostConf.Vhostname, containerIP, conf.VhostConf.IfMac}
	ExecCommand(conf.VhostConf.Vhosttool, args)

	// Write the configuration to file
	config := fmt.Sprintf(NET_CONFIG_TEMPLATE, containerIP, conf.VhostConf.IfMac, conf.VhostConf.VhostMac)
	fileName := fmt.Sprintf("%s-%s-ip4.conf", containerID[:12], conf.If0name)
	sockDir := filepath.Join(conf.CNIDir, containerID)
	configFile := filepath.Join(sockDir, fileName)
	ioutil.WriteFile(configFile, []byte(config), 0644)
}

func cmdAdd(args *skel.CmdArgs) error {
	var result *types.Result
	var n *NetConf

	n, err := loadConf(args.StdinData)
	if err != nil {
		return result.Print()
	}

	createVhostPort(n, args.ContainerID)

	// run the IPAM plugin and get back the config to apply
	result, err = ipam.ExecAdd(n.IPAM.Type, args.StdinData)
	if err != nil {
		return fmt.Errorf("failed to set up IPAM: %v", err)
	}
	if result.IP4 == nil {
		return errors.New("IPAM plugin returned missing IPv4 config")
	}

	containerIP := result.IP4.IP.IP.String()
	SetupContainerNetwork(n, args.ContainerID, containerIP)

	return result.Print()
}

func cmdDel(args *skel.CmdArgs) error {
	if n, err := loadConf(args.StdinData); err == nil {
		if err = destroyVhostPort(n, args.ContainerID); err != nil {
			return err
		}

		return ipam.ExecDel(n.IPAM.Type, args.StdinData)
	} else {
		return err
	}
}

func main() {
	skel.PluginMain(cmdAdd, cmdDel)
}
