#
# To build:
#  docker build --rm -t ovs-centos-userspace-cni .
#


# -------- Builder stage.
FROM centos
MAINTAINER Billy McFall <bmcfall@redhat.com>

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

# Install OvS
RUN yum install -y centos-release-openstack-queens; yum install -y openvswitch openvswitch-devel sudo git; export PATH=$PATH:/usr/share/openvswitch/scripts

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
