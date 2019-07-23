// Copyright (c) 2018 Red Hat.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//
// This module provides the library functions to implement the
// VPP UserSpace CNI implementation. The input to the library is json
// data defined in usrsptypes. If the configuration contains local data,
// the 'api' library is used to send the request to the local govpp-agent,
// which provisions the local VPP instance. If the configuration contains
// remote data, the database library is used to store the data, which is
// later read and processed locally by the remotes agent (usrapp-app running
// in the container)
//

package annotations

import (
	"encoding/json"
	"fmt"
	"strings"
	"bytes"
	"io/ioutil"

	v1 "k8s.io/api/core/v1"

	"github.com/containernetworking/cni/pkg/types/current"

	"github.com/go-logfmt/logfmt"

	"github.com/intel/userspace-cni-network-plugin/logging"
	"github.com/intel/userspace-cni-network-plugin/usrsptypes"
	"github.com/intel/userspace-cni-network-plugin/k8sclient"
)


// Annotation
// These structures are used to document the set of annotations used in
// the Userspace CNI pod spec to pass data from Admission Controller to
// the CNI and from the CNI to the Container.

// List of Annotations supported in the podSpec
const (
	networkAttachmentAnnot       = "k8s.v1.cni.cncf.io/networks"
	networkAttachmentStatusAnnot = "k8s.v1.cni.cncf.io/networks-status"
	configDataAnnot              = "userspace-cni/configuration-data"
	mappedDirAnnot               = "userspace-cni/mappedDir"
	sharedDirVolumeName          = "shared-dir"
)


// configDataAnnot
// Used by CNI to pass configuration data to a container. 
// Populated by CNI.
// Example:
//  configData: '{
//        "container": {
//                "engine": "vpp",
//                "iftype": "memif",
//                "netType": "interface",
//                "memif": {
//                        "role": "slave",
//                        "mode": "ethernet"
//                }
//        }
//    }'
type ConfigData struct {
	ContainerId   string                   `json:"containerId"` // From args.ContainerId, used locally. Used in several place, namely in the socket filenames.
	IfName        string                   `json:"ifName"`      // From args.IfName, used locally. Used in several place, namely in the socket filenames.
	NetConf       usrsptypes.NetConf       `json:"config"`
	IPResult      current.Result           `json:"ipResult"`    // Network Status also has IP, but wrong format
}

// Errors returned from this module
type NoSharedDirProvidedError struct {
	message string
}
func (e *NoSharedDirProvidedError) Error() string { return string(e.message) }


func GetPodVolumeMountHostSharedDir(pod *v1.Pod) (string, error) {
	var hostSharedDir string

	logging.Verbosef("GetPodVolumeMountSharedDir: type=%T Volumes=%v", pod.Spec.Volumes, pod.Spec.Volumes)

	if len(pod.Spec.Volumes) == 0 {
		return hostSharedDir, &NoSharedDirProvidedError{"Error: No Volumes. Need \"shared-dir\" in podSpec \"Volumes\""}
	}

	for _, volumeMount := range pod.Spec.Volumes {
		if volumeMount.Name == sharedDirVolumeName {
			hostSharedDir = volumeMount.HostPath.Path
			break
		}
	}

	if len(hostSharedDir) == 0 {
		return hostSharedDir, &NoSharedDirProvidedError{"Error: No shared-dir. Need \"shared-dir\" in podSpec \"Volumes\""}
	}

	return hostSharedDir, nil
}

func GetPodVolumeMountHostMappedSharedDir(pod *v1.Pod) (string, error) {
	var mappedSharedDir string

	logging.Verbosef("GetPodVolumeMountHostMappedSharedDir: type=%T Containers=%v", pod.Spec.Containers, pod.Spec.Containers)

	if len(pod.Spec.Containers) == 0 {
		return mappedSharedDir, &NoSharedDirProvidedError{"Error: No Containers. Need \"shared-dir\" in podSpec \"Volumes\""}
	}

	for _, container := range pod.Spec.Containers {
		if len(container.VolumeMounts) != 0 {
			for _, volumeMount := range container.VolumeMounts {
				if volumeMount.Name == sharedDirVolumeName {
					mappedSharedDir = volumeMount.MountPath
					break
				}
			}
		}
	}

	if len(mappedSharedDir) == 0 {
		return mappedSharedDir, &NoSharedDirProvidedError{"Error: No mapped shared-dir. Need \"shared-dir\" in podSpec \"Volumes\""}
	}

	return mappedSharedDir, nil
}

func SetPodAnnotationMappedDir(kubeClient k8sclient.KubeClient,
							   kubeConfig string,
							   pod *v1.Pod,
							   mappedDir string) (*v1.Pod, error) {
	logging.Verbosef("SetPodAnnotationMappedDir: inputMappedDir=%s Annot - type=%T mappedDir=%v", mappedDir, pod.Annotations[mappedDirAnnot], pod.Annotations[mappedDirAnnot])

	// If pod annotations is empty, make sure it allocatable
	if len(pod.Annotations) == 0 {
		pod.Annotations = make(map[string]string)
	}

	// Get current data, if any. The current data is a string containing the
	// directory in the container to find shared files. If the data already exists,
	// it should be the same as the input data.
	annotDataStr := pod.Annotations[mappedDirAnnot]
	if len(annotDataStr) != 0 {
		if annotDataStr == mappedDir {
			logging.Verbosef("SetPodAnnotationMappedDir: Existing matches input. Do nothing.")
			return pod, nil
		} else {
			return pod, logging.Errorf("SetPodAnnotationMappedDir: Input \"%s\" does not match existing \"%s\"", mappedDir, annotDataStr)
		}
	}

	// Append the just converted JSON string to any existing strings and
	// store in the annotation in the pod.
	pod.Annotations[mappedDirAnnot] = mappedDir

	// Write the annotation back to the pod.
	return k8sclient.WritePodAnnotation(kubeClient, kubeConfig, pod)
}

