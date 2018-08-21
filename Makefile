SUDO?=sudo
OS_ID        = $(shell grep '^ID=' /etc/os-release | cut -f2- -d= | sed -e 's/\"//g')
OS_VERSION_ID= $(shell grep '^VERSION_ID=' /etc/os-release | cut -f2- -d= | sed -e 's/\"//g')

ifeq ($(filter ubuntu debian,$(OS_ID)),$(OS_ID))
	PKG=deb
else ifeq ($(filter rhel centos fedora opensuse opensuse-leap opensuse-tumbleweed,$(OS_ID)),$(OS_ID))
	PKG=rpm
endif


#
# VPP Variables
#

VPPVERSION=1804
VPPDOTVERSION=18.04

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

#
# OVS Variables
#
ifeq ($(shell test -e /usr/share/openvswitch/scripts/ovs-config.py && echo -n yes),yes)
        OVS_PY_INSTALLED=1
else
        OVS_PY_INSTALLED=0
endif


# Default to build
default: build
all: build


help:
	@echo "Make Targets:"
	@echo " make                - Build UserSpace CNI."
	@echo " make clean          - Cleanup all build artifacts. Will remove VPP files installed from *make install*."
	@echo " make install        - If VPP is not installed, install the minimum set of files to build."
	@echo "                       CNI-VPP will fail because VPP is still not installed. Also install OvS Python Script."
	@echo " make install-dep    - Install software dependencies, currently only needed for *make install*."
	@echo " make extras         - Build *vpp-app*, small binary to run in Docker container for testing."
	@echo " make test           - Build test code."
	@echo ""
	@echo "Other:"
	@echo " glide update --strip-vendor - Recalculate dependancies and update *vendor\* with proper packages."
	@echo ""
#	@echo "Makefile variables (debug):"
#	@echo "   SUDO=$(SUDO) OS_ID=$(OS_ID) OS_VERSION_ID=$(OS_VERSION_ID) PKG=$(PKG) VPPVERSION=$(VPPVERSION) $(VPPDOTVERSION)"
#	@echo "   VPPLIBDIR=$(VPPLIBDIR)"
#	@echo "   VPPINSTALLED=$(VPPINSTALLED) VPPLCLINSTALLED=$(VPPLCLINSTALLED)"
#	@echo ""

build:
ifeq ($(VPPINSTALLED),0)
	@echo VPP not installed. Run *make install* to install the minimum set of files to compile, or install VPP.
	@echo
endif
	@cd vendor/git.fd.io/govpp.git/cmd/binapi-generator && go build -v
	@./vendor/git.fd.io/govpp.git/cmd/binapi-generator/binapi-generator \
		--input-dir=/usr/share/vpp/api/ \
		--output-dir=vendor/git.fd.io/govpp.git/core/bin_api/
	@cd userspace && go build -v

test:
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
ifeq ($(VPPINSTALLED),0)
	@echo VPP not installed, installing required files. Run *sudo make clean* to remove installed files.
	@mkdir -p tmpvpp/
ifeq ($(PKG),rpm)
	@cd tmpvpp && wget http://cbs.centos.org/kojifiles/packages/vpp/$(VPPDOTVERSION)/1/x86_64/vpp-lib-$(VPPDOTVERSION)-1.x86_64.rpm
	@cd tmpvpp && wget http://cbs.centos.org/kojifiles/packages/vpp/$(VPPDOTVERSION)/1/x86_64/vpp-devel-$(VPPDOTVERSION)-1.x86_64.rpm
	@cd tmpvpp && rpm2cpio ./vpp-devel-$(VPPDOTVERSION)-1.x86_64.rpm | cpio -ivd \
		./usr/include/vpp-api/client/vppapiclient.h
	@cd tmpvpp && rpm2cpio ./vpp-lib-$(VPPDOTVERSION)-1.x86_64.rpm | cpio -ivd \
		./usr/lib64/libvppapiclient.so.0.0.0
	@cd tmpvpp && rpm2cpio ./vpp-lib-$(VPPDOTVERSION)-1.x86_64.rpm | cpio -ivd \
		./usr/share/vpp/api/interface.api.json \
		./usr/share/vpp/api/l2.api.json \
		./usr/share/vpp/api/memif.api.json \
		./usr/share/vpp/api/vhost_user.api.json \
		./usr/share/vpp/api/vpe.api.json
