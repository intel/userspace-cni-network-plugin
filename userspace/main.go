/*
 * Copyright(c) 2022 Intel Corporation.
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"github.com/containernetworking/cni/pkg/skel"
	cniversion "github.com/containernetworking/cni/pkg/version"
	"github.com/intel/userspace-cni-network-plugin/userspace/cni"
)

// args *skel.CmdArgs, exec invoke.Exec, kubeClient kubernetes.Interface
func main() {
	skel.PluginMainFuncs(
		skel.CNIFuncs{
			Add: func(args *skel.CmdArgs) error {
				err := cni.CmdAdd(args, nil, nil)
				if err != nil {
					return err
				}
				return nil
			},

			Del: func(args *skel.CmdArgs) error { return cni.CmdDel(args, nil, nil) },

			Check: func(args *skel.CmdArgs) error {
				return cni.CmdGet(args, nil, nil)
			},
			GC:     nil,
			Status: nil},
		cniversion.All, "USERSPACE CNI Plugin")
}
