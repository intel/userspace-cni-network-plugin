#!/bin/bash
set -e

#USERSPACEDIR="/runner/_work/userspace-cni-network-plugin/userspace-cni-network-plugin/"
USERSPACEDIR="/home/runner/work/userspace-cni-network-plugin/userspace-cni-network-plugin"
CI_DIR="$USERSPACEDIR/ci/"

vpp_ligato_latest_container()
{
IMAGE="ligato/vpp-base:latest"

cd ${USERSPACEDIR}

echo "Changing to latest tag in dockerfile"
sed -i "s|\(FROM\).*\(as builder\)|\1 $IMAGE \2|g" ./docker/userspacecni/Dockerfile
grep -n "$IMAGE" ./docker/userspacecni/Dockerfile

echo "Changing to latest tag vpp pod"
sed -i "s|\(image:\).*\(#imagename\)|\1 $IMAGE \2|g" ./ci/vpp_test_setup/vpp_pod.sh
grep -n "$IMAGE" ./ci/vpp_test_setup/vpp_pod.sh

echo "Changing to latest tag vpp host pod"
sed -i "s|\(image:\).*\(#imagename\)|\1 $IMAGE \2|g" ./ci/vpp_test_setup/vpp_host.sh
grep -n "$IMAGE" ./ci/vpp_test_setup/vpp_host.sh
}

install_go_kubectl_kind(){
wget -qO- https://golang.org/dl/go1.20.1.linux-amd64.tar.gz |tar -C "$HOME" -xz
export PATH="${PATH}:${HOME}/go/bin"
echo "export PATH=\"${PATH}:${HOME}/go/bin/:${HOME}.local/bin/\"" >>~/.bashrc
go install sigs.k8s.io/kind@v0.20.0

sudo bash -c 'echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.27/deb/ /" >> /etc/apt/sources.list.d/kubernetes.list'
sudo bash -c 'curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.27/deb/Release.key | gpg --dearmor >> /etc/apt/keyrings/kubernetes-apt-keyring.gpg'
#curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.27/deb/Release.key -o release.key
#sudo bash -c 'gpg --no-tty -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg --dearmor ./release.key'
sudo apt-get update
#sudo apt-get install -y kubectl=1.27.3-1.1
}

create_kind_cluster(){
kubectl_version="v1.27.3"
case "$1" in
	-v | --version ) kubectl_version="$2";
esac
	
kind create cluster --image "kindest/node:$kubectl_version"
kubectl get all --all-namespaces

#docker run -itd --device=/dev/hugepages:/dev/hugepages --privileged -v "$(pwd)/example/sample-vpp-host-config:/etc/vpp/" --name vpp ligato/vpp-base
sleep 10
cd $USERSPACEDIR

make build
# gets path for one directopry above, needed for mkdir with docker cp below
mkdir_var=$(dirname ${USERSPACEDIR})
kind load docker-image localhost:5000/userspacecni
docker exec -i kind-control-plane bash -c "mkdir -p $mkdir_var"
docker cp "${USERSPACEDIR}" "kind-control-plane:${USERSPACEDIR}"
}

deploy_multus(){
## Multus main branch has major bugs so we fix version
cd $CI_DIR
MULTUS_VERSION="v4.0.2"
wget https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/$MULTUS_VERSION/deployments/multus-daemonset.yml
sed -i "s/snapshot-thick/v4.0.2/g" multus-daemonset.yml
kubectl apply -f ./multus-daemonset.yml
}

deploy_userspace(){
cd $USERSPACEDIR
#kubectl label nodes kind-control-plane app=userspace-cni
kubectl label nodes --all app=userspace-cni
make deploy
echo "sleeping for 20 to allow userspace to deploy first"
sleep 20
}

