FROM ubuntu:22.04@sha256:e6173d4dc55e76b87c4af8db8821b1feae4146dd47341e4d431118c7dd060a74
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
        && pip3 install --no-cache-dir --upgrade meson==1.3.0 \
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
