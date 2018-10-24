#
# To build:
#  docker build --rm -t vpp-centos-userspace-cni .
#


# -------- Builder stage.
FROM centos
MAINTAINER Billy McFall <bmcfall@redhat.com>

# Add any additional repos to image
COPY *.repo /etc/yum.repos.d/
# Install VPP - Needed by CNI-VPP
RUN yum install -y vpp-plugins vpp-devel vpp-api-python vpp-api-lua vpp-api-java; yum clean all

#
# Download and Build Container usrsp-app
#

# Pull in GO
RUN rpm --import https://mirror.go-repo.io/centos/RPM-GPG-KEY-GO-REPO && curl -s https://mirror.go-repo.io/centos/go-repo.repo | tee /etc/yum.repos.d/go-repo.repo
RUN yum install -y git golang make

# Build the usrsp-app
WORKDIR /root/go/src/github.com/intel/
RUN git clone https://github.com/intel/userspace-cni-network-plugin
WORKDIR /root/go/src/github.com/intel/userspace-cni-network-plugin
RUN make extras
RUN cp docker/usrsp-app/usrsp-app /usr/sbin/usrsp-app


# -------- Import stage.
FROM centos

# Install UserSpace CNI
COPY --from=0 /usr/sbin/usrsp-app /usr/sbin/usrsp-app


# Add any additional repos to image
COPY *.repo /etc/yum.repos.d/


# Install VPP
RUN yum install -y vpp-plugins vpp-devel vpp-api-python vpp-api-lua vpp-api-java; yum clean all

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


CMD bash -C './vppcni.sh';'bash'
#CMD [ "./vppcni.sh" ]
