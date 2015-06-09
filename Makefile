MAKEFLAGS += --warn-undefined-variables
SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail
define NL


endef

VENDOR_DIR := vendor
VENDOR_LOCAL_DIR := $(VENDOR_DIR).local
GOPATH_ORIG := $(GOPATH)
GOPATH = $(BUILD_DIR)/$(VENDOR_DIR):$(BUILD_DIR)
export GOPATH

.PHONY: build build-centos shell edit get


build: BUILD_DIR=$(CURDIR)/build
build:
	mkdir -p "$(BUILD_DIR)/$(VENDOR_DIR)"
	$(foreach d,src $(VENDOR_DIR)/src,\
	    [[ -e "$(BUILD_DIR)/$(d)" ]] || ln -s "$(CURDIR)/$(d)" "$(BUILD_DIR)/$(d)" $(NL))
	go install -v my/ev/...

build-centos: LOCAL_BUILD_DIR=$(CURDIR)/build.centos
build-centos: BUILD_DIR=/home/devuser/go
build-centos:
	mkdir -p $(foreach v,. $(VENDOR_DIR) $(VENDOR_LOCAL_DIR),$(LOCAL_BUILD_DIR)/$(v)/src)
	docker run --tty --interactive --user 1000 --env GOPATH \
	    --volume "$(LOCAL_BUILD_DIR):$(BUILD_DIR)" \
	    $(foreach d,src $(VENDOR_DIR)/src $(VENDOR_LOCAL_DIR)/src,\
		--volume "$(CURDIR)/$(d):$(BUILD_DIR)/$(d)") \
	    ekagobuild \
	    go install -v my/ev/...

shell: BUILD_DIR=$(CURDIR)/build
shell:
	@echo GOPATH=$$GOPATH
	bash

edit: BUILD_DIR=$(CURDIR)/build
edit: GOPATH:=$(GOPATH):$(GOPATH_ORIG)
edit:
	@echo GOPATH=$$GOPATH
	cd $(BUILD_DIR)/src/my/ev; gvim


DEPS := b go-flags struc gopacket errs

b-url := https://github.com/cznic/b
b-dir := $(b-url:https://%=%)
b-cid := master

go-flags-url := https://github.com/jessevdk/go-flags
go-flags-dir := $(go-flags-url:https://%=%)
go-flags-cid := master

struc-url := https://github.com/lunixbochs/struc
struc-dir := $(struc-url:https://%=%)
struc-cid := master

gopacket-url := https://github.com/google/gopacket
gopacket-dir := $(gopacket-url:https://%=%)
gopacket-cid := master

.SECONDARY: $(DEPS:%=$(VENDOR_DIR)/.stamp.get-%)
$(VENDOR_DIR)/.stamp.get-errs:
	ln -s ../../$(VENDOR_LOCAL_DIR)/src/my $(VENDOR_DIR)/src
	touch "$@"
$(VENDOR_DIR)/.stamp.get-%:
	git clone $($*-url) $(VENDOR_DIR)/src/$($*-dir)
	git -C $(VENDOR_DIR)/src/$($*-dir) checkout $($*-cid)
	touch "$@"
get-%: $(VENDOR_DIR)/.stamp.get-%
	@
get: $(DEPS:%=get-%)
