IMAGE_REGISTRY?=localhost:5000/
IMAGE_VERSION?=latest
IMAGE_BUILDER?=docker

IMAGE_NAME?=$(IMAGE_REGISTRY)userspacecni:$(IMAGE_VERSION)

default: build
all: build deploy


help:
	@echo "Make Targets:"
	@echo "make build                 - Build UserSpace CNI container."
	@echo "make deploy                - Copy binary from container to host: /opt/cni/bin."
	@echo "make all                   - build and deploy"

build: 
	@$(IMAGE_BUILDER) build . -f ./docker/userspacecni/Dockerfile -t $(IMAGE_NAME)

deploy:
	# Copying the ovs binary to host /opt/cni/bin/
	@mkdir -p /opt/cni/bin/
	@$(IMAGE_BUILDER) run -it --rm -v /opt/cni/bin/:/opt/cni/bin/ $(IMAGE_NAME)

generate-bin: generate
	@cd userspace && go build -v

generate:
	for package in cnivpp/api/* ; do cd $$package ; pwd ; go generate ; cd - ; done

