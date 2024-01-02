IMAGE_REGISTRY?=localhost:5000/
IMAGE_VERSION?=latest
IMAGE_BUILDER?=docker

IMAGE_NAME?=$(IMAGE_REGISTRY)userspacecni:$(IMAGE_VERSION)

default: build
local: build copy
all: build push deploy

help:
	@echo "Make Targets:"
	@echo "make build                 - Build UserSpace CNI container."
	@echo "make copy                  - Copy binary from container to host:/opt/cni/bin."
	@echo "make local                 - build and copy"
	@echo "make deploy                - kubectl apply daemonset"
	@echo "make undeploy              - kubectl delete daemonset"
	@echo "make all                   - build push and deploy to kubernetes"

build: 
	@$(IMAGE_BUILDER) build . -f ./docker/userspacecni/Dockerfile -t $(IMAGE_NAME)

push:
	@$(IMAGE_BUILDER) push $(IMAGE_NAME)

copy:
	# Copying the ovs binary to host /opt/cni/bin/
	@mkdir -p /opt/cni/bin/
	@$(IMAGE_BUILDER) run -it --rm -v /opt/cni/bin/:/opt/cni/bin/ $(IMAGE_NAME)

generate-bin: generate
	# Used in dockerfile
	@cd userspace && go build -v

generate:
	# Used in dockerfile
	@for package in cnivpp/api/* ; do cd $$package ; pwd ; go generate ; cd - ; done

deploy:
	# Use sed to replace image name and then apply deployment file
	@sed "s|\(image:\).*\(#registory\)|\1 $(IMAGE_NAME) \2|g" ./kubernetes/userspace-daemonset.yml |kubectl apply -f -

undeploy:
	kubectl delete -f ./kubernetes/userspace-daemonset.yml

testpmd:
	@$(IMAGE_BUILDER) build -t $(IMAGE_REGISTRY)testpmd:latest -f ./docker/testpmd/Dockerfile ./docker/testpmd/
	@$(IMAGE_BUILDER) push $(IMAGE_REGISTRY)testpmd:latest

build-test-container:
	@$(IMAGE_BUILDER) rm -f userspacecni-unittest
	@$(IMAGE_BUILDER) build . -f ./docker/userspacecni/Dockerfile.unittest -t userspacecni-unittest:latest
	@$(IMAGE_BUILDER) run -m 100g --privileged -v ./examples/sample-vpp-host-config/startup.conf:/etc/vpp/startup.conf --name userspacecni-unittest -itd userspacecni-unittest:latest
	@$(IMAGE_BUILDER) cp userspacecni-unittest:/root/userspace-cni-network-plugin/cnivpp ./

unit-test: build-test-container
	@$(IMAGE_BUILDER) exec userspacecni-unittest bash -c "go test ./cnivpp/ -v -cover"
	@$(IMAGE_BUILDER) rm -f userspacecni-unittest

fuzz: build-test-container
	@$(IMAGE_BUILDER) exec userspacecni-unittest bash -c "cd ./cnivpp/test; go test -fuzz=FuzzLoadNetConf -v -fuzztime 120s"
	@$(IMAGE_BUILDER) rm -f userspacecni-unittest
