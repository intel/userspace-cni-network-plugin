#!/bin/bash

CI_DIR="/runner/_work/userspace-cni-network-plugin/userspace-cni-network-plugin/ci/"
kubectl apply -f $CI_DIR/vpp_test_setup/network_attachment_definition.yaml
kubectl create -n vpp configmap vpp-app-startup-config --from-file=$CI_DIR/vpp_test_setup/startup.conf
worker="kind-control-plane"
numbers=("1" "2")

for number in "${numbers[@]}"; do
docker exec -i kind-control-plane bash -c "rm -rf /var/run/vpp/app$number"
docker exec -i kind-control-plane bash -c "ls -lah /var/run/vpp/"
docker exec -i kind-control-plane bash -c "mkdir -p /var/run/vpp/app$number"

cat << EOF | kubectl apply -f -
---
apiVersion: v1
kind: Pod
metadata:
  name: vpp-app$number-$worker
  labels:
    name: vpp-app$number
  namespace: vpp
  annotations:
    k8s.v1.cni.cncf.io/networks: userspace-vpp-net-1
    userspace/mappedDir: /var/lib/cni/usrspcni/
spec:
  nodeSelector:
    kubernetes.io/hostname: $worker
  hostname: vpp-app$number-$worker
  subdomain: vpp
  containers:
  - image: ligato/vpp-base
    imagePullPolicy: IfNotPresent
    name: vpp-app$number-$worker
    volumeMounts:
    - name: podinfo
      mountPath: /etc/podinfo
    - name: vpp-startup-config
      mountPath: /etc/vpp/
    - name: hugepage
      mountPath: /hugepages
    - name: shared-dir
      mountPath: /var/lib/cni/usrspcni/
    - name: scripts
      mountPath: /vpp/
    resources:
      requests:
        hugepages-2Mi: 1Gi
        memory: "1Gi"
        cpu: "3"
      limits:
        hugepages-2Mi: 1Gi
        memory: "1Gi"
        cpu: "3"
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
    - name: vpp-startup-config
      configMap:
        name: vpp-startup-config
    - name: hugepage
      emptyDir:
        medium: HugePages
    - name: shared-dir
      hostPath:
        path: /run/vpp/app$number
    - name: scripts
      hostPath:
        path: /runner/_work/userspace-cni-network-plugin/userspace-cni-network-plugin/ci/vpp_test_setup/
EOF
done
