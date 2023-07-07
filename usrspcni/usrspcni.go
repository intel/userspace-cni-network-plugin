// Copyright 2019 Intel Corp.
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

package usrspcni

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/containernetworking/cni/pkg/skel"
	current "github.com/containernetworking/cni/pkg/types/100"

	"github.com/intel/userspace-cni-network-plugin/pkg/types"
)

//
// Exported Types
//
type UsrSpCni interface {
	AddOnHost(conf *types.NetConf,
		args *skel.CmdArgs,
		kubeClient kubernetes.Interface,
		sharedDir string,
		ipResult *current.Result) error
	AddOnContainer(conf *types.NetConf,
		args *skel.CmdArgs,
		kubeClient kubernetes.Interface,
		sharedDir string,
		pod *v1.Pod,
		ipResult *current.Result) (*v1.Pod, error)
	DelFromHost(conf *types.NetConf,
		args *skel.CmdArgs,
		sharedDir string) error
	DelFromContainer(conf *types.NetConf,
		args *skel.CmdArgs,
		sharedDir string,
		pod *v1.Pod) error
}
