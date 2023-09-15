FROM ubuntu:22.04
RUN apt-get -q update && apt-get install -y openvswitch-switch-dpdk
RUN update-alternatives --set ovs-vswitchd /usr/lib/openvswitch-switch-dpdk/ovs-vswitchd-dpdk
##RUN sed -i "/rmmod bridge/d" /usr/share/openvswitch/scripts/ovs-kmod-ctl
RUN apt install -y linux-headers-$(uname -r)
CMD /usr/share/openvswitch/scripts/ovs-ctl start && sleep inf
