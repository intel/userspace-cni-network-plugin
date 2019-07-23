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

package k8sclient

import (
	"os"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"

	_ "github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"

	"github.com/intel/userspace-cni-network-plugin/logging"
	"github.com/intel/userspace-cni-network-plugin/usrsptypes"
)

// defaultKubeClient implements KubeClient
type defaultKubeClient struct {
	client kubernetes.Interface
}

var _ KubeClient = &defaultKubeClient{}

func (d *defaultKubeClient) GetRawWithPath(path string) ([]byte, error) {
	return d.client.ExtensionsV1beta1().RESTClient().Get().AbsPath(path).DoRaw()
}

func (d *defaultKubeClient) GetPod(namespace, name string) (*v1.Pod, error) {
	return d.client.Core().Pods(namespace).Get(name, metav1.GetOptions{})
}

func (d *defaultKubeClient) UpdatePodStatus(pod *v1.Pod) (*v1.Pod, error) {
	return d.client.Core().Pods(pod.Namespace).UpdateStatus(pod)
}

type KubeClient interface {
	GetRawWithPath(path string) ([]byte, error)
	GetPod(namespace, name string) (*v1.Pod, error)
	UpdatePodStatus(pod *v1.Pod) (*v1.Pod, error)
}

func getK8sArgs(args *skel.CmdArgs) (*usrsptypes.K8sArgs, error) {
	k8sArgs := &usrsptypes.K8sArgs{}

	logging.Verbosef("getK8sArgs: %v", args)
	err := cnitypes.LoadArgs(args.Args, k8sArgs)
	if err != nil {
		return nil, err
	}

	return k8sArgs, nil
}

func getK8sClient(kubeClient KubeClient, kubeConfig string) (KubeClient, error) {
	logging.Verbosef("getK8sClient: %s, %v", kubeClient, kubeConfig)
	// If we get a valid kubeClient (eg from testcases) just return that
	// one.
	if kubeClient != nil {
		return kubeClient, nil
	}

	var err error
	var config *rest.Config

	// Otherwise try to create a kubeClient from a given kubeConfig
	if kubeConfig != "" {
		// uses the current context in kubeConfig
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			return nil, logging.Errorf("getK8sClient: failed to get context for the kubeConfig %v, refer Multus README.md for the usage guide: %v", kubeConfig, err)
		}
	} else if os.Getenv("KUBERNETES_SERVICE_HOST") != "" && os.Getenv("KUBERNETES_SERVICE_PORT") != "" {
		// Try in-cluster config where multus might be running in a kubernetes pod
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, logging.Errorf("createK8sClient: failed to get context for in-cluster kube config, refer Multus README.md for the usage guide: %v", err)
		}
	} else {
		// No kubernetes config; assume we shouldn't talk to Kube at all
		return nil, nil
	}

	// Specify that we use gRPC
	config.AcceptContentTypes = "application/vnd.kubernetes.protobuf,application/json"
	config.ContentType = "application/vnd.kubernetes.protobuf"

	// creates the clientset
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &defaultKubeClient{client: client}, nil
}

func GetPod(args *skel.CmdArgs, kubeClient KubeClient, kubeConfig string) (*v1.Pod, error) {
	var err error

	logging.Verbosef("GetPod: ENTER - %v, %v, %v", args, kubeClient, kubeConfig)

	// Get k8sArgs
	k8sArgs, err := getK8sArgs(args)
	if err != nil {
		logging.Errorf("GetPod: Err in getting k8s args: %v", err)
		return nil, err
	}

	// Get kubeClient. If passed in, GetK8sClient() will just return it back.
	kubeClient, err = getK8sClient(kubeClient, kubeConfig)
	if err != nil {
		logging.Errorf("GetPod: Err in getting kubeClient: %v", err)
		return nil, err
	}

	if kubeClient == nil {
		logging.Errorf("GetPod: No kubeClient: %v", err)
		return nil, err
	}

	// Get the pod info. If cannot get it, we use cached delegates
	pod, err := kubeClient.GetPod(string(k8sArgs.K8S_POD_NAMESPACE), string(k8sArgs.K8S_POD_NAME))
	if err != nil {
		logging.Debugf("GetPod: Err in loading K8s cluster default network from pod annotation: %v, use cached delegates", err)
		return nil, err
	}

	logging.Verbosef("pod.Annotations: %v", pod.Annotations)

	return pod, err
}

func WritePodAnnotation(kubeClient KubeClient, kubeConfig string, pod *v1.Pod) (*v1.Pod, error) {
	var err error

	// Get kubeClient. If passed in, getK8sClient() will just return it back.
	kubeClient, err = getK8sClient(kubeClient, kubeConfig)
	if err != nil {
		logging.Errorf("WritePodAnnotation: Err in getting kubeClient: %v", err)
		return pod, err
	}

	if kubeClient == nil {
		logging.Errorf("WritePodAnnotation: No kubeClient: %v", err)
		return pod, err
	}

	// Update the pod
	pod = pod.DeepCopy()
	if resultErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err != nil {
			// Re-get the pod unless it's the first attempt to update
			pod, err = kubeClient.GetPod(pod.Namespace, pod.Name)
			if err != nil {
				return err
			}
		}

		pod, err = kubeClient.UpdatePodStatus(pod)
		return err
	}); resultErr != nil {
		return nil, logging.Errorf("status update failed for pod %s/%s: %v", pod.Namespace, pod.Name, resultErr)
	}
	return pod, nil
}