else ifeq ($(PKG),deb)
	@cd tmpvpp && wget https://nexus.fd.io/content/repositories/fd.io.stable.$(VPPVERSION).ubuntu.xenial.main/io/fd/vpp/vpp/$(VPPDOTVERSION)-release_amd64/vpp-$(VPPDOTVERSION)-release_amd64-deb.deb
	@cd tmpvpp && wget https://nexus.fd.io/content/repositories/fd.io.stable.$(VPPVERSION).ubuntu.xenial.main/io/fd/vpp/vpp-lib/$(VPPDOTVERSION)-release_amd64/vpp-lib-$(VPPDOTVERSION)-release_amd64-deb.deb
	@cd tmpvpp && wget https://nexus.fd.io/content/repositories/fd.io.stable.$(VPPVERSION).ubuntu.xenial.main/io/fd/vpp/vpp-dev/$(VPPDOTVERSION)-release_amd64/vpp-dev-$(VPPDOTVERSION)-release_amd64-deb.deb
	@cd tmpvpp && wget https://nexus.fd.io/content/repositories/fd.io.stable.$(VPPVERSION).ubuntu.xenial.main/io/fd/vpp/vpp-plugins/$(VPPDOTVERSION)-release_amd64/vpp-plugins-$(VPPDOTVERSION)-release_amd64-deb.deb
	@cd tmpvpp && dpkg-deb --fsys-tarfile vpp-dev-$(VPPDOTVERSION)-release_amd64-deb.deb | tar -x \
		./usr/include/vpp-api/client/vppapiclient.h
	@cd tmpvpp && dpkg-deb --fsys-tarfile vpp-lib-$(VPPDOTVERSION)-release_amd64-deb.deb | tar -x \
		./usr/lib/x86_64-linux-gnu/libvppapiclient.so.0.0.0
	@cd tmpvpp && dpkg-deb --fsys-tarfile vpp-$(VPPDOTVERSION)-release_amd64-deb.deb | tar -x \
		./usr/share/vpp/api/interface.api.json \
		./usr/share/vpp/api/l2.api.json \
		./usr/share/vpp/api/vhost_user.api.json \
		./usr/share/vpp/api/vpe.api.json
	@cd tmpvpp && dpkg-deb --fsys-tarfile vpp-plugins-$(VPPDOTVERSION)-release_amd64-deb.deb | tar -x \
		./usr/share/vpp/api/memif.api.json
endif
	@$(SUDO) -E mkdir -p /usr/include/vpp-api/client/
	@$(SUDO) -E cp tmpvpp/usr/include/vpp-api/client/vppapiclient.h /usr/include/vpp-api/client/.
	@$(SUDO) -E chown -R bin:bin /usr/include/vpp-api/
	@echo   Installed /usr/include/vpp-api/client/vppapiclient.h
	@$(SUDO) -E cp tmpvpp$(VPPLIBDIR)/libvppapiclient.so.0.0.0 $(VPPLIBDIR)/.
	@$(SUDO) -E ln -s $(VPPLIBDIR)/libvppapiclient.so.0.0.0 $(VPPLIBDIR)/libvppapiclient.so
	@$(SUDO) -E ln -s $(VPPLIBDIR)/libvppapiclient.so.0.0.0 $(VPPLIBDIR)/libvppapiclient.so.0
	@$(SUDO) -E chown -R bin:bin $(VPPLIBDIR)/libvppapiclient.so*
	@echo   Installed $(VPPLIBDIR)/libvppapiclient.so
	@$(SUDO) -E mkdir -p /usr/share/vpp/api/
	@$(SUDO) -E cp tmpvpp/usr/share/vpp/api/*.json /usr/share/vpp/api/.
	@$(SUDO) -E chown -R bin:bin /usr/share/vpp/
	@echo   Installed /usr/share/vpp/api/*.json
	@rm -rf tmpvpp
endif
ifeq ($(OVS_PY_INSTALLED),0)
	@echo OVS Python Script not installed. Installing now.
	@$(SUDO) -E mkdir -p /usr/share/openvswitch/scripts/
	@$(SUDO) -E cp ./cniovs/scripts/ovs-config.py /usr/share/openvswitch/scripts/.
endif


extras:
	@cd vendor/git.fd.io/govpp.git/cmd/binapi-generator && go build -v
	@./vendor/git.fd.io/govpp.git/cmd/binapi-generator/binapi-generator \
		--input-dir=/usr/share/vpp/api/ \
		--output-dir=vendor/git.fd.io/govpp.git/core/bin_api/
	@cd cnivpp/vpp-app && go build -v

clean:
	@rm -f cnivpp/vpp-app/vpp-app
	@rm -f cnivpp/test/memifAddDel/memifAddDel
	@rm -f cnivpp/test/vhostUserAddDel/vhostUserAddDel
	@rm -f cnivpp/test/ipAddDel/ipAddDel
	@rm -f vendor/git.fd.io/govpp.git/cmd/binapi-generator/binapi-generator 
	@rm -f userspace/userspace
ifeq ($(VPPLCLINSTALLED),1)
	@echo VPP was installed by *make install*, so cleaning up files.
	@$(SUDO) -E rm -rf /usr/include/vpp-api/
	@$(SUDO) -E rm $(VPPLIBDIR)/libvppapiclient.so*
	@$(SUDO) -E rm -rf /usr/share/vpp/
endif

generate:

lint:

.PHONY: build test install extras clean generate

