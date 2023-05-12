SUDO?=sudo
OS_ID        = $(shell grep '^ID=' /etc/os-release | cut -f2- -d= | sed -e 's/\"//g')
OS_VERSION_ID= $(shell grep '^VERSION_ID=' /etc/os-release | cut -f2- -d= | sed -e 's/\"//g')

ifeq ($(filter ubuntu debian,$(OS_ID)),$(OS_ID))
	PKG=deb
else ifeq ($(filter rhel centos fedora opensuse opensuse-leap opensuse-tumbleweed,$(OS_ID)),$(OS_ID))
	PKG=rpm
endif

#
# Unit test specific variables
#
UT_IMAGE=userspace_cni_plugin
UT_GO_VERSION=1.14.3
UT_OS_CENTOS=centos7 centos8
UT_OS_FEDORA=fedora31 fedora32
UT_OS_UBUNTU=ubuntu16.04 ubuntu18.04 ubuntu20.04
UT_OS_DEFAULT=ubuntu20.04
UT_OS_ALL=$(UT_OS_CENTOS) $(UT_OS_FEDORA) $(UT_OS_UBUNTU)
TEST_TARGETS=$(addprefix test-,$(UT_OS_ALL))
TEST_BUILD_TARGETS=$(addprefix test-build-,$(UT_OS_ALL))
COVERAGE_TARGETS=$(addprefix coverage-,$(UT_OS_ALL))

# Get hash of recent commit for IMAGE tagging
GIT_HASH=$(shell git rev-parse --short HEAD)

# Fail in case that unsupported UT_OS is set
ifeq ($(filter $(UT_OS), $(UT_OS_ALL)),)
ifneq ($(UT_OS),)
$(warning Unsupported unit test OS was selected: UT_OS=$(UT_OS))
$(error Supported values are: $(UT_OS_ALL))
endif
UT_OS=$(UT_OS_DEFAULT)
endif

#
# VPP Variables
#
VPPGPG=ge4a0f9f~b72
VPPMAJOR=19
VPPMINOR=04
VPPDOTRL=1

VPPVERSION=$(VPPMAJOR)$(VPPMINOR)
VPPDOTVERSION=$(VPPMAJOR).$(VPPMINOR).$(VPPDOTRL)
ifeq ($(VPPDOTRL),0)
	VPPDOTVERSION=$(VPPMAJOR).$(VPPMINOR)
else
	VVPPDOTVERSION=$(VPPMAJOR).$(VPPMINOR).$(VPPDOTRL)
endif

ifeq ($(PKG),rpm)
	VPPLIBDIR=/usr/lib64
else ifeq ($(PKG),deb)
	VPPLIBDIR=/usr/lib/x86_64-linux-gnu
endif

# Building the cnivpp subfolder requires VPP to be installed, or at least a
# handful of files in the proper installed location. VPPINSTALLED indicates
# if required VPP files are installed.
# For 'make clean', VPPLCLINSTALLED indicates if 'make install' installed
# the minimum set of files or if VPP is actually installed.
ifeq ($(shell test -e $(VPPLIBDIR)/libvppapiclient.so && echo -n yes),yes)
	VPPINSTALLED=1
ifeq ($(shell test -e /usr/bin/vpp && echo -n yes),yes)
	VPPLCLINSTALLED=0
else
	VPPLCLINSTALLED=1
endif
else
	VPPINSTALLED=0
	VPPLCLINSTALLED=0
endif


# Default to build
default: build
all: build


