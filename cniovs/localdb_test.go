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
	"os"
	"path"
	"testing"

	"github.com/intel/userspace-cni-network-plugin/pkg/annotations"
	"github.com/intel/userspace-cni-network-plugin/userspace/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveConfig(t *testing.T) {
	args := testdata.GetTestArgs()

	testCases := []struct {
		name string
		data *OvsSavedData
	}{
		{
			name: "save and load data 1",
			data: &OvsSavedData{Vhostname: "vhost0", VhostMac: "fe:ed:de:ad:be:ef", IfMac: "co:co:ca:fe:da:da"},
		},
		{
			name: "save and load data 2",
			data: &OvsSavedData{VhostMac: "fe:ed:de:ad:be:ef", IfMac: "co:co:ca:fe:da:da"},
		},
		{
			name: "save and load data 3",
			data: &OvsSavedData{VhostMac: "fe:ed:de:ad:be:ef"},
		},
		{
			name: "save and load data 4",
			data: &OvsSavedData{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var data OvsSavedData
			require.NoError(t, SaveConfig(nil, args, tc.data), "Unexpected error")
			require.NoError(t, LoadConfig(nil, args, &data), "Can't read stored data")
			assert.Equal(t, tc.data, &data, "Unexpected data retrieved")
		})

	}
}

func TestLoadConfig(t *testing.T) {
	args := testdata.GetTestArgs()

	testCases := []struct {
		name     string
		jsonFile string
		expErr   error
		expData  *OvsSavedData
	}{
		// test error cases; Successful config load is tested by TestSaveConfig
		{
			name:     "no file with saved data",
			jsonFile: "none",
			expErr:   nil,
			expData:  &OvsSavedData{},
		},
		{
			name:     "fail to load corrupted JSON",
			jsonFile: "corrupted",
			expErr:   errors.New("ERROR: Failed to parse"),
			expData:  &OvsSavedData{},
		},
		{
			name:     "fail to read broken file",
			jsonFile: "directory",
			expErr:   errors.New("ERROR: Failed to read"),
			expData:  &OvsSavedData{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			localDir := annotations.DefaultLocalCNIDir
			fileName := fmt.Sprintf("local-%s-%s.json", args.ContainerID[:12], args.IfName)
			if _, err := os.Stat(localDir); err != nil {
				require.NoError(t, os.MkdirAll(localDir, 0700), "Can't create data dir")
				defer os.RemoveAll(localDir)
			}
			path := path.Join(localDir, fileName)

			switch tc.jsonFile {
			case "none":
				require.NoFileExists(t, path, "Saved configuration shall not exist")
			case "corrupted":
				require.NoError(t, os.WriteFile(path, []byte("{"), 0644), "Can't create test file")
				defer os.Remove(path)
			case "directory":
				require.NoError(t, os.Mkdir(path, 0700), "Can't create test dir")
				defer os.RemoveAll(path)
			}
			var data OvsSavedData
			err := LoadConfig(nil, args, &data)
			if tc.expErr == nil {
				assert.Equal(t, tc.expErr, err, "Unexpected result")
			} else {
				require.Error(t, err, "Unexpected result")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected result")
			}
			assert.Equal(t, tc.expData, &data, "Unexpected result")
		})

	}
}
