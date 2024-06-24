FROM ubuntu:24.04@sha256:2e863c44b718727c860746568e1d54afd13b2fa71b160f5cd9058fc436217b30
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN apt-get update -y \
    && apt-get install -y linux-headers-generic openvswitch-switch-dpdk \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*
RUN update-alternatives --set ovs-vswitchd /usr/lib/openvswitch-switch-dpdk/ovs-vswitchd-dpdk
# RUN sed -i "/rmmod bridge/d" /usr/share/openvswitch/scripts/ovs-kmod-ctl
RUN apt-get install -y --no-install-recommends linux-headers-"$(uname -r | cut -d'-' -f1)"
CMD ["sh", "-c","/usr/share/openvswitch/scripts/ovs-ctl start && sleep inf"]