help:
	@echo "Make Targets:"
	@echo " make                 - Build UserSpace CNI."
	@echo " make clean           - Cleanup all build artifacts. Will remove VPP files installed from *make install*."
	@echo " make install         - If VPP is not installed, install the minimum set of files to build."
	@echo "                        CNI-VPP will fail because VPP is still not installed."
	@echo " make install-dep     - Install software dependencies, currently only needed for *make install*."
	@echo " make extras          - Build *usrsp-app*, small binary to run in Docker container for testing."
	@echo " make test-app        - Build test code."
	@echo ""
	@echo "Make Targets for unit testing inside containers:"
	@echo " make test-clean      - Remove test container images and generated Dockerfiles."
	@echo " make test-build      - Build container image for unit tests with OS defined by UT_OS: UT_OS="$(UT_OS)
	@echo " make test            - Run unit tests inside container with OS defined by UT_OS: UT_OS="$(UT_OS)
	@echo " make coverage        - Calculate code coverage in container with OS defined by UT_OS: UT_OS="$(UT_OS)
	@echo " make test-build-<os> - Build container image for unit tests with <os>, e.g. make test-build-centos8"
	@echo " make test-<os>       - Run unit tests inside container with <os>, e.g. make test-centos8"
	@echo " make coverage-<os>   - Calculate code coverage inside container with <os>, e.g. make coverage-centos8"
	@echo " make test-build-all  - Build container images for unit tests for all supported OS distributions"
	@echo "                        e.g. make -j 5 test-build-all"
	@echo " make test-all        - Run unit tests inside container for all supported OS distributions"
	@echo "                        e.g. make -j 5 test-all"
	@echo " make coverage-all    - Calculate code coverage inside container for all supported OS distributions."
	@echo "                        e.g. make -j 5 coverage-all"
	@echo ""
	@echo " Supported OS distributions for unit testing are: $(UT_OS_ALL)"
	@echo ""
#	@echo "Makefile variables (debug):"
#	@echo "   SUDO=$(SUDO) OS_ID=$(OS_ID) OS_VERSION_ID=$(OS_VERSION_ID) PKG=$(PKG) VPPVERSION=$(VPPVERSION) $(VPPDOTVERSION)"
#	@echo "   VPPLIBDIR=$(VPPLIBDIR)"
#	@echo "   VPPINSTALLED=$(VPPINSTALLED) VPPLCLINSTALLED=$(VPPLCLINSTALLED)"
#	@echo ""

build: generate
	@cd userspace && go build -v

test-app:
	@cd cnivpp/test/memifAddDel && go build -v
	@cd cnivpp/test/vhostUserAddDel && go build -v
	@cd cnivpp/test/ipAddDel && go build -v

install-dep:
ifeq ($(VPPINSTALLED),0)
ifeq ($(PKG),rpm)
	@$(SUDO) -E yum install -y wget cpio rpm
else ifeq ($(PKG),deb)
	@$(SUDO) -E apt-get install -y binutils wget
endif
endif

install:
	go get go.fd.io/govpp/cmd/binapi-generator@v0.3.5
ifeq ($(VPPINSTALLED),0)
	@echo VPP not installed, installing required files. Run *sudo make clean* to remove installed files.
	@mkdir -p tmpvpp/
ifeq ($(PKG),rpm)
	@cd tmpvpp && wget --content-disposition https://packagecloud.io/fdio/$(VPPVERSION)/packages/el/7/vpp-lib-$(VPPDOTVERSION)-1~$(VPPGPG).x86_64.rpm/download.rpm
	@cd tmpvpp && wget --content-disposition https://packagecloud.io/fdio/$(VPPVERSION)/packages/el/7/vpp-devel-$(VPPDOTVERSION)-1~$(VPPGPG).x86_64.rpm/download.rpm
	@cd tmpvpp && rpm2cpio ./vpp-devel-$(VPPDOTVERSION)-1~$(VPPGPG).x86_64.rpm | cpio -ivd \
		./usr/include/vpp-api/client/vppapiclient.h
	@cd tmpvpp && rpm2cpio ./vpp-lib-$(VPPDOTVERSION)-1~$(VPPGPG).x86_64.rpm | cpio -ivd \
		./usr/lib64/libsvm.so.$(VPPDOTVERSION) \
		./usr/lib64/libvlibmemoryclient.so.$(VPPDOTVERSION) \
		./usr/lib64/libvppapiclient.so.$(VPPDOTVERSION) \
		./usr/lib64/libvppinfra.so.$(VPPDOTVERSION)
	@cd tmpvpp && rpm2cpio ./vpp-lib-$(VPPDOTVERSION)-1~$(VPPGPG).x86_64.rpm | cpio -ivd \
		./usr/share/vpp/api/interface.api.json \
		./usr/share/vpp/api/l2.api.json \
		./usr/share/vpp/api/memif.api.json \
		./usr/share/vpp/api/vhost_user.api.json \
		./usr/share/vpp/api/vpe.api.json
