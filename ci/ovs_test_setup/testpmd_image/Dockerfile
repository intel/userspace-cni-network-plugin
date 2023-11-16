FROM ubuntu:22.04@sha256:2b7412e6465c3c7fc5bb21d3e6f1917c167358449fecac8176c6e496e5c1f05f
RUN apt-get -q update && apt-get install -y git vim meson python3-pyelftools libnuma-dev
RUN git clone https://github.com/DPDK/dpdk.git
WORKDIR /dpdk/
RUN meson build
RUN cd build && ninja
WORKDIR /dpdk/build/app
COPY ./testpmd.sh testpmd.sh
CMD ./testpmd.sh
