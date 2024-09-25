FROM ligato/vpp-base:24.02@sha256:daa54ffefce805a2da087f5577d5a6644d41ab3748bde3d9a2c39b0f507019f5 as builder
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
FROM alpine:3.20.3@sha256:a8f120106f5549715aa966fd7cefaf3b7045f6414fed428684de62fec8c2ca4b
RUN mkdir -p /root/userspace-cni-network-plugin/userspace
COPY --from=builder /root/userspace-cni-network-plugin/userspace/userspace /root/userspace-cni-network-plugin/userspace/userspace
CMD ["cp", "-rf", "/root/userspace-cni-network-plugin/userspace/userspace", "/opt/cni/bin"]