else ifeq ($(PKG),deb)
	@cd tmpvpp && wget --content-disposition https://packagecloud.io/fdio/release/packages/ubuntu/xenial/vpp_$(VPPDOTVERSION)-release_amd64.deb/download.deb
	@cd tmpvpp && wget --content-disposition https://packagecloud.io/fdio/release/packages/ubuntu/xenial/vpp-dev_$(VPPDOTVERSION)-release_amd64.deb/download.deb
	@cd tmpvpp && wget --content-disposition https://packagecloud.io/fdio/release/packages/ubuntu/xenial/libvppinfra_$(VPPDOTVERSION)-release_amd64.deb/download.deb
	@cd tmpvpp && wget --content-disposition https://packagecloud.io/fdio/release/packages/ubuntu/xenial/vpp-plugin-core_$(VPPDOTVERSION)-release_amd64.deb/download.deb
	@cd tmpvpp && dpkg-deb --fsys-tarfile vpp-dev_$(VPPDOTVERSION)-release_amd64.deb | tar -x \
		./usr/include/vpp-api/client/vppapiclient.h
	@cd tmpvpp && dpkg-deb --fsys-tarfile libvppinfra_$(VPPDOTVERSION)-release_amd64.deb | tar -x \
		./usr/lib/x86_64-linux-gnu/libvppinfra.so.$(VPPDOTVERSION)
	@cd tmpvpp && dpkg-deb --fsys-tarfile vpp_$(VPPDOTVERSION)-release_amd64.deb | tar -x \
		./usr/share/vpp/api/core/interface.api.json \
		./usr/share/vpp/api/core/l2.api.json \
		./usr/share/vpp/api/core/vhost_user.api.json \
		./usr/share/vpp/api/core/vpe.api.json \
		./usr/lib/x86_64-linux-gnu/libsvm.so.$(VPPDOTVERSION) \
		./usr/lib/x86_64-linux-gnu/libvlibmemoryclient.so.$(VPPDOTVERSION) \
		./usr/lib/x86_64-linux-gnu/libvppapiclient.so.$(VPPDOTVERSION)
	@cd tmpvpp && dpkg-deb --fsys-tarfile vpp-plugin-core_$(VPPDOTVERSION)-release_amd64.deb | tar -x \
		./usr/share/vpp/api/plugins/memif.api.json
endif
	@$(SUDO) -E mkdir -p /usr/include/vpp-api/client/
	@$(SUDO) -E cp tmpvpp/usr/include/vpp-api/client/vppapiclient.h /usr/include/vpp-api/client/.
	@$(SUDO) -E chown -R bin:bin /usr/include/vpp-api/
	@echo   Installed /usr/include/vpp-api/client/vppapiclient.h
	@$(SUDO) -E cp tmpvpp$(VPPLIBDIR)/libsvm.so.$(VPPDOTVERSION) $(VPPLIBDIR)/.
	@$(SUDO) -E cp tmpvpp$(VPPLIBDIR)/libvlibmemoryclient.so.$(VPPDOTVERSION) $(VPPLIBDIR)/.
	@$(SUDO) -E cp tmpvpp$(VPPLIBDIR)/libvppapiclient.so.$(VPPDOTVERSION) $(VPPLIBDIR)/.
	@$(SUDO) -E cp tmpvpp$(VPPLIBDIR)/libvppinfra.so.$(VPPDOTVERSION) $(VPPLIBDIR)/.
