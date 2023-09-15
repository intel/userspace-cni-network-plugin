#!/bin/bash
kubectl delete ns ovs
kubectl create ns ovs

worker="kind-control-plane"
cat << EOF | kubectl apply -f -
---
apiVersion: v1
kind: Pod
metadata:
  name: ovs-$worker
  labels:
    name: ovs
  namespace: ovs
spec:
  hostNetwork: false
  nodeSelector:
    kubernetes.io/hostname: $worker
  hostname: ovs-$worker
  subdomain: ovs
  containers:
  - image: ovs
    imagePullPolicy: IfNotPresent
    name: ovs-$worker
    volumeMounts:
    - name: vpp-api
      mountPath: /run/openvswitch
    - name: modules
      mountPath: /lib/modules
    - name: vpp-run
      mountPath: /var/run/openvswitch
    - name: hugepage
      mountPath: /hugepages
    resources:
      requests:
        hugepages-2Mi: 1Gi
        memory: "1Gi"
        cpu: "10"
      limits:
        hugepages-2Mi: 1Gi
        memory: "1Gi"
        cpu: "10"
    securityContext:
      capabilities:
        add: ["NET_ADMIN", "SYS_TIME"]
  restartPolicy: Always
  volumes:
    - name: vpp-run
      hostPath:
        path: /var/run/openvswitch/
    - name: modules
      hostPath:
        path: /lib/modules
    - name: vpp-api
      hostPath:
        path: /run/openvswitch/
    - name: userspace-api
      hostPath:
        path: /var/lib/cni/usrspcni/
    - name: hugepage
      emptyDir:
        medium: HugePages
EOF

