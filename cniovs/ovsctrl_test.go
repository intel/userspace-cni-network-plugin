// Copyright 2020 Intel Corp.
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

package cniovs

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateVhostPort(t *testing.T) {
	expCmd := "ovs-vsctl"
	testCases := []struct {
		name       string
		client     bool
		ovsDir     bool
		noDirSlash bool
		renameFail bool
		fakeErr    error
	}{
		{
			name:    "fail to run ovs-ctl",
			client:  true,
			fakeErr: errors.New("error"),
		},
		{
			name:   "create vhost server interface",
			client: false,
		},
		{
			name:       "create vhost server interface and fail to move socket",
			client:     false,
			renameFail: true,
		},
		{
			name:       "create vhost server interface and fail to move socket from OVS_SOCKETDIR",
			client:     false,
			renameFail: true,
			ovsDir:     true,
		},
		{
			name:   "create vhost client interface",
			client: true,
		},
		{
			name:       "create vhost client interface with socket dir without trailing slash",
			client:     true,
			noDirSlash: true,
		},
		{
			name:   "create vhost client interface with OVS_SOCKDIR set",
			client: true,
			ovsDir: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)
			execCommand := &FakeExecCommand{Err: tc.fakeErr}

			socketDir, dirErr := ioutil.TempDir("/tmp", "test-cniovs-")
			require.NoError(dirErr, "Can't create temporary directory")
			defer os.RemoveAll(socketDir)

			randSuffix := strings.Split(socketDir, "-")[2]
			socket := "socket-" + randSuffix

			expArgs := []string{"add-port", "br0", socket, "--", "set", "Interface", socket}
			expClientArgs := append(expArgs, "type=dpdkvhostuserclient", "options:vhost-server-path="+path.Join(socketDir, socket))
			expServerArgs := append(expArgs, "type=dpdkvhostuser")

			// error scenario to trigger os.Rename failure
			if tc.renameFail {
				socketDir = "/proc/"
			}

			// create fake socket file at OVS socket dir
			ovsDir := defaultOvSSocketDir
			if tc.ovsDir {
				ovsDir = fmt.Sprintf("/tmp/test-ovs-%v/", randSuffix)
				os.Setenv("OVS_SOCKDIR", ovsDir)
				defer os.Unsetenv("OVS_SOCKDIR")
			}
			if _, err := os.Stat(ovsDir); err != nil {
				require.NoError(os.MkdirAll(ovsDir, os.ModePerm), "Can't create ovsDir")
				defer os.RemoveAll(ovsDir)
			}
			socketFull := path.Join(ovsDir, socket)
			_, socketErr := os.Create(socketFull)
			require.NoError(socketErr, "Can't create socket")
			defer os.Remove(socketFull)

			require.NoFileExists(path.Join(socketDir, socket), "Socket file shall not be in socketDir")

			SetExecCommand(execCommand)
			result, err := createVhostPort(socketDir, socket, tc.client, "br0")
			SetDefaultExecCommand()

			if tc.fakeErr == nil {
				assert.Equal(socket, result, "Unexpected result value")
			} else {
				assert.Equal("", result, "Unexpected result value")
			}
			assert.Equal(tc.fakeErr, err, "Unexpected error value")
			assert.Equal(expCmd, execCommand.Cmd, "Unexpected command executed")

			if tc.client {
				assert.Equal(expClientArgs, execCommand.Args, "Unexpected command arguments")
			} else {
				assert.Equal(expServerArgs, execCommand.Args, "Unexpected command arguments")
				// test if vhostuser SERVER port socket was moved to socketDir
				if tc.renameFail {
					assert.NoFileExists(path.Join(socketDir, socket), "Socket file was found in socketDir")
					assert.FileExists(path.Join(ovsDir, socket), "Socket file was not found in ovsDir")
				} else {
					assert.FileExists(path.Join(socketDir, socket), "Socket file was not moved from ovsDir to socketDir")
					assert.NoFileExists(path.Join(ovsDir, socket), "Socket file was not moved from ovsDir to socketDir")
				}
			}

		})
	}
}