ifneq ($(VPPDOTRL),0)
	@$(SUDO) -E ln -s $(VPPLIBDIR)/libsvm.so.$(VPPDOTVERSION) $(VPPLIBDIR)/libsvm.so.$(VPPMAJOR).$(VPPMINOR)
	@$(SUDO) -E ln -s $(VPPLIBDIR)/libvlibmemoryclient.so.$(VPPDOTVERSION) $(VPPLIBDIR)/libvlibmemoryclient.so.$(VPPMAJOR).$(VPPMINOR)
	@$(SUDO) -E ln -s $(VPPLIBDIR)/libvppapiclient.so.$(VPPDOTVERSION) $(VPPLIBDIR)/libvppapiclient.so.$(VPPMAJOR).$(VPPMINOR)
	@$(SUDO) -E ln -s $(VPPLIBDIR)/libvppinfra.so.$(VPPDOTVERSION) $(VPPLIBDIR)/libvppinfra.so.$(VPPMAJOR).$(VPPMINOR)
endif
	@$(SUDO) -E ln -s $(VPPLIBDIR)/libsvm.so.$(VPPDOTVERSION) $(VPPLIBDIR)/libsvm.so.$(VPPMAJOR)
	@$(SUDO) -E ln -s $(VPPLIBDIR)/libvlibmemoryclient.so.$(VPPDOTVERSION) $(VPPLIBDIR)/libvlibmemoryclient.so.$(VPPMAJOR)
	@$(SUDO) -E ln -s $(VPPLIBDIR)/libvppapiclient.so.$(VPPDOTVERSION) $(VPPLIBDIR)/libvppapiclient.so.$(VPPMAJOR)
	@$(SUDO) -E ln -s $(VPPLIBDIR)/libvppinfra.so.$(VPPDOTVERSION) $(VPPLIBDIR)/libvppinfra.so.$(VPPMAJOR)
	@$(SUDO) -E ln -s $(VPPLIBDIR)/libsvm.so.$(VPPDOTVERSION) $(VPPLIBDIR)/libsvm.so
	@$(SUDO) -E ln -s $(VPPLIBDIR)/libvlibmemoryclient.so.$(VPPDOTVERSION) $(VPPLIBDIR)/libvlibmemoryclient.so
	@$(SUDO) -E ln -s $(VPPLIBDIR)/libvppapiclient.so.$(VPPDOTVERSION) $(VPPLIBDIR)/libvppapiclient.so
	@$(SUDO) -E ln -s $(VPPLIBDIR)/libvppinfra.so.$(VPPDOTVERSION) $(VPPLIBDIR)/libvppinfra.so
	@$(SUDO) -E chown -R bin:bin $(VPPLIBDIR)/libsvm.so*
	@$(SUDO) -E chown -R bin:bin $(VPPLIBDIR)/libvlibmemoryclient.so*
	@$(SUDO) -E chown -R bin:bin $(VPPLIBDIR)/libvppapiclient.so*
	@$(SUDO) -E chown -R bin:bin $(VPPLIBDIR)/libvppinfra.so*
	@echo   Installed  $(VPPLIBDIR)/libsvm.so $(VPPLIBDIR)/libvlibmemoryclient.so $(VPPLIBDIR)/libvppapiclient.so $(VPPLIBDIR)/libvppinfra.so
	@$(SUDO) -E mkdir -p /usr/share/vpp/api/
