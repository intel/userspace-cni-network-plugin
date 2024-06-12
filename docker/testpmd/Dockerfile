FROM ubuntu:24.04@sha256:e3f92abc0967a6c19d0dfa2d55838833e947b9d74edbcb0113e48535ad4be12a
RUN apt-get -q update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        git \
        vim \
        meson \
        python3-pyelftools \
        libnuma-dev \
        python3-pip \
        ninja-build \
        build-essential \
    && rm -rf /var/lib/apt/lists/*
RUN apt-get update -y \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
RUN git clone https://github.com/DPDK/dpdk.git
WORKDIR /dpdk/
RUN meson build
WORKDIR /dpdk/build/ 
RUN ninja
WORKDIR /dpdk/build/app
COPY ./testpmd.sh testpmd.sh
CMD ["sh", "-c","./testpmd.sh"]
