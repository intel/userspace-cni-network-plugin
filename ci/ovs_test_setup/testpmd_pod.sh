#!/bin/bash
CI_DIR="/runner/_work/userspace-cni-network-plugin/userspace-cni-network-plugin/ci/"
kubectl apply -f $CI_DIR/ovs_test_setup/ovs_network_attachment_definition.yaml
worker="kind-control-plane"
numbers=("1" "2")

for number in "${numbers[@]}"; do
docker exec -i kind-control-plane bash -c ' rm -rf "/var/run/openvswitch/app$number"'
docker exec -i kind-control-plane bash -c ' ls -lah /var/run/openvswitch/'
docker exec -i kind-control-plane bash -c ' mkdir -p "/var/run/openvswitch/app$number"'
docker exec -i kind-control-plane bash -c ' mkdir -p "/run/openvswitch/app$number"'

cat << EOF | kubectl apply -f -
---
apiVersion: v1
kind: Pod
metadata:
  name: ovs-app$number-$worker
  labels:
    name: ovs-app$number
  namespace: ovs
  annotations:
    k8s.v1.cni.cncf.io/networks: userspace-ovs-net-1
    userspace/mappedDir: /var/lib/cni/usrspcni/
spec:
  nodeSelector:
    kubernetes.io/hostname: $worker
  hostname: ovs-app$number-$worker
  subdomain: ovs
  containers:
  - image: testpmd
    imagePullPolicy: IfNotPresent
    name: ovs-app$number-$worker
    volumeMounts:
    - name: podinfo
      mountPath: /etc/podinfo
    - name: hugepage
      mountPath: /hugepages
    - name: shared-dir
      mountPath: /var/lib/cni/usrspcni/
    - name: scripts
      mountPath: /scripts/
    - name: vfio
      mountPath: /dev/vfio/
    resources:
      requests:
        hugepages-2Mi: 1Gi
        memory: "500Mi"
        cpu: "5"
      limits:
        hugepages-2Mi: 1Gi
        memory: "500Mi"
        cpu: "5"
#    command: ["/bin/sh"]
#    args: ["-c", "sleep inf"]
  restartPolicy: Always
  volumes:
    - name: podinfo
      downwardAPI:
        items:
          - path: "labels"
            fieldRef:
              fieldPath: metadata.labels
          - path: "annotations"
            fieldRef:
              fieldPath: metadata.annotations
    - name: hugepage
      emptyDir:
        medium: HugePages
    - name: shared-dir
#    - name: socket
      hostPath:
        path: /run/openvswitch/app$number
    - name: scripts
      hostPath:
        path: $CI_DIR/ovs_test_setup/
    - name: vfio
      hostPath:
        path: /dev/vfio/
EOF
done
