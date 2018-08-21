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
// VPP implementation of the UserSpace CNI on the host will write
// configuration data in the from of json data to local files. The
// directory containing the files is then mapped to a container.
//
// This application is designed to run in a container, process the
// files written by the host and config the local VPP instance in
// the container. All the work is done in the cnivpp library. This
// is just a wrapper to access the library.
//

package main

import (
	"fmt"
	"time"

	"github.com/intel/vhost-user-net-plugin/cnivpp/cnivpp"
)

//
// Constants
//

//
// Types
//

//
// API Functions
//
func main() {
	var count int = 0
	var processed bool = false
	var processedCnt int = 0

	for {
		count++

		found, err := cnivpp.CniContainerConfig()

		if err != nil {
			fmt.Println("ERROR returned:", err)
		}

		fmt.Println("LOOP", count, " - FOUND:", found)

		//
		// Once files have been found, wait 1 more loop and exit.
		//
		if found {
			processed = true
		}

		if processed {
			processedCnt++

			if processedCnt > 1 {
				fmt.Println("DONE: Exiting vpp-app")
				break
			}
		}

		time.Sleep(20 * time.Second)
	}
}
