#
# To build:
#  docker build --rm -t ovs-centos-userspace-cni .
#


# -------- Builder stage.
FROM centos
MAINTAINER Billy McFall <bmcfall@redhat.com>

#
# OvS-DPDK
#
RUN yum groupinstall -y "Development Tools"
RUN yum install -y epel-release
RUN yum install -y numactl-devel wget libpcap-devel python-pip
RUN pip install six

# Download and build DPDK
RUN mkdir -p /root/dpdk/
WORKDIR /root/dpdk/
RUN wget http://fast.dpdk.org/rel/dpdk-17.11.4.tar.xz
RUN tar xf dpdk-17.11.4.tar.xz
WORKDIR /root/dpdk/dpdk-stable-17.11.4
RUN sed -i -e 's/EAL_IGB_UIO=y/EAL_IGB_UIO=n/' config/common_linuxapp
RUN sed -i -e 's/KNI_KMOD=y/KNI_KMOD=n/' config/common_linuxapp
RUN sed -i -e 's/LIBRTE_KNI=y/LIBRTE_KNI=n/' config/common_linuxapp
RUN sed -i -e 's/LIBRTE_PMD_KNI=y/LIBRTE_PMD_KNI=n/' config/common_linuxapp
RUN make install T=x86_64-native-linuxapp-gcc DESTDIR=install

# Download and build OvS
RUN mkdir -p /root/ovs/
WORKDIR /root/ovs/
RUN wget http://openvswitch.org/releases/openvswitch-2.9.2.tar.gz
RUN tar xf openvswitch-2.9.2.tar.gz
WORKDIR /root/ovs/openvswitch-2.9.2/
RUN ./configure --with-dpdk=/root/dpdk/dpdk-stable-17.11.4/x86_64-native-linuxapp-gcc 
RUN make
RUN make install

#
# Download and Build container app
#

# Pull in GO
RUN rpm --import https://mirror.go-repo.io/centos/RPM-GPG-KEY-GO-REPO && curl -s https://mirror.go-repo.io/centos/go-repo.repo | tee /etc/yum.repos.d/go-repo.repo
RUN yum install -y git golang make sudo

# Build the container app
WORKDIR /root/go/src/github.com/intel/
RUN git clone https://github.com/intel/userspace-cni-network-plugin
WORKDIR /root/go/src/github.com/intel/userspace-cni-network-plugin
RUN make install-dep; make install; make extras
RUN cp docker/usrsp-app/usrsp-app /usr/sbin/usrsp-app

# -------- Import stage.
FROM centos

# Install UserSpace CNI
COPY --from=0 /usr/sbin/usrsp-app /usr/sbin/usrsp-app
COPY --from=0 /usr/local/bin/ovs* /usr/local/bin/
COPY --from=0 /usr/local/sbin/ovs* /usr/local/sbin/
COPY --from=0 /usr/local/share/openvswitch/ /usr/local/share/openvswitch/
COPY --from=0 /usr/local/etc/openvswitch/ /usr/local/etc/openvswitch/
COPY --from=0 /usr/lib64/libnuma.so.1 /usr/lib64/libnuma.so.1

# Install OvS
RUN yum install -y centos-release-openstack-queens; yum install -y make sudo git; export PATH=$PATH:/usr/local/share/openvswitch/scripts:/usr/local/bin

# Install VPP libraries
WORKDIR /root/go/src/github.com/intel/
RUN git clone https://github.com/intel/userspace-cni-network-plugin
WORKDIR /root/go/src/github.com/intel/userspace-cni-network-plugin
RUN make install-dep; make install
WORKDIR /root/
RUN yum clean all

# Install script to start both OvS and usrsp-app
COPY ovscni.sh ovscni.sh


# For Development, overwrite repo generated usrsp-app with local development binary.
# Needs to be commented out before each merge.
#COPY usrsp-app /usr/sbin/usrsp-app


CMD bash -C './ovscni.sh';'bash'
#CMD [ "./ovscni.sh" ]
