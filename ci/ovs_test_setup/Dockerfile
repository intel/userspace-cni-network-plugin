FROM ubuntu:22.04@sha256:6042500cf4b44023ea1894effe7890666b0c5c7871ed83a97c36c76ae560bb9b
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN apt-get update -y \
    && apt-get install -y linux-headers-generic openvswitch-switch-dpdk \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*
RUN update-alternatives --set ovs-vswitchd /usr/lib/openvswitch-switch-dpdk/ovs-vswitchd-dpdk
# RUN sed -i "/rmmod bridge/d" /usr/share/openvswitch/scripts/ovs-kmod-ctl
RUN apt-get install -y --no-install-recommends linux-headers-"$(uname -r | cut -d'-' -f1)"
CMD ["sh", "-c","/usr/share/openvswitch/scripts/ovs-ctl start && sleep inf"]
