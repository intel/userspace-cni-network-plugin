# To build:
#  docker build --rm -t dpdk-app-centos .
#


# -------- Builder stage.
FROM centos
MAINTAINER Billy McFall <bmcfall@redhat.com>

# Install required packages
RUN rpm --import https://mirror.go-repo.io/centos/RPM-GPG-KEY-GO-REPO && curl -s https://mirror.go-repo.io/centos/go-repo.repo | tee /etc/yum.repos.d/go-repo.repo
RUN yum groupinstall -y "Development Tools"
RUN yum install -y wget numactl-devel git golang make; yum clean all

# Download DPDK
ENV DPDK_VER 19.02
ENV DPDK_DIR /usr/src/dpdk-${DPDK_VER}
WORKDIR /usr/src/
RUN wget http://fast.dpdk.org/rel/dpdk-${DPDK_VER}.tar.xz
RUN tar -xpvf dpdk-${DPDK_VER}.tar.xz

ENV RTE_TARGET=x86_64-native-linuxapp-gcc
ENV RTE_SDK=${DPDK_DIR}
WORKDIR ${DPDK_DIR}
RUN sed -i -e 's/EAL_IGB_UIO=y/EAL_IGB_UIO=n/' config/common_linuxapp
RUN sed -i -e 's/KNI_KMOD=y/KNI_KMOD=n/' config/common_linuxapp
RUN sed -i -e 's/LIBRTE_KNI=y/LIBRTE_KNI=n/' config/common_linuxapp
RUN sed -i -e 's/LIBRTE_PMD_KNI=y/LIBRTE_PMD_KNI=n/' config/common_linuxapp
RUN make install T=${RTE_TARGET} DESTDIR=${RTE_SDK}

#
# Download and Build Network Utility
#

#
# Build TestPmd
#
WORKDIR ${DPDK_DIR}/app/test-pmd
COPY ./dpdk-app.c ./testpmd.c
COPY ./dpdk-args.c ./dpdk-args.c
RUN sed -i -e 's/SRCS-y += parameters.c/SRCS-y += parameters.c dpdk-args.c/' Makefile
RUN make
RUN cp testpmd /usr/bin/dpdk-app

# -------- Import stage.
# Docker 17.05 or higher
##FROM centos

# Install UserSpace CNI
##COPY --from=0 /usr/bin/dpdk-app /usr/bin/dpdk-app

COPY ./docker-entrypoint.sh /
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["dpdk-app"]
