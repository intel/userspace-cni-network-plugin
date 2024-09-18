FROM ligato/vpp-base:24.02@sha256:2a0d92375b4579ce8ca75fd2563ece8814d89099b5c15ebe6686ee76b3ab7c22 as builder
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
COPY . /root/userspace-cni-network-plugin
WORKDIR /root/userspace-cni-network-plugin
RUN apt-get update -y \
    && DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y binutils bash wget make git \
    && wget -qO- https://golang.org/dl/go1.22.3.linux-amd64.tar.gz | tar -C /usr/local -xz \
    && rm -rf /var/lib/apt/lists/*
ENV PATH="${PATH}:/usr/local/go/bin"
RUN go mod download \
    && go get go.fd.io/govpp/binapigen/vppapi@v0.7.0 \
    && make generate \
    && go mod tidy \
    && make generate-bin
#End of builder container
# Copy build userspace cni bin to a small deployer container
FROM alpine:3.20@sha256:b89d9c93e9ed3597455c90a0b88a8bbb5cb7188438f70953fede212a0c4394e0
RUN mkdir -p /root/userspace-cni-network-plugin/userspace
COPY --from=builder /root/userspace-cni-network-plugin/userspace/userspace /root/userspace-cni-network-plugin/userspace/userspace
CMD ["cp", "-rf", "/root/userspace-cni-network-plugin/userspace/userspace", "/opt/cni/bin"]