vpp_e2e_test(){
cd $CI_DIR/vpp_test_setup/
echo "Setting up vpp host"
./vpp_host.sh
sleep 20
echo "Setting up vpp pods"
./vpp_pod.sh

sleep 20
kubectl get all -A

kubectl exec -n vpp vpp-app1-kind-control-plane -- ./vpp_pod_setup_memif.sh
kubectl exec -n vpp vpp-app2-kind-control-plane -- ./vpp_pod_setup_memif.sh
kubectl exec -n vpp vpp-app1-kind-control-plane -- vppctl "sh int address"
kubectl exec -n vpp vpp-app2-kind-control-plane -- vppctl "sh int address"

kubectl exec -n vpp vpp-app1-kind-control-plane -- vppctl "ping  192.168.1.4"
if kubectl exec -n vpp vpp-app1-kind-control-plane -- vppctl "ping  192.168.1.4" |grep -q "bytes"; then
	echo "VPP ping test pass"
else 
	echo VPP ping test failed
	exit 1
fi

printf "\n\n Removing vpp app pods \n\n"
kubectl delete -n vpp pod/vpp-app1-kind-control-plane
kubectl delete -n vpp pod/vpp-app2-kind-control-plane
echo "pods deleted"

echo "kubectl get all, app pods should have been removed"
kubectl get all -A

printf "\n vppctl show interface \n\n"
kubectl exec -n vpp pod/vpp-kind-control-plane -- vppctl "sh int"

printf "\n vppctl show memif, only the default socket 0 should be still here \n\n"
kubectl exec -n vpp pod/vpp-kind-control-plane -- vppctl "sh memif"
kubectl delete -n vpp pod/vpp-kind-control-plane
}

build_ovs_container(){
cd $CI_DIR/ovs_test_setup
docker build . -t ovs
kind load docker-image ovs
# set alias for ovs-vsctl and ovs-ofctl in kind container, would be better if we could just install its bin
docker exec -i kind-control-plane bash -c "echo '#!/bin/bash' > /usr/bin/ovs-vsctl"
docker exec -i kind-control-plane bash -c "echo 'export KUBECONFIG=/etc/kubernetes/admin.conf' >> /usr/bin/ovs-vsctl"
docker exec -i kind-control-plane bash -c "echo 'kubectl exec -n ovs ovs-kind-control-plane -- ovs-vsctl \"\$@\"' >> /usr/bin/ovs-vsctl"
docker exec -i kind-control-plane bash -c "chmod +x  /usr/bin/ovs-vsctl"

docker exec -i kind-control-plane bash -c "echo '#!/bin/bash' > /usr/bin/ovs-ofctl"
docker exec -i kind-control-plane bash -c "echo 'export KUBECONFIG=/etc/kubernetes/admin.conf' >> /usr/bin/ovs-ofctl"
docker exec -i kind-control-plane bash -c "echo 'kubectl exec -n ovs ovs-kind-control-plane -- ovs-ofctl \"\$@\"' >> /usr/bin/ovs-ofctl"
docker exec -i kind-control-plane bash -c "chmod +x  /usr/bin/ovs-ofctl"
}

build_testpmd_container(){
cd $CI_DIR/ovs_test_setup/testpmd_image
docker build -t testpmd .
kind load docker-image testpmd
}

ovs_e2e_test(){
cd $CI_DIR/ovs_test_setup
./ovs_host.sh
sleep 20

# workaround, cant create in dockerfile
kubectl exec -n ovs pod/ovs-kind-control-plane -- bash -c "mkdir -p /dev/net/"
kubectl exec -n ovs pod/ovs-kind-control-plane -- bash -c "mknod /dev/net/tun c 10 200"
kubectl exec -n ovs pod/ovs-kind-control-plane -- bash -c 'ovs-vsctl set Open_vSwitch . "other_config:dpdk-init=true"'

./testpmd_pod.sh

sleep 20
kubectl get all -A
kubectl logs -n ovs pod/ovs-kind-control-plane
kubectl describe -n ovs pod/ovs-app1-kind-control-plane
kubectl logs -n ovs pod/ovs-app1-kind-control-plane |tail -11
kubectl logs -n ovs pod/ovs-app2-kind-control-plane |tail -11
pps="$(kubectl logs -n ovs pod/ovs-app2-kind-control-plane |tail -11 | grep 'RX-packets'|sed 's/ * / /g' |cut -d ' ' -f 3)"
echo "RX Packets: $pps"

if [ "$pps" -eq "0" ] || [ -z "${pps}" ]; then
   echo "Test Failed: no traffic";
   exit 1;
else
   echo "OVS Test Pass";
fi
}


run_all(){
# theese steps are triggered by the ci by sourcing this script and running the following separately
# it gives much better logging breakdown on github
# the run_all function is only used for manual deployment
install_go_kubectl_kind
create_kind_cluster -v v1.27.3
deploy_multus
deploy_userspace
vpp_e2e_test
build_ovs_container
build_testpmd_container
ovs_e2e_test
}
#run_all