func TestDeleteVhostPort(t *testing.T) {
	expCmd := "ovs-vsctl"
	bridge := "br0"
	socket := "tmp-socket"
	expArgs := []string{"--if-exists", "del-port", "br0", "tmp-socket"}

	testCases := []struct {
		name    string
		fakeErr error
	}{
		{
			name:    "delete vhost port",
			fakeErr: nil,
		},
		{
			name:    "fail to delete vhost port",
			fakeErr: errors.New("Can't remove socket"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execCommand := &FakeExecCommand{Err: tc.fakeErr}
			SetExecCommand(execCommand)
			result := deleteVhostPort(socket, bridge)
			SetDefaultExecCommand()
			assert.Equal(t, tc.fakeErr, result, "Unexpected result")
			assert.Equal(t, expCmd, execCommand.Cmd, "Unexpected command executed")
			assert.Equal(t, expArgs, execCommand.Args, "Unexpected command arguments")

		})
	}
}

func TestCreateBridge(t *testing.T) {
	expCmd := "ovs-vsctl"
	bridge := "br0"
	expArgs := []string{"add-br", "br0", "--", "set", "bridge", "br0", "datapath_type=netdev"}

	testCases := []struct {
		name    string
		fakeErr error
	}{
		{
			name:    "create bridge",
			fakeErr: nil,
		},
		{
			name:    "fail to create bridge",
			fakeErr: errors.New("Can't create bridge"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execCommand := &FakeExecCommand{Err: tc.fakeErr}
			SetExecCommand(execCommand)
			result := createBridge(bridge)
			SetDefaultExecCommand()
			assert.Equal(t, tc.fakeErr, result, "Unexpected result")
			assert.Equal(t, expCmd, execCommand.Cmd, "Unexpected command executed")
			assert.Equal(t, expArgs, execCommand.Args, "Unexpected command arguments")

		})
	}
}

func TestConfigL2Bridge(t *testing.T) {
	expCmd := "ovs-ofctl"
	bridge := "br0"
	expArgs := []string{"add-flow", "br0", "actions=NORMAL"}

	testCases := []struct {
		name    string
		fakeErr error
	}{
		{
			name:    "add L2 flow",
			fakeErr: nil,
		},
		{
			name:    "fail to add L2 flow",
			fakeErr: errors.New("Can't insert a flow"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execCommand := &FakeExecCommand{Err: tc.fakeErr}
			SetExecCommand(execCommand)
			result := configL2Bridge(bridge)
			SetDefaultExecCommand()
			assert.Equal(t, tc.fakeErr, result, "Unexpected result")
			assert.Equal(t, expCmd, execCommand.Cmd, "Unexpected command executed")
			assert.Equal(t, expArgs, execCommand.Args, "Unexpected command arguments")

		})
	}
}

func TestDeleteBridge(t *testing.T) {
	expCmd := "ovs-vsctl"
	bridge := "br0"
	expArgs := []string{"del-br", "br0"}

	testCases := []struct {
		name    string
		fakeErr error
	}{
		{
			name:    "delete bridge",
			fakeErr: nil,
		},
		{
			name:    "fail to delete bridge",
			fakeErr: errors.New("Can't delete bridge"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execCommand := &FakeExecCommand{Err: tc.fakeErr}
			SetExecCommand(execCommand)
			result := deleteBridge(bridge)
			SetDefaultExecCommand()
			assert.Equal(t, tc.fakeErr, result, "Unexpected result")
			assert.Equal(t, expCmd, execCommand.Cmd, "Unexpected command executed")
			assert.Equal(t, expArgs, execCommand.Args, "Unexpected command arguments")

		})
	}
}

func TestGetVhostPortMac(t *testing.T) {
	expCmd := "ovs-vsctl"
	socket := "tmp-socket"
	expArgs := []string{"--bare", "--columns=mac", "find", "port", "name=tmp-socket"}

	testCases := []struct {
		name      string
		fakeOut   []byte
		fakeErr   error
		expResult string
	}{
		{
			name:      "get MAC",
			fakeOut:   []byte("fe:ed:de:ad:be:ef"),
			fakeErr:   nil,
			expResult: "fe:ed:de:ad:be:ef",
		},
		{
			name:      "get MAC with one new line",
			fakeOut:   []byte("fe:ed:de:ad:be:ef\n"),
			fakeErr:   nil,
			expResult: "fe:ed:de:ad:be:ef",
		},
		{
			name:      "get MAC with multiple new lines",
			fakeOut:   []byte("fe:ed\n:de:ad\n:be:ef\n"),
			fakeErr:   nil,
			expResult: "fe:ed:de:ad:be:ef",
		},
		{
			name:      "fail to get MAC",
			fakeOut:   []byte("fe:ed:de:ad:be:ef"),
			fakeErr:   errors.New("Can't read MAC"),
			expResult: "",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execCommand := &FakeExecCommand{Out: tc.fakeOut, Err: tc.fakeErr}
			SetExecCommand(execCommand)
			result, err := getVhostPortMac(socket)
			SetDefaultExecCommand()
			assert.Equal(t, tc.expResult, result, "Unexpected result")
			assert.Equal(t, tc.fakeErr, err, "Unexpected error")
			assert.Equal(t, expCmd, execCommand.Cmd, "Unexpected command executed")
			assert.Equal(t, expArgs, execCommand.Args, "Unexpected command arguments")

		})
	}
}

