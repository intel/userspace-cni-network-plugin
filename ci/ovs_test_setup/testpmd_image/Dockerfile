FROM ubuntu:22.04
RUN apt-get -q update && apt-get install -y git vim meson python3-pyelftools libnuma-dev
RUN git clone https://github.com/DPDK/dpdk.git
WORKDIR /dpdk/
RUN meson build
RUN cd build && ninja
WORKDIR /dpdk/build/app
COPY ./testpmd.sh testpmd.sh
CMD ./testpmd.sh
