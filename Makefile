MAKEFLAGS += --warn-undefined-variables
SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail
define NL


endef

VENDOR_DIR := vendor
VENDOR_LOCAL_DIR := $(VENDOR_DIR).local
BUILD_DIR=$(CURDIR)/build
BUILD_DIR_centos=$(CURDIR)/build.centos
GOPATH_ORIG := $(GOPATH)
GOPATH = $(BUILD_DIR)/$(VENDOR_DIR):$(BUILD_DIR)
export GOPATH

.PHONY: build build-centos shell edit get


build:
	mkdir -p "$(BUILD_DIR)/$(VENDOR_DIR)"
	$(foreach d,src $(VENDOR_DIR)/src,\
	    [[ -e "$(BUILD_DIR)/$(d)" ]] || ln -s "$(CURDIR)/$(d)" "$(BUILD_DIR)/$(d)" $(NL))
	@echo GOPATH=$(GOPATH)
	go install -v my/ev/...

build-centos: BUILD_DIR=/home/devuser/go
build-centos:
	mkdir -p $(foreach v,. $(VENDOR_DIR) $(VENDOR_LOCAL_DIR),$(BUILD_DIR_centos)/$(v)/src)
	docker run --tty --interactive --user 1000 --env GOPATH \
	    --volume "$(BUILD_DIR_centos):$(BUILD_DIR)" \
	    $(foreach d,src $(VENDOR_DIR)/src $(VENDOR_LOCAL_DIR)/src,\
		--volume "$(CURDIR)/$(d):$(BUILD_DIR)/$(d)") \
	    ekagobuild \
	    go install -v my/ev/...

deploy: deploy-xn02 deploy-xn01
deploy-%: build-centos
	rsync -azP $(BUILD_DIR_centos)/bin/ev $*:
	ssh -t $* 'sudo cp ev /usr/local/bin && sudo setcap CAP_NET_RAW=+ep /usr/local/bin/ev'

shell:
	@echo GOPATH=$$GOPATH
	bash

edit: GOPATH:=$(GOPATH):$(GOPATH_ORIG)
edit:
	@echo GOPATH=$$GOPATH
	cd $(BUILD_DIR)/src/my/ev; gvim


DEPS := b go-flags struc gopacket yaml errs

errs-url := https://github.com/ikravets/errs
errs-dir := $(errs-url:https://%=%)
errs-cid := master

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

yaml-url := https://github.com/go-yaml/yaml
yaml-dir := $(yaml-url:https://%=%)
yaml-cid := v2

.SECONDARY: $(DEPS:%=$(VENDOR_DIR)/.stamp.get-%)
$(VENDOR_DIR)/.stamp.get-%:
	git clone $($*-url) $(VENDOR_DIR)/src/$($*-dir)
	cd $(VENDOR_DIR)/src/$($*-dir) && git checkout $($*-cid)
	touch "$@"
get-%: $(VENDOR_DIR)/.stamp.get-%
	@
get: $(DEPS:%=get-%)