ifeq ($(PKG),rpm)
	@$(SUDO) -E cp tmpvpp/usr/share/vpp/api/*.json /usr/share/vpp/api/.
else ifeq ($(PKG),deb)
	@$(SUDO) -E cp tmpvpp/usr/share/vpp/api/core/*.json /usr/share/vpp/api/.
	@$(SUDO) -E cp tmpvpp/usr/share/vpp/api/plugins/*.json /usr/share/vpp/api/.
endif
	@$(SUDO) -E chown -R bin:bin /usr/share/vpp/
	@echo   Installed /usr/share/vpp/api/*.json
	@rm -rf tmpvpp
endif


extras:
	@cd docker/usrsp-app && go build -v

clean: test-clean
	@rm -f docker/usrsp-app/usrsp-app
	@rm -f cnivpp/test/memifAddDel/memifAddDel
	@rm -f cnivpp/test/vhostUserAddDel/vhostUserAddDel
	@rm -f cnivpp/test/ipAddDel/ipAddDel
	@rm -rf cnivpp/bin_api
	@rm -f userspace/userspace
ifeq ($(VPPLCLINSTALLED),1)
	@echo VPP was installed by *make install*, so cleaning up files.
	@$(SUDO) -E rm -rf /usr/include/vpp-api/
	@$(SUDO) -E rm $(VPPLIBDIR)/libsvm.so*
	@$(SUDO) -E rm $(VPPLIBDIR)/libvlibmemoryclient.so*
	@$(SUDO) -E rm $(VPPLIBDIR)/libvppapiclient.so*
	@$(SUDO) -E rm $(VPPLIBDIR)/libvppinfra.so*
	@$(SUDO) -E rm -rf /usr/share/vpp/

endif

generate:
ifeq ($(VPPINSTALLED),0)
	@echo VPP not installed. Run *make install* to install the minimum set of files to compile, or install VPP.
	@echo
endif
	for package in cnivpp/api/* ; do cd $$package ; pwd ; go generate ; cd - ; done

lint:

check-test-dep:
	@for TEST_DEP in docker cpp git ; do \
		if ! which $$TEST_DEP > /dev/null ; then \
			echo "$$TEST_DEP is required for unit test execution, please install it."; \
			exit 1; \
		fi \
	done

$(TEST_BUILD_TARGETS): test-build-%: check-test-dep
	@# Skip image build in case that image with recent code changes exists
	@if [ "`docker images -q $(UT_IMAGE):$*_$(GIT_HASH)`" = "" ] ; then \
		echo Build unit test image for $*; \
		cpp -o docker/unit-tests/Dockerfile.$* docker/unit-tests/Dockerfile.$*.in; \
		docker build . \
			-t $(UT_IMAGE):$* \
			-t $(UT_IMAGE):$*_$(GIT_HASH) \
			--build-arg UT_GO_VERSION=$(UT_GO_VERSION) \
			-f docker/unit-tests/Dockerfile.$*; \
	fi

test-build: test-build-$(UT_OS)

test-build-all: $(TEST_BUILD_TARGETS)

$(TEST_TARGETS): test-%: test-build-%
	@echo Run unit tests at $*
	@$(SUDO) docker run --rm --privileged $(UT_IMAGE):$*_$(GIT_HASH) bash -c \
		'UT_LIST=`find . -name "*_test.go" -not -path "./vendor/*" -exec dirname \{\} \; | sort -u`; \
		for UT_DIR in $$UT_LIST ; do \
			cd $${UT_DIR}; \
			go test -v || exit 1; \
			cd -; \
		done'

test: test-$(UT_OS)

test-all: $(TEST_TARGETS)

$(COVERAGE_TARGETS): coverage-%: test-build-%
	@echo Calculate code coverage at $*
	@$(SUDO) docker run --rm --privileged $(UT_IMAGE):$* bash -c "go-carpet -summary | sort"

coverage: coverage-$(UT_OS)

coverage-all: $(COVERAGE_TARGETS)

test-clean:
	@if which docker > /dev/null ; then \
		echo Remove unit test container images with name $(UT_IMAGE); \
		for IMAGE in `docker images -a -q --filter=reference=$(UT_IMAGE) | sort -u`; do \
			docker rmi -f $$IMAGE; \
		done; \
		echo Remove generated dockerfiles; \
		for IMAGEOS in $(UT_OS_ALL) ; do \
			rm -f docker/unit-tests/Dockerfile.$$IMAGEOS; \
		done; \
	else \
		echo "Docker is not installed, nothing to delete."; \
	fi

.PHONY: build test-app install extras clean generate check-test-dep test-clean test-build test-build-all \
	test test-all coverage coverage-all $(TEST_TARGETS) $(TEST_BUILD_TARGETS) $(COVERAGE_TARGETS)
