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
	@echo " make                - Build UserSpace CNI."
	@echo " make clean          - Cleanup all build artifacts. Will remove VPP files installed from *make install*."
	@echo " make install        - If VPP is not installed, install the minimum set of files to build."
	@echo "                       CNI-VPP will fail because VPP is still not installed."
	@echo " make install-dep    - Install software dependencies, currently only needed for *make install*."
	@echo " make extras         - Build *usrsp-app*, small binary to run in Docker container for testing."
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
endif
	@$(SUDO) -E chown -R bin:bin /usr/share/vpp/
	@echo   Installed /usr/share/vpp/api/*.json
	@rm -rf tmpvpp
endif


extras:
	@cd vendor/git.fd.io/govpp.git/cmd/binapi-generator && go build -v
	@./vendor/git.fd.io/govpp.git/cmd/binapi-generator/binapi-generator \
		--input-dir=/usr/share/vpp/api/ \
		--output-dir=vendor/git.fd.io/govpp.git/core/bin_api/
	@cd docker/usrsp-app && go build -v

clean:
	@rm -f docker/usrsp-app/usrsp-app
	@rm -f cnivpp/test/memifAddDel/memifAddDel
	@rm -f cnivpp/test/vhostUserAddDel/vhostUserAddDel
	@rm -f cnivpp/test/ipAddDel/ipAddDel
	@rm -f vendor/git.fd.io/govpp.git/cmd/binapi-generator/binapi-generator
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

lint:

.PHONY: build test install extras clean generate

