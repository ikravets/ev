# Copyright (c) Ilia Kravets, 2014-2016. All rights reserved. PROVIDED "AS IS"
# WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

GO_WORK_DIR         := $(CURDIR)
GO_VENDOR_DIR       := $(GO_WORK_DIR)/vendor
GO_VENDOR_LOCAL_DIR := $(wildcard $(GO_WORK_DIR)/vendor.local)
GO_VNDS             := $(GO_WORK_DIR) $(GO_VENDOR_DIR) $(GO_VENDOR_LOCAL_DIR)
GOPATH_ORIG         := $(GOPATH)
GO_BUILD_DIR         = $(GO_WORK_DIR)/build
GO_DIRS              = $(GO_VNDS:$(GO_WORK_DIR)%=$(GO_BUILD_DIR)%)
GOPATH               = $(subst $(_go_space),:,$(GO_DIRS))
GOPATH_COMBINED      = $(GOPATH):$(GOPATH_ORIG)
export GOPATH
_go_empty:=
_go_space:= $(_go_empty) $(_go_empty)

.PHONY: go-build go-shell go-get

go-build: go-get $(GO_BUILD_DIR)
	@echo GOPATH=$$GOPATH
	go install -v $(GO_INST)
$(GO_BUILD_DIR): $(GO_DIRS:=/src)
	touch "$@"
$(GO_DIRS:=/src):
	mkdir -p $(@D)
	ln -s "$(GO_WORK_DIR)$(@:$(GO_BUILD_DIR)%=%)" "$@"

go-get: $(GO_DEPS:%=go-get-%)
go-get-%: $(GO_VENDOR_DIR)/.stamp.get-% ;
$(GO_VENDOR_DIR)/.stamp.get-%:
	git clone $($*-url) $(GO_VENDOR_DIR)/src/$($*-dir)
	cd $(GO_VENDOR_DIR)/src/$($*-dir) && git checkout $($*-cid)
	touch "$@"
.SECONDARY: $(GO_DEPS:%=$(GO_VENDOR_DIR)/.stamp.get-%)

go-shell: GOPATH := $(GOPATH_COMBINED)
go-shell:
	@echo GOPATH=$$GOPATH
	$(SHELL)
