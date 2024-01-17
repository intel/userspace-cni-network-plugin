FROM ubuntu:22.04@sha256:ab75ad5f473e7d04598c975c4dd9ee5d23449334425b9cc46eecc10879f19858
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN apt-get update -y \
    && apt-get install -y linux-headers-generic openvswitch-switch-dpdk \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*
RUN update-alternatives --set ovs-vswitchd /usr/lib/openvswitch-switch-dpdk/ovs-vswitchd-dpdk
# RUN sed -i "/rmmod bridge/d" /usr/share/openvswitch/scripts/ovs-kmod-ctl
RUN apt-get install -y --no-install-recommends linux-headers-"$(uname -r | cut -d'-' -f1)"
CMD ["sh", "-c","/usr/share/openvswitch/scripts/ovs-ctl start && sleep inf"]
