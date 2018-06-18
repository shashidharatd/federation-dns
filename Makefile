# Copyright 2018 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

TARGET = federation-dns
GOTARGET = github.com/shashidharatd/$(TARGET)
REGISTRY ?= shashidharatd
IMAGE = $(REGISTRY)/$(TARGET)
DIR := ${CURDIR}
DOCKER ?= docker

GIT_VERSION ?= $(shell git describe --always --dirty)
GIT_TAG ?= $(shell git describe --tags --exact-match 2>/dev/null)

ifneq ($(VERBOSE),)
VERBOSE_FLAG = -v
endif
BUILDMNT = /go/src/$(GOTARGET)
BUILD_IMAGE ?= golang:alpine
BUILDCMD = go build -o $(TARGET) $(VERBOSE_FLAG) -ldflags "-X github.com/shashidharatd/federation-dns/pkg/buildinfo.Version=$(GIT_VERSION)"
BUILD = $(BUILDCMD) cmd/${TARGET}/main.go

TESTARGS ?= $(VERBOSE_FLAG) -timeout 60s
TEST_PKGS ?= $(GOTARGET)/cmd/... $(GOTARGET)/pkg/...
TEST_CMD = go test $(TESTARGS)
TEST = $(TEST_CMD) $(TEST_PKGS) 

VET = go vet $(TEST_PKGS)
FMT = go fmt $(TEST_PKGS)

WORKDIR ?= /federation-dns
DOCKER_BUILD ?= $(DOCKER) run --rm -v $(DIR):$(BUILDMNT) -w $(BUILDMNT) $(BUILD_IMAGE) /bin/sh -c

.PHONY: all container push clean cbuild test local-test local generate vet fmt

all: container

local-test: 
	$(TEST)

# Unit tests 
test: cbuild vet
	$(DOCKER_BUILD) '$(TEST)'

vet:
	$(DOCKER_BUILD) '$(VET)'

fmt:
	$(FMT)

container:
	$(DOCKER) build \
		-t $(REGISTRY)/$(TARGET):$(GIT_VERSION) \
		.

cbuild:
	$(DOCKER_BUILD) '$(BUILD)'

push:
	$(DOCKER) push $(REGISTRY)/$(TARGET):$(GIT_VERSION)
	if git describe --tags --exact-match >/dev/null 2>&1; \
	then \
		$(DOCKER) tag $(REGISTRY)/$(TARGET):$(GIT_VERSION) $(REGISTRY)/$(TARGET):$(GIT_TAG); \
		$(DOCKER) push $(REGISTRY)/$(TARGET):$(GIT_TAG); \
		$(DOCKER) tag $(REGISTRY)/$(TARGET):$(GIT_VERSION) $(REGISTRY)/$(TARGET):latest; \
		$(DOCKER) push $(REGISTRY)/$(TARGET):latest; \
	fi

clean:
	rm -f $(TARGET)
	$(DOCKER) rmi $(REGISTRY)/$(TARGET) || true
