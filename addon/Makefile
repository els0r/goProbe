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

PKG    = goProbe
PREFIX = /opt/ntm

# GoLang main version
BASEPATH		= github.com/els0r/goProbe
GO_PRODUCT	    = goProbe
GO_QUERY        = goQuery

# get the operating system and git variables for versioning
UNAME_OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
GIT_COMMIT := $(shell git rev-parse HEAD)
GIT_VERSION := $(shell git describe --tags)
BUILD_DATE  := $(shell date +%FT%T%z)

# easy to use build command for everything related goprobe
VERSION_PATH = $(BASEPATH)/pkg/version

GO_BUILDTAGS = netcgo public $(UNAME_OS)
GO_LDFLAGS   = -ldflags '-X $(VERSION_PATH).version=$(GIT_VERSION) -X $(VERSION_PATH).commit=$(GIT_COMMIT) -X $(VERSION_PATH).builddate=$(BUILD_DATE)'
GPBUILD      = go install -i -tags '$(GO_BUILDTAGS)' $(GO_LDFLAGS)
GPTESTBUILD  = go test -c -tags '$(GO_BUILDTAGS)'

# gopacket and gopcap
GOPACKET_SRC = github.com/fako1024/gopacket

fetch:

	echo "*** fetching dependencies"
	go get github.com/mdlayher/raw
	go get $(GOPACKET_SRC)
	go get github.com/els0r/status

compile:

	## GO CODE COMPILATION ##
	echo "*** compiling $(GO_PRODUCT) ***"
	$(GPBUILD) $(BASEPATH)/cmd/$(GO_PRODUCT)   # build the goProbe binary

	echo "*** compiling $(GO_QUERY) ***"
	$(GPBUILD) $(BASEPATH)/cmd/$(GO_QUERY)     # build the goquery binary

	echo "*** compiling goConvert ***"
	$(GPBUILD) $(BASEPATH)/cmd/goConvert 		# build the conversion tool

install: go_install

go_install:

	rm -rf absolute

	# additional directories
	echo "*** creating binary tree ***"
	mkdir -p absolute$(PREFIX)/$(PKG)/bin    && chmod 755 absolute$(PREFIX)/$(PKG)/bin
	mkdir -p absolute$(PREFIX)/$(PKG)/etc    && chmod 755 absolute$(PREFIX)/$(PKG)/etc
	mkdir -p absolute/etc/init.d             && chmod 755 absolute/etc/init.d

	echo "*** installing $(GO_PRODUCT) and $(GO_QUERY) ***"
	mv $(GOPATH)/bin/goProbe 	absolute$(PREFIX)/$(PKG)/bin
	mv $(GOPATH)/bin/goQuery   	absolute$(PREFIX)/$(PKG)/bin
	mv $(GOPATH)/bin/goConvert  absolute$(PREFIX)/$(PKG)/bin

	# change the prefix variable in the init script
	cp goprobe.init absolute/etc/init.d/goprobe.init
	sed "s#PREFIX=#PREFIX=$(PREFIX)#g" -i absolute/etc/init.d/goprobe.init

	echo "*** generating example configuration ***"
	echo -e "{\n\t\"db_path\" : \"$(PREFIX)/$(PKG)/db\",\n\t\"interfaces\" : {\n\t\t\"eth0\" : {\n\t\t\t\"bpf_filter\" : \"not arp and not icmp\",\n\t\t\t\"buf_size\" : 2097152,\n\t\t\t\"promisc\" : false\n\t\t}\n\t}\n}" > absolute$(PREFIX)/$(PKG)/etc/goprobe.conf.example

	#set the appropriate permissions
	chmod -R 755 absolute$(PREFIX)/$(PKG)/bin \
		absolute$(PREFIX)/$(PKG)/etc \
		absolute/etc/init.d \

	echo "*** cleaning unneeded files ***"

	# strip binaries
	if [ "$(UNAME_OS)" != "darwin" ]; \
	then \
		strip --strip-unneeded absolute$(PREFIX)/$(PKG)/bin/*; \
	fi

package: go_package

go_package:

	cd absolute; tar cjf $(PKG).tar.bz2 *; mv $(PKG).tar.bz2 ../

deploy:

	if [ "$(USER)" != "root" ]; \
	then \
		echo "*** [deploy] Error: command must be run as root"; \
	else \
		echo "*** syncing binary tree ***"; \
		rsync -a absolute/ /; \
		ln -sf $(PREFIX)/$(PKG)/bin/goQuery /usr/local/bin/goquery; \
		chown root.root /etc/init.d/goprobe.init; \
	fi

clean:

	echo "*** removing binary tree ***"
	rm -rf absolute

	echo "*** removing dependencies and binaries ***"
	rm -rf cmd/$(GO_PRODUCT)/$(GO_PRODUCT) cmd/$(GO_QUERY)/$(GO_QUERY) cmd/goConvert/goConvert

	rm -rf $(PKG).tar.bz2

all: clean fetch compile install

.SILENT:
