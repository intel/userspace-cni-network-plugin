FROM ubuntu:22.04@sha256:6042500cf4b44023ea1894effe7890666b0c5c7871ed83a97c36c76ae560bb9b
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
USER 10000
