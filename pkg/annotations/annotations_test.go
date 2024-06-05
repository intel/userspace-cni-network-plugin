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

package annotations

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/intel/userspace-cni-network-plugin/pkg/types"
	"github.com/intel/userspace-cni-network-plugin/userspace/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	usrDir  = "userspace/mapped-dir"
	usrConf = "userspace/configuration-data"
)

func TestProvidedErrors(t *testing.T) {
	var err error
	err = &NoSharedDirProvidedError{"Error NoSharedDir"}
	require.Error(t, err, "Error was expected")
	assert.Equal(t, "Error NoSharedDir", err.Error(), "Unexpected error")

	err = &NoKubeClientProvidedError{"Error NoKubeClient"}
	require.Error(t, err, "Error was expected")
	assert.Equal(t, "Error NoKubeClient", err.Error(), "Unexpected error")

	err = &NoPodProvidedError{"Error NoPod"}
	require.Error(t, err, "Error was expected")
	assert.Equal(t, "Error NoPod", err.Error(), "Unexpected error")
}

func TestGetPodVolumeMountHostSharedDir(t *testing.T) {
	testCases := []struct {
		name       string
		podNil     bool
		volumes    []v1.Volume
		containers []v1.Container
		expErr     error
		expDir     string
	}{
		{
			name:    "pod with SharedDir",
			volumes: []v1.Volume{{Name: "shared-dir", VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/tmp/test-annotations"}}}},

			expDir: "/tmp/test-annotations",
		},
		{
			name:    "pod with SharedDir 2",
			volumes: []v1.Volume{{Name: "some-dir"}, {Name: "shared-dir", VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/tmp/test-annotations"}}}},

			expDir: "/tmp/test-annotations",
		},
		{
			name:       "pod with SharedDir 3",
			volumes:    []v1.Volume{{Name: "shared-dir", VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/tmp/test-annotations/volumes"}}}},
			containers: []v1.Container{{Name: "container", VolumeMounts: []v1.VolumeMount{{Name: "shared-dir", MountPath: "/tmp/test-annotations/containers"}}}},

			expDir: "/tmp/test-annotations/volumes",
		},
		{
			name:    "pod with EmptyDir",
			volumes: []v1.Volume{{Name: "shared-dir", VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}}}},

			expDir: "/var/lib/kubelet/pods/#UUID#/volumes/kubernetes.io~empty-dir/shared-dir",
		},
		{
			name:   "fail with pod set to nil",
			podNil: true,
			expErr: errors.New("Error: Pod not provided."),
		},
		{
			name:   "fail with pod without volumes",
			expErr: errors.New("Error: No Volumes."),
		},
		{
			name:    "fail with pod without shareddir volumes",
			volumes: []v1.Volume{{Name: "shared_dir"}, {Name: "shareddir"}},
			expErr:  errors.New("Error: No shared-dir. Need \"shared-dir\" in podSpec \"Volumes\""),
		},
		{
			name:    "fail with pod without shareddir HostPath",
			volumes: []v1.Volume{{Name: "shared-dir", VolumeSource: v1.VolumeSource{}}},
			expErr:  errors.New("Error: Volume is invalid"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var pod *v1.Pod

			if tc.podNil == false {
				pod = testdata.GetTestPod("")
				pod.Spec.Volumes = tc.volumes
				pod.Spec.Containers = tc.containers
				tc.expDir = strings.Replace(tc.expDir, "#UUID#", string(pod.UID), -1)
			}

			sharedDir, err := GetPodVolumeMountHostSharedDir(pod)

			if tc.expErr != nil {
				require.Error(t, err, "Error was expected")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected error")
			} else {
				require.NoError(t, err, "Unexpected error")
				assert.Equal(t, tc.expDir, sharedDir, "Unexpected result")
			}

		})
	}
}

func TestGetPodVolumeMountHostMappedSharedDir(t *testing.T) {
	testCases := []struct {
		name       string
		podNil     bool
		volumes    []v1.Volume
		containers []v1.Container
		expErr     error
		expDir     string
	}{
		{
			name:       "pod with SharedDir",
			containers: []v1.Container{{Name: "container", VolumeMounts: []v1.VolumeMount{{Name: "shared-dir", MountPath: "/tmp/test-annotations"}}}},

			expDir: "/tmp/test-annotations",
		},
		{
			name:       "pod with SharedDir 2",
			containers: []v1.Container{{Name: "init-container"}, {Name: "container", VolumeMounts: []v1.VolumeMount{{Name: "some-dir"}, {Name: "shared-dir", MountPath: "/tmp/test-annotations"}}}},

			expDir: "/tmp/test-annotations",
		},
		{
			name:       "pod with SharedDir 3",
			volumes:    []v1.Volume{{Name: "shared-dir", VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/tmp/test-annotations/volumes"}}}},
			containers: []v1.Container{{Name: "container", VolumeMounts: []v1.VolumeMount{{Name: "shared-dir", MountPath: "/tmp/test-annotations/containers"}}}},

			expDir: "/tmp/test-annotations/containers",
		},
		{
			name:       "fail with empty SharedDir",
			containers: []v1.Container{{Name: "container", VolumeMounts: []v1.VolumeMount{{Name: "shared-dir", MountPath: ""}}}},
			expErr:     errors.New("Error: No mapped shared-dir."),
		},
		{
			name:   "fail with pod set to nil",
			podNil: true,
			expErr: errors.New("Error: Pod not provided."),
		},
		{
			name:   "fail with pod without containers",
			expErr: errors.New("Error: No Containers. Need \"shared-dir\" in podSpec \"Volumes\""),
		},
		{
			name:       "fail with pod without shareddir containers",
			containers: []v1.Container{{Name: "container1", VolumeMounts: []v1.VolumeMount{{Name: "shareddir"}, {Name: "shared_dir"}}}, {Name: "container2", VolumeMounts: []v1.VolumeMount{{Name: "some-dir"}, {Name: "empty-dir"}}}},
			expErr:     errors.New("Error: No mapped shared-dir. Need \"shared-dir\" in podSpec \"Volumes\""),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var pod *v1.Pod

			if tc.podNil == false {
				pod = testdata.GetTestPod("")
				pod.Spec.Volumes = tc.volumes
				pod.Spec.Containers = tc.containers
			}

			sharedDir, err := getPodVolumeMountHostMappedSharedDir(pod)

			if tc.expErr != nil {
				require.Error(t, err, "Error was expected")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected error")
			} else {
				require.NoError(t, err, "Unexpected error")
				assert.Equal(t, tc.expDir, sharedDir, "Unexpected result")
			}

		})
	}
}

func TestWritePodAnnotation(t *testing.T) {
	testCases := []struct {
		name        string
		testType    string
		configData  *types.ConfigurationData
		annotations map[string]string
		expErr      error
	}{
		{
			name:       "write annotations",
			configData: &types.ConfigurationData{Name: "Container New Name", ContainerId: "123-456-789-007"},
		},
		{
			name:        "write annotations with modified config data",
			annotations: map[string]string{usrDir: "#sharedDir#"},
			configData:  &types.ConfigurationData{Name: "Container New Name", ContainerId: "123-456-789-007"},
		},
		{
			name:       "write annotations without sharedDir",
			testType:   "no_shareddir",
			configData: &types.ConfigurationData{Name: "Container New Name", ContainerId: "123-456-789-007"},
		},
		{
			name:        "fail when shared directoriers in Annotations and container are different",
			annotations: map[string]string{usrDir: "/proc/"},
			expErr:      errors.New("SetPodAnnotationMappedDir: Input "),
		},
		{
			name:     "fail when pod is not found",
			testType: "pod_not_found",
			expErr:   errors.New("status update failed for pod"),
		},
		{
			name:     "fail with pod set to nil",
			testType: "pod_nil",
			expErr:   errors.New("Error: Pod not provided."),
		},
		{
			name:     "fail with kubeClient set to nil",
			testType: "client_nil",
			expErr:   errors.New("Error: KubeClient not provided."),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var kubeClient kubernetes.Interface
			var pod *v1.Pod

			sharedDir, dirErr := os.MkdirTemp("/tmp", "test-annotation-")
			require.NoError(t, dirErr, "Can't create temporary directory")
			defer os.RemoveAll(sharedDir)

			switch tc.testType {
			case "client_nil":
				pod = testdata.GetTestPod(sharedDir)
				kubeClient = nil
			case "pod_nil":
				pod = nil
				kubeClient = fake.NewSimpleClientset()
			case "pod_not_found":
				pod = testdata.GetTestPod(sharedDir)
				kubeClient = fake.NewSimpleClientset()
			case "no_shareddir":
				pod = testdata.GetTestPod("")
				kubeClient = fake.NewSimpleClientset(pod)
			default:
				pod = testdata.GetTestPod(sharedDir)
				kubeClient = fake.NewSimpleClientset(pod)
			}

			if pod != nil && tc.annotations != nil {
				pod.Annotations = tc.annotations
				pod.Annotations[usrDir] = strings.Replace(pod.Annotations[usrDir], "#sharedDir#", sharedDir, -1)
			}

			resPod, err := WritePodAnnotation(kubeClient, pod, tc.configData)

			if tc.expErr != nil {
				require.Error(t, err, "Error was expected")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected error")
			} else {
				require.NoError(t, err, "Unexpected error")
				assert.NotEmpty(t, resPod.Annotations[usrConf], "Unexpected result")
				assert.Contains(t, resPod.Annotations[usrConf], "Container New Name", "Unexpected result")
				assert.Contains(t, resPod.Annotations[usrConf], "123-456-789-007", "Unexpected result")
			}

		})
	}
}

func TestSetPodAnnotationMappedDir(t *testing.T) {
	testCases := []struct {
		name        string
		mappedDir   string
		annotations map[string]string
		containers  []v1.Container
		podNil      bool
		expErr      error
		expResult   bool
	}{
		{
			name:      "write annotations",
			mappedDir: "/tmp/test-annotations",
			expResult: true,
		},
		{
			name:        "write annotations 2",
			annotations: map[string]string{"some-dir": "/tmp/test-annotations/some"},
			mappedDir:   "/tmp/test-annotations",
			expResult:   true,
		},
		{
			name:        "write annotations with existing mappedDir",
			annotations: map[string]string{usrDir: "/tmp/test-annotations"},
			mappedDir:   "/tmp/test-annotations",
			expResult:   false,
		},
		{
			name:        "write annotations with existing mappedDir 2",
			annotations: map[string]string{usrDir: "/tmp/test-annotations/"},
			mappedDir:   "/tmp/test-annotations",
			expResult:   false,
		},
		{
			name:        "fail to write annotations with different mappedDir",
			annotations: map[string]string{usrDir: "/proc/"},
			mappedDir:   "/tmp/test-annotations",
			expErr:      errors.New("SetPodAnnotationMappedDir: Input"),
		},
		{
			name:   "fail with pod set to nil",
			podNil: true,
			expErr: errors.New("Error: Pod not provided."),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var pod *v1.Pod

			if tc.podNil == false {
				pod = testdata.GetTestPod("")
				pod.Spec.Containers = tc.containers
				pod.Annotations = tc.annotations
			}

			result, err := setPodAnnotationMappedDir(pod, tc.mappedDir)

			if tc.expErr != nil {
				require.Error(t, err, "Error was expected")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected error")
			} else {
				require.NoError(t, err, "Unexpected error")
				assert.Equal(t, tc.expResult, result, "Unexpected result")
				assert.Equal(t, filepath.Clean(pod.Annotations[usrDir]), filepath.Clean(tc.mappedDir), "Unexpected result")
			}

		})
	}
}

func TestSetPodAnnotationConfigData(t *testing.T) {
	testCases := []struct {
		name        string
		configData  *types.ConfigurationData
		annotations map[string]string
		containers  []v1.Container
		podNil      bool
		expErr      error
		expResult   bool
	}{
		{
			name:       "write annotations",
			configData: &types.ConfigurationData{Name: "Container New Name", ContainerId: "123-456-789-007", IfName: "eth7"},
			expResult:  true,
		},
		{
			name:        "write annotations for another interface",
			annotations: map[string]string{usrConf: "[{\n    \"containerId\": \"123-456-789-007\",\n    \"ifName\": \"eth9\",\n    \"name\": \"Container Old Name\"}]"},

			configData: &types.ConfigurationData{Name: "Container New Name", ContainerId: "123-456-789-007", IfName: "eth7"},
			expResult:  true,
		},
		{
			// NOTE: current implementation just appends new data, old one are not updated
			name:        "write annotations for the same interface",
			annotations: map[string]string{usrConf: "[{\n    \"containerId\": \"123-456-789-007\",\n    \"ifName\": \"eth7\",\n    \"name\": \"Container New Name\"}]"},

			configData: &types.ConfigurationData{Name: "Container New Name", ContainerId: "123-456-789-007", IfName: "eth7"},
			expResult:  true,
		},
		{
			name:   "fail with pod set to nil",
			podNil: true,
			expErr: errors.New("Error: Pod not provided."),
		},
		{
			name:       "write annotations with config data set to nil",
			configData: nil,
			expResult:  false,
		},
		{
			name:        "write annotations for another interface with config data set to nil",
			annotations: map[string]string{usrConf: "[{\n    \"containerId\": \"123-456-789-007\",\n    \"ifName\": \"eth9\",\n    \"name\": \"Container Old Name\"}]"},
			configData:  nil,
			expResult:   false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var pod *v1.Pod
			var origData string

			if tc.podNil == false {
				pod = testdata.GetTestPod("")
				pod.Spec.Containers = tc.containers
				pod.Annotations = tc.annotations
				origData = tc.annotations[usrConf]
			}

			result, err := setPodAnnotationConfigData(pod, tc.configData)

			if tc.expErr != nil {
				require.Error(t, err, "Error was expected")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected error")
			} else {
				require.NoError(t, err, "Unexpected error")
				assert.Equal(t, tc.expResult, result, "Unexpected result")
				if tc.configData == nil {
					assert.Equal(t, origData, pod.Annotations[usrConf], origData, "Unexpected result")
				} else {
					assert.NotEmpty(t, pod.Annotations[usrConf], "Unexpected result")
					assert.Contains(t, pod.Annotations[usrConf], "Container New Name", "Unexpected result")
					assert.Contains(t, pod.Annotations[usrConf], "123-456-789-007", "Unexpected result")
					assert.Contains(t, pod.Annotations[usrConf], strings.TrimSuffix(origData, "}]"), "Unexpected result")
				}
			}

		})
	}
}

func TestCommitAnnotation(t *testing.T) {
	testCases := []struct {
		name     string
		testType string
		expErr   error
	}{
		{
			name: "write annotations",
		},
		{
			name:     "fail with pod set to nil",
			testType: "pod_nil",
			expErr:   errors.New("WritePodAnnotation: No pod:"),
		},
		{
			name:     "fail with kubeClient set to nil",
			testType: "client_nil",
			expErr:   errors.New("WritePodAnnotation: No kubeClient:"),
		},
		{
			name:     "fail to get pod",
			testType: "pod_not_found",
			expErr:   errors.New("status update failed for pod"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var kubeClient kubernetes.Interface
			var pod *v1.Pod

			sharedDir, dirErr := os.MkdirTemp("/tmp", "test-k8sclient-")
			require.NoError(t, dirErr, "Can't create temporary directory")
			defer os.RemoveAll(sharedDir)

			switch tc.testType {
			case "client_nil":
				kubeClient = nil
			case "pod_not_found":
				pod = testdata.GetTestPod(sharedDir)
				kubeClient = fake.NewSimpleClientset()
			case "pod_nil":
				pod = nil
				kubeClient = fake.NewSimpleClientset()
			default:
				pod = testdata.GetTestPod(sharedDir)
				kubeClient = fake.NewSimpleClientset(pod)
			}

			origPod := pod.DeepCopy()
			resPod, err := commitAnnotation(kubeClient, pod)

			if tc.expErr != nil {
				require.Error(t, err, "Error was expected")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected error")
			} else {
				require.NoError(t, err, "Unexpected error")
				assert.Equal(t, origPod, resPod, "Unexpected result")
			}

		})
	}
}

func TestGetFileAnnotation(t *testing.T) {
	testCases := []struct {
		name        string
		annotIndex  string
		annot       string
		defaultFile bool
		expErr      error
		expResult   string
	}{
		{
			name:       "get key from annotations",
			annot:      "key=value",
			annotIndex: "key",
			expResult:  "value",
		},
		{
			name:       "get key from annotations 2",
			annot:      "any=key has=some key=value take=it",
			annotIndex: "key",
			expResult:  "value",
		},
		{
			name:       "get key from annotations 3",
			annot:      "any=key\nhas=some key=value take=it\n",
			annotIndex: "key",
			expResult:  "value",
		},
		{
			name:       "fail to get key",
			annot:      "key=value",
			annotIndex: "nokey",
			expErr:     errors.New(`ERROR: "nokey" missing from pod annotation`),
		},
		{
			name:       "fail to get annotations",
			annotIndex: "nokey",
			expErr:     errors.New("error reading"),
		},
		{
			name:        "fail to get annotations from default file",
			annotIndex:  "n0keyShallBeF0und",
			defaultFile: true,
			expErr:      errors.New("error reading"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var annotFile, testDir string
			var err error
			var result []byte

			if tc.defaultFile {
				// modify expected error in case that default file exists
				if _, err = os.Stat("/etc/podinfo/annotations"); err == nil {
					tc.expErr = fmt.Errorf(fmt.Sprintf(`ERROR: "%v" missing from pod annotation`, tc.annotIndex))
				}
				result, err = getFileAnnotation(tc.annotIndex, "")
			} else {
				testDir, err = os.MkdirTemp("/tmp", "test-annotation-")
				require.NoError(t, err, "Can't create temporary directory")
				defer os.RemoveAll(testDir)

				annotFile = filepath.Join(testDir, "annotations")
				if len(tc.annot) > 0 {
					_ = os.WriteFile(annotFile, []byte(tc.annot), 0644)
				}
				result, err = getFileAnnotation(annotFile, tc.annotIndex)
			}

			if tc.expErr != nil {
				require.Error(t, err, "Error was expected")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected error")
			} else {
				require.NoError(t, err, "Unexpected error")
				require.Equal(t, tc.expResult, string(result), "Unexpected result")
			}

		})
	}
}

func TestGetFileAnnotationMappedDir(t *testing.T) {
	testCases := []struct {
		name      string
		annot     string
		expErr    error
		expResult string
	}{
		{
			name:      "get mapped dir from annotations",
			annot:     "userspace/mapped-dir=/tmp/test-annotations",
			expResult: "/tmp/test-annotations",
		},
		{
			name:   "fail to get mapped dir",
			annot:  "userspace/mappeddir=/tmp/test-annotations",
			expErr: errors.New(`ERROR: "userspace/mapped-dir" missing from pod annotation`),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testDir, err := os.MkdirTemp("/tmp", "test-annotation-")
			require.NoError(t, err, "Can't create temporary directory")
			defer os.RemoveAll(testDir)

			annotFile := filepath.Join(testDir, "annotations")
			if len(tc.annot) > 0 {
				_ = os.WriteFile(annotFile, []byte(tc.annot), 0644)
			}
			result, err := GetFileAnnotationMappedDir(annotFile)

			if tc.expErr != nil {
				require.Error(t, err, "Error was expected")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected error")
				assert.Empty(t, result, "Unexpected result")
			} else {
				require.NoError(t, err, "Unexpected error")
				require.Equal(t, tc.expResult, result, "Unexpected result")
			}

		})
	}
}

func TestGetFileAnnotationConfigData(t *testing.T) {
	testCases := []struct {
		name      string
		annot     string
		expErr    error
		expResult []*types.ConfigurationData
	}{
		{
			name:      "get configuration data from annotations",
			annot:     `userspace/configuration-data="[{\"Name\":\"Container New Name\",\"ContainerId\":\"123-456-789-007\"}]"`,
			expResult: []*types.ConfigurationData{{Name: "Container New Name", ContainerId: "123-456-789-007"}},
		},
		{
			name:   "fail to get configuration data",
			annot:  "userspace/configurationdata=",
			expErr: errors.New(`ERROR: "userspace/configuration-data" missing from pod annotation`),
		},
		{
			name:   "fail to parse configuration data",
			annot:  `userspace/configuration-data="[{\"Name\":\"Container\"]"`,
			expErr: errors.New("GetFileAnnotationConfigData: Failed to parse ConfigData Annotation JSON format:"),
		},
		{
			name:   "fail to parse configuration data 2",
			annot:  "userspace/configuration-data=invalid-json",
			expErr: errors.New("GetFileAnnotationConfigData: Invalid formatted JSON data"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testDir, err := os.MkdirTemp("/tmp", "test-annotation-")
			require.NoError(t, err, "Can't create temporary directory")
			defer os.RemoveAll(testDir)

			annotFile := filepath.Join(testDir, "annotations")
			if len(tc.annot) > 0 {
				_ = os.WriteFile(annotFile, []byte(tc.annot), 0644)
			}
			result, err := GetFileAnnotationConfigData(annotFile)

			if tc.expErr != nil {
				require.Error(t, err, "Error was expected")
				assert.Contains(t, err.Error(), tc.expErr.Error(), "Unexpected error")
				assert.Nil(t, result, "Unexpected result")
			} else {
				require.NoError(t, err, "Unexpected error")
				require.Equal(t, tc.expResult, result, "Unexpected result")
			}

		})
	}
}
