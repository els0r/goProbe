###############################################################################
#
# Makefile: Makefile to build the goProbe traffic monitor
#
# Written by Lennart Elsen and Fabian Kohn, August 2014
# Copyright (c) 2014 Open Systems AG, Switzerland
# All Rights Reserved.
#
# Package for network traffic statistics capture (goProbe), storage (goDB)
# and retrieval (goquery)
#
################################################################################
# This code has been developed by Open Systems AG
#
# goProbe is free software; you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation; either version 2 of the License, or
# (at your option) any later version.
#
# goProbe is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with goProbe; if not, write to the Free Software
# Foundation, Inc., 59 Temple Place, Suite 330, Boston, MA  02111-1307  USA

# Build tags for go compilation
# 'netcgo' tells go to use the system resolver for name resolution.
# (See https://golang.org/pkg/net/#pkg-overview)
# We use the 'OSAG' build tag to switch between implementations. When the OSAG
# tag is specified, we use the internal/confidential code, otherwise the
# public code is used.
SHELL := /bin/bash

# GoLang main version
BASEPATH	= github.com/els0r/goProbe
GO_PRODUCT	= goProbe
GO_QUERY	= goQuery

# get the operating system and git variables for versioning
UNAME_OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
COMMIT_SHA := $(shell git rev-parse HEAD)
COMMIT_SHA_SHORT := $(shell git rev-parse --short HEAD)
GIT_VERSION := $(shell git describe --tags)
BUILD_DATE  := $(shell date +%FT%T%z)

GOOS := $(shell uname | tr '[:upper:]' '[:lower:]')
GOARCH := $(shell uname -m)

# Debian build variables
DEB_VERSION := $(shell echo $(GIT_VERSION) | sed 's/v//g')
DEB_VERSION := '$(DEB_VERSION)-$(COMMIT_SHA_SHORT)'
DEB_BASE_IMAGE?=golang:latest

# easy to use build command for everything related goprobe
VERSION_PATH = $(BASEPATH)/pkg/version

GO_BUILDTAGS = netcgo public $(UNAME_OS)

GPBUILD      := go install -v -tags '$(GO_BUILDTAGS)'
GPTESTBUILD  := go test -c -tags '$(GO_BUILDTAGS)'

fetch:

	echo "*** fetching dependencies"
	go mod tidy
	go mod download

build_version:

	## VERSION ###
	echo "*** setting version information ***"
	COMMIT_SHA=$(COMMIT_SHA) go generate $(VERSION_PATH)
	echo "commit is $(COMMIT_SHA)"

build_goprobe:

	## GO CODE COMPILATION ##
	echo "*** compiling $(GO_PRODUCT) ***"
	echo "$(GPBUILD) $(BASEPATH)/cmd/$(GO_PRODUCT)"
	$(GPBUILD) $(BASEPATH)/cmd/$(GO_PRODUCT)

build_goquery:

	echo "*** compiling $(GO_QUERY) ***"
	$(GPBUILD) $(BASEPATH)/cmd/$(GO_QUERY)

build_goconvert:

	echo "*** compiling goConvert ***"
	$(GPBUILD) $(BASEPATH)/cmd/goConvert

build_legacy:

	echo "*** compiling legacy DB conversion tool ***"
	$(GPBUILD) $(BASEPATH)/cmd/legacy

build: build_version build_goprobe build_goquery build_goconvert build_legacy

install: go_install

go_install:

	rm -rf absolute

	# additional directories
	echo "*** creating binary tree ***"
	mkdir -p absolute/bin                    && chmod 755 absolute/bin
	mkdir -p absolute/etc/systemd/system     && chmod 755 absolute/etc/systemd/system

	echo "*** installing binaries ***"
	cp $(GOPATH)/bin/goProbe 	absolute/bin
	cp $(GOPATH)/bin/goQuery   	absolute/bin
	cp $(GOPATH)/bin/goConvert  absolute/bin
	cp $(GOPATH)/bin/legacy     absolute/bin

	# systemd service definition
	cp goprobe.service absolute/etc/systemd/system/goprobe.service

	echo "*** copying example configuration ***"
	cp goprobe.conf.example absolute/etc/

	#set the appropriate permissions for files
	chmod -R 755 absolute/bin

	echo "*** cleaning unneeded files ***"
	# strip binaries
	if [ "$(UNAME_OS)" != "darwin" ]; \
	then \
		strip --strip-unneeded absolute/bin/*; \
	fi

package: go_package

go_package:

	cd absolute; tar cjf $(GO_PRODUCT).tar.bz2 *; mv $(GO_PRODUCT).tar.bz2 ../

deb:

	echo "*** building Debian package against $(DEB_BASE_IMAGE) ***"
	docker build -t goprobe:$(DEB_VERSION) --build-arg BASE_IMAGE=$(DEB_BASE_IMAGE) --build-arg DEB_VERSION=$(DEB_VERSION) --build-arg GO_PRODUCT=$(GO_PRODUCT) -f Dockerfile.debpkg .
	docker run --rm --entrypoint cat goprobe:$(DEB_VERSION) /$(GO_PRODUCT)-$(DEB_VERSION).deb > $(GO_PRODUCT)-$(DEB_VERSION).deb
	docker rmi goprobe:$(DEB_VERSION)

deploy:

	if [ "$(USER)" != "root" ]; \
	then \
		echo "*** [deploy] Error: command must be run as root"; \
	else \
		echo "*** syncing binary tree ***"; \
		rsync -a absolute/ /; \
		chown root.root /etc/systemd/system/goprobe.service; \
		systemctl daemon-reload
	fi

clean:

	echo "*** removing binary tree ***"
	rm -rf absolute

	echo "*** removing dependencies and binaries ***"
	rm -rf cmd/$(GO_PRODUCT)/$(GO_PRODUCT) cmd/$(GO_QUERY)/$(GO_QUERY) cmd/goConvert/goConvert

	rm -f v$(LZ4_VERSION).tar.gz
	rm -rf $(LZ4_LIB)

	rm -f $(GO_PRODUCT).tar.bz2
	rm -f $(GO_PRODUCT)-$(DEB_VERSION).deb

all: clean fetch build install

.SILENT:
