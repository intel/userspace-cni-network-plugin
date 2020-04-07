package testdata

import (
	"fmt"

	"github.com/containernetworking/cni/pkg/skel"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

//
// Unit test related functions
//
func GetTestPod(sharedDir string) *v1.Pod {
	id := uuid.NewUUID()
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			UID:       id,
			Name:      fmt.Sprintf("pod-%v", id[:8]),
			Namespace: fmt.Sprintf("namespace-%v", id[:8]),
		},
	}
	if sharedDir != "" {
		pod.Spec.Volumes = append(pod.Spec.Volumes,
			v1.Volume{
				Name: "shared-dir",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: sharedDir,
					},
				},
			})
		pod.Spec.Containers = append(pod.Spec.Containers,
			v1.Container{
				Name:         "container",
				VolumeMounts: []v1.VolumeMount{{Name: "shared-dir", MountPath: sharedDir}},
			})
	}
	return pod
}

func GetTestArgs() *skel.CmdArgs {
	id := uuid.NewUUID()
	return &skel.CmdArgs{
		ContainerID: string(id),
		IfName:      fmt.Sprintf("eth%v", int(id[7])),
		StdinData:   []byte("{}"),
	}
}
