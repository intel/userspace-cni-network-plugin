FROM ubuntu:22.04@sha256:2b7412e6465c3c7fc5bb21d3e6f1917c167358449fecac8176c6e496e5c1f05f
RUN apt-get -q update && apt-get install -y openvswitch-switch-dpdk
RUN update-alternatives --set ovs-vswitchd /usr/lib/openvswitch-switch-dpdk/ovs-vswitchd-dpdk
##RUN sed -i "/rmmod bridge/d" /usr/share/openvswitch/scripts/ovs-kmod-ctl
RUN apt install -y linux-headers-$(uname -r)
CMD /usr/share/openvswitch/scripts/ovs-ctl start && sleep inf