func TestFindBridge(t *testing.T) {
	expCmd := "ovs-vsctl"
	bridge := "br0"
	expArgs := []string{"--bare", "--columns=name", "find", "bridge", "name=br0"}

	testCases := []struct {
		name      string
		fakeOut   []byte
		fakeErr   error
		expResult bool
	}{
		{
			name:      "find bridge",
			fakeOut:   []byte("br0"),
			fakeErr:   nil,
			expResult: true,
		},
		{
			name:      "fail to find bridge",
			fakeOut:   []byte(""),
			fakeErr:   errors.New("Can't find bridge"),
			expResult: false,
		},
		{
			name:      "fail to find bridge 2",
			fakeOut:   []byte("br0"),
			fakeErr:   errors.New("Can't find bridge"),
			expResult: false,
		},
		{
			name:      "fail to find bridge - bridge has invalid name",
			fakeOut:   []byte(""),
			fakeErr:   nil,
			expResult: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execCommand := &FakeExecCommand{Out: tc.fakeOut, Err: tc.fakeErr}
			SetExecCommand(execCommand)
			result := findBridge(bridge)
			SetDefaultExecCommand()
			assert.Equal(t, tc.expResult, result, "Unexpected result")
			assert.Equal(t, expCmd, execCommand.Cmd, "Unexpected command executed")
			assert.Equal(t, expArgs, execCommand.Args, "Unexpected command arguments")

		})
	}
}

func TestDoesBridgeContainInterfaces(t *testing.T) {
	expCmd := "ovs-vsctl"
	bridge := "br0"
	expArgs := []string{"list-ports", "br0"}

	testCases := []struct {
		name      string
		fakeOut   []byte
		fakeErr   error
		expResult bool
	}{
		{
			name:      "find interface connected to bridge",
			fakeOut:   []byte("eth2"),
			fakeErr:   nil,
			expResult: true,
		},
		{
			name:      "find interface with new line connected to bridge",
			fakeOut:   []byte("eth2\n"),
			fakeErr:   nil,
			expResult: true,
		},
		{
			name:      "find multiple interfaces connected to bridge",
			fakeOut:   []byte("eth2\neno15\ntun15\n"),
			fakeErr:   nil,
			expResult: true,
		},
		{
			name:      "fail to find interfaces",
			fakeOut:   []byte(""),
			fakeErr:   errors.New("Can't find interfaces"),
			expResult: false,
		},
		{
			name:      "fail to find interfaces 2",
			fakeOut:   []byte("eth2"),
			fakeErr:   errors.New("Can't find interfaces"),
			expResult: false,
		},
		{
			name:      "fail to find interfaces - interface has invalid name",
			fakeOut:   []byte(""),
			fakeErr:   nil,
			expResult: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execCommand := &FakeExecCommand{Out: tc.fakeOut, Err: tc.fakeErr}
			SetExecCommand(execCommand)
			result := doesBridgeContainInterfaces(bridge)
			SetDefaultExecCommand()
			assert.Equal(t, tc.expResult, result, "Unexpected result")
			assert.Equal(t, expCmd, execCommand.Cmd, "Unexpected command executed")
			assert.Equal(t, expArgs, execCommand.Args, "Unexpected command arguments")

		})
	}
}

func TestExecCommand(t *testing.T) {
	t.Run("verify execCommand", func(t *testing.T) {
		cmd := "echo"
		cmdArgs := []string{"param1", "param2"}
		expOut := []byte(strings.Join(cmdArgs, " ") + "\n")

		// test default (i.e. real) execCommand implementation
		SetDefaultExecCommand()
		out, err := execCommand(cmd, cmdArgs)

		assert.NoError(t, err, "Unexpected error")
		assert.Equal(t, expOut, out, "Unexpected result")
	})
}