func SetPodAnnotationConfigData(kubeClient k8sclient.KubeClient,
								kubeConfig string,
								pod *v1.Pod,
								configData *ConfigData) (*v1.Pod, error) {
	var configDataStr []string

	logging.Verbosef("SetPodAnnotationConfigData: type=%T configData=%v", pod.Annotations[configDataAnnot], pod.Annotations[configDataAnnot])

	// If pod annotations is empty, make sure it allocatable
	if len(pod.Annotations) == 0 {
		pod.Annotations = make(map[string]string)
	}

	// Get current data, if any. The current data is a string in JSON format with 
	// data for multiple interfaces appended together. A given container can have
	// multiple interfaces, added one at a time. So existing data may be empty if
	// this is the first interface, or already contain data.
	annotDataStr := pod.Annotations[configDataAnnot]
	if len(annotDataStr) != 0 {
		// Strip wrapping [], will be added back around entire field.
		annotDataStr = strings.TrimLeft(annotDataStr, "[")
		annotDataStr = strings.TrimRight(annotDataStr, "]")

		// Add current string to slice of strings.
		configDataStr = append(configDataStr, annotDataStr)
	}

	// Marshal the input config data struct into a JSON string.
	data, err := json.MarshalIndent(configData, "", "    ")
	if err != nil {
		return pod, logging.Errorf("SetPodAnnotationConfigData: error with Marshal Indent: %v", err)
	}
	configDataStr = append(configDataStr, string(data))

	// Append the just converted JSON string to any existing strings and
	// store in the annotation in the pod.
	pod.Annotations[configDataAnnot] = fmt.Sprintf("[%s]", strings.Join(configDataStr, ","))

	// Write the annotation back to the pod.
	return k8sclient.WritePodAnnotation(kubeClient, kubeConfig, pod)
}

//
// Container Access Functions
// These functions can be called from code running in a container. It reads
// the data from the exposed Downward API.
//
const DefaultAnnotationsFile = "/etc/podinfo/annotations"

func getFileAnnotation(annotIndex string) ([]byte, error) {
	var rawData []byte

	fileData, err := ioutil.ReadFile(DefaultAnnotationsFile)
	if err != nil {
		logging.Errorf("getFileAnnotation: File Read ERROR - %v", err)
		return rawData, fmt.Errorf("error reading %s: %s", DefaultAnnotationsFile, err)
	}

	d := logfmt.NewDecoder(bytes.NewReader(fileData))
	for d.ScanRecord() {
		for d.ScanKeyval() {
			//fmt.Printf("k: %T %s v: %T %s\n", d.Key(), d.Key(), d.Value(), d.Value())
			//logging.Debugf("  k: %T %s v: %T %s\n", d.Key(), d.Key(), d.Value(), d.Value())

			if string(d.Key()) == annotIndex {
				rawData = d.Value()
				return rawData, nil
			}
		}
		//fmt.Println()
	}

	return rawData, fmt.Errorf("ERROR: \"%s\" missing from pod annotation", annotIndex)
}

func GetFileAnnotationMappedDir() (string, error) {
	rawData, err := getFileAnnotation(mappedDirAnnot)
	if err != nil {
		return "", err
	}

	return string(rawData), err	
}

func GetFileAnnotationConfigData() ([]*ConfigData, error) {
	var configDataList []*ConfigData

	// Remove
	logging.Debugf("GetFileAnnotationConfigData: ENTER")

	rawData, err := getFileAnnotation(configDataAnnot)
	if err != nil {
		return nil, err
	}

	rawString := string(rawData)
	if strings.IndexAny(rawString, "[{\"") >= 0 {
		if err := json.Unmarshal([]byte(rawString), &configDataList); err != nil {
			return nil, logging.Errorf("GetFileAnnotationConfigData: Failed to parse ConfigData Annotation JSON format: %v", err)
		}
	} else {
		return nil, logging.Errorf("GetFileAnnotationConfigData: Invalid formatted JSON data")
	}

	return configDataList, err	
}

//func GetFileAnnotationNetworksStatus() ([]*multusTypes.NetworkStatus, error) {
//	var networkStatusList []*multusTypes.NetworkStatus
//
//	// Remove
//	logging.Debugf("GetFileAnnotationNetworksStatus: ENTER")
//
//	rawData, err := getFileAnnotation(networkAttachmentStatusAnnot)
//	if err != nil {
//		return nil, err
//	}
//
//	rawString := string(rawData)
//	if strings.IndexAny(rawString, "[{\"") >= 0 {
//		if err := json.Unmarshal([]byte(rawString), &networkStatusList); err != nil {
//			return nil, logging.Errorf("GetFileAnnotationNetworksStatus: Failed to parse networkStatusList Annotation JSON format: %v", err)
//		}
//	} else {
//		return nil, logging.Errorf("GetFileAnnotationNetworksStatus: Invalid formatted JSON data")
//	}
//
//	return networkStatusList, err	
//}