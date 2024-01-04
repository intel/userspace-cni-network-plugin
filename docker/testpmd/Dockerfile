FROM ubuntu:22.04@sha256:bbf3d1baa208b7649d1d0264ef7d522e1dc0deeeaaf6085bf8e4618867f03494
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