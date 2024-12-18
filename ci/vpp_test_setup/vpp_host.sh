#!/bin/bash

# Set USERSPACEDIR if not defined in parent script
USERSPACEDIR="${USERSPACEDIR:=/home/runner/work/userspace-cni-network-plugin/userspace-cni-network-plugin/}"
#USERSPACEDIR="${USERSPACEDIR:=/runner/_work/userspace-cni-network-plugin/userspace-cni-network-plugin/}"

kubectl delete ns vpp
kubectl create ns vpp
kubectl create -n vpp configmap vpp-startup-config --from-file="${USERSPACEDIR}/examples/sample-vpp-host-config/startup.conf"

worker="kind-control-plane"

docker exec -i kind-control-plane bash -c "mkdir -p /var/run/vpp/app"

cat << EOF | kubectl apply -f -
---
apiVersion: v1
kind: Pod
metadata:
  name: vpp-$worker
  labels:
    name: vpp
  namespace: vpp
spec:
  nodeSelector:
    kubernetes.io/hostname: $worker
  hostname: vpp-$worker
  subdomain: vpp
  containers:
  - image: ligato/vpp-base:23.02 #imagename
    imagePullPolicy: IfNotPresent
    name: vpp-$worker
    volumeMounts:
    - name: vpp-api
      mountPath: /run/vpp/
    - name: vpp-run
      mountPath: /var/run/vpp/
    - name: vpp-startup-config
      mountPath: /etc/vpp/
    - name: hugepage
      mountPath: /hugepages
    - name: userspace-api
      mountPath: /var/lib/cni/usrspcni/
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
    - name: vpp-run
      hostPath:
        path: /var/run/vpp/
    - name: vpp-api
      hostPath:
        path: /run/vpp/
    - name: userspace-api
      hostPath:
        path: /var/lib/cni/usrspcni/
    - name: vpp-startup-config
      configMap:
        name: vpp-startup-config
    - name: hugepage
      emptyDir:
        medium: HugePages
EOF

