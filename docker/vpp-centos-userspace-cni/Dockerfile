#
# To build:
#  docker build --rm -t vpp-centos-userspace-cni .
#


# -------- Builder stage.
FROM centos:7

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# Install VPP - Needed by CNI-VPP
RUN curl -s https://packagecloud.io/install/repositories/fdio/release/script.rpm.sh | bash
RUN yum install -y epel-release
RUN yum install -y epel-release vpp-plugins vpp-devel vpp-api-python vpp-api-lua; yum clean all

#
# Download and Build Container usrsp-app
#

# Pull in GO
RUN rpm --import https://mirror.go-repo.io/centos/RPM-GPG-KEY-GO-REPO && curl -s https://mirror.go-repo.io/centos/go-repo.repo | tee /etc/yum.repos.d/go-repo.repo
RUN yum install -y git golang make

# Build the usrsp-app
WORKDIR /root/go/src/
RUN go get github.com/intel/userspace-cni-network-plugin > /tmp/UserspaceDockerBuild.log 2>&1 || echo "Can ignore no GO files."
WORKDIR /root/go/src/github.com/intel/userspace-cni-network-plugin
RUN make extras
RUN cp docker/usrsp-app/usrsp-app /usr/sbin/usrsp-app


# -------- Import stage.
# Docker 17.05 or higher, remove ##
##FROM centos

# Install UserSpace CNI
##COPY --from=0 /usr/sbin/usrsp-app /usr/sbin/usrsp-app


# Install VPP
##RUN curl -s https://packagecloud.io/install/repositories/fdio/release/script.rpm.sh | bash
##RUN yum install -y epel-release
##RUN yum install -y vpp-plugins vpp-devel vpp-api-python vpp-api-lua; yum clean all

# Overwrite VPP systemfiles
COPY startup.conf /etc/vpp/startup.conf
COPY 80-vpp.conf /etc/sysctl.d/80-vpp.conf


# Install script to start both VPP and usrsp-app
COPY vppcni.sh vppcni.sh


# Setup VPP UserGroup and User
#RUN useradd --no-log-init -r -g vpp vpp
#USER vpp


# For Development, overwrite repo generated usrsp-app with local development binary.
# Needs to be commented out before each merge.
#COPY usrsp-app /usr/sbin/usrsp-app


CMD ["bash", "-C", "./vppcni.sh"]
#CMD [ "./vppcni.sh" ]
