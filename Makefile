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
GO_BUILDTAGS     = netcgo public
GO_LDFLAGS       = -X OSAG/version.version=$(VERSION) -X OSAG/version.commit=$(GIT_DIRTY)$(GIT_COMMIT) -X OSAG/version.builddate=$(TODAY)

# easy to use build command for everything related goprobe
GPBUILD     = go build -tags '$(GO_BUILDTAGS)' -ldflags '$(GO_LDFLAGS)' -a
GPTESTBUILD = go test -c -tags '$(GO_BUILDTAGS)' -ldflags '$(GO_LDFLAGS)' -a

SHELL := /bin/bash

PKG    = goProbe
PREFIX = /opt/ntm

# downloader used for grabbing the external code. Change this if it does not
# correspond to the usual way you download files on your system
DOWNLOAD	= curl --progress-bar -L --url

# GoLang main version
GO_PRODUCT	    = goProbe
GO_QUERY        = goQuery

GOLANG		    = go1.11.1.linux-amd64
GOLANG_SITE	  = https://storage.googleapis.com/golang
GO_SRCDIR	    = $(PWD)/addon/gocode/src

# for providing the go compiler with the right env vars
export GOROOT := $(PWD)/go
export PATH := $(GOROOT)/bin:$(PATH)
export GOPATH := $(PWD)/addon/gocode

# gopacket and gopcap
GOPACKET      = 1.1.15
GOPACKET_REV  = 1.1.15
GOPACKET_SITE = https://github.com/google/gopacket/archive
GOPACKETDIR   = github.com/google

# pcap libraries
PCAP_VERSION = 1.9.0
PCAP		 = libpcap-$(PCAP_VERSION)
PCAP_SITE	 = http://www.tcpdump.org/release
PCAP_DIR	 := $(PWD)/$(PCAP)

# libprotoident libraries
LIBTRACE	    = libtrace-3.0.20
LIBTRACE_SITE	= http://research.wand.net.nz/software/libtrace
LIBTRACE_DIR	:= $(PWD)/$(LIBTRACE)

LIBPROTOIDENT	= libprotoident-2.0.7
LPIDENT_SITE	= http://research.wand.net.nz/software/libprotoident
LPIDENT_DIR	    := $(PWD)/$(LIBPROTOIDENT)
export LD_LIBRARY_PATH := $(PWD)/$(PCAP)

# for building with cgo
export CGO_CFLAGS := -I$(PCAP_DIR)
export CGO_LDFLAGS := -L$(PCAP_DIR)

fetch:
	## GO SETUP ##
	echo "*** downloading $(GOLANG) ***"
	$(DOWNLOAD) $(GOLANG_SITE)/$(GOLANG).tar.gz -O
	# Useful for debugging:
	# cp ~/$(GOLANG).tar.gz .

	echo "*** downloading gopacket_$(GOPACKET) ***"
	$(DOWNLOAD) $(GOPACKET_SITE)/v$(GOPACKET).tar.gz -O

	echo "*** downloading $(PCAP) ***"
	$(DOWNLOAD) $(PCAP_SITE)/$(PCAP).tar.gz -O

unpack:
	echo "*** unpacking $(GOLANG) ***"
	tar xf $(GOLANG).tar.gz

	echo "*** unpacking dependency gopacket_$(GOPACKET) ***"
	tar xf v$(GOPACKET).tar.gz
	mv gopacket-$(GOPACKET) gopacket

	echo "*** fetching gopacket dependencies"
	go get github.com/mdlayher/raw

	echo "*** unpacking dependency $(PCAP) ***"
	tar xf $(PCAP).tar.gz

patch:
	echo "*** patching dependency gopacket_$(GOPACKET) ***"
	patch -Np0 < addon/gopacket-v$(GOPACKET).patch
	# change the library path inside pcap.go
	sed -i -e 's#LIBPCAPPATH#$(PCAP_DIR)#g' gopacket/pcap/pcap.go

	mkdir -p $(GO_SRCDIR)/$(GOPACKETDIR)
	mv gopacket $(GO_SRCDIR)/$(GOPACKETDIR)

	echo "*** patching dependency $(PCAP) ***"
	patch -Np0 < addon/libpcap-$(PCAP_VERSION).patch

configure:
	echo "*** configuring dependency $(PCAP) ***"
	cd $(PCAP); sh configure --prefix=$(PREFIX)/$(PKG) --quiet >> /dev/null

compile:

	## GO CODE COMPILATION ##
	# first, compile libpcap and libprotoident because the go code depends on it
	echo "*** compiling $(PCAP) ***"
	cd $(PCAP); make -s > /dev/null; rm libpcap.a; ln -sf libpcap.so.$(PCAP_VERSION) libpcap.so; ln -sf libpcap.so.$(PCAP_VERSION) libpcap.so.1

	# make the protocol-category mappings available to goquery using the
	# compiled helper binary and a bash script for IP protocols
	addon/serialize_ipprot_list.sh > $(GO_SRCDIR)/OSAG/goDB/GPDPIProtocols.go

	# make all reverse lookup keys lowercase
	sed -i 's/\"\(.*\)\": \([0-9]*\)/\L"\1": \2/g' $(GO_SRCDIR)/OSAG/goDB/GPDPIProtocols.go

	echo "*** compiling $(GO_PRODUCT) ***"
	cd $(GO_SRCDIR)/OSAG/capture; $(GPBUILD) -o $(GO_PRODUCT)   # build the goProbe binary

	echo "*** compiling $(GO_QUERY) ***"
	 cd $(GO_SRCDIR)/OSAG/query; $(GPBUILD) -o $(GO_QUERY)      # build the goquery binary

install: go_install

go_install:

	rm -rf absolute

	# additional directories
	echo "*** creating binary tree ***"
	mkdir -p absolute$(PREFIX)/$(PKG)/etc    && chmod 755 absolute$(PREFIX)/$(PKG)/etc
	mkdir -p absolute$(PREFIX)/$(PKG)/shared && chmod 755 absolute$(PREFIX)/$(PKG)/shared
	mkdir -p absolute/etc/init.d             && chmod 755 absolute/etc/init.d

	echo "*** installing $(GO_PRODUCT) and $(GO_QUERY) ***"
	cd $(PCAP); make -s install DESTDIR=$(PWD)/absolute >> /dev/null

	cp $(GO_SRCDIR)/OSAG/capture/$(GO_PRODUCT) absolute$(PREFIX)/$(PKG)/bin
	cp $(GO_SRCDIR)/OSAG/query/$(GO_QUERY)     absolute$(PREFIX)/$(PKG)/bin
	cp addon/gp_status.pl                      absolute$(PREFIX)/$(PKG)/shared

	# change the prefix variable in the init script
	cp addon/goprobe.init absolute/etc/init.d/goprobe.init
	sed "s#PREFIX=#PREFIX=$(PREFIX)#g" -i absolute/etc/init.d/goprobe.init

	echo "*** generating example configuration ***"
	echo -e "{\n\t\"db_path\" : \"$(PREFIX)/$(PKG)/db\",\n\t\"interfaces\" : {\n\t\t\"eth0\" : {\n\t\t\t\"bpf_filter\" : \"not arp and not icmp\",\n\t\t\t\"buf_size\" : 2097152,\n\t\t\t\"promisc\" : false\n\t\t}\n\t}\n}" > absolute$(PREFIX)/$(PKG)/etc/goprobe.conf.example

	# set the appropriate permissions
	chmod -R 755 absolute$(PREFIX)/$(PKG)/bin \
		absolute$(PREFIX)/$(PKG)/shared \
		absolute$(PREFIX)/$(PKG)/etc \
		absolute$(PREFIX)/$(PKG)/lib \
		absolute/etc/init.d \

	echo "*** cleaning unneeded files ***"

	# binary cleaning
	# libpcap binaries
	rm -f absolute$(PREFIX)/$(PKG)/bin/pcap-config

	# library cleaning
	rm -f absolute$(PREFIX)/$(PKG)/lib/*.la
	rm -f absolute$(PREFIX)/$(PKG)/lib/*.lai
	rm -f absolute$(PREFIX)/$(PKG)/lib/*.a
	rm -rf absolute$(PREFIX)/$(PKG)/include
	rm -rf absolute$(PREFIX)/$(PKG)/share
	rm -rf absolute$(PREFIX)/$(PKG)/lib/libpacketdump*

	# strip binaries
	strip --strip-unneeded absolute$(PREFIX)/$(PKG)/bin/*

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

	echo "*** removing $(GOLANG), gopacket and $(PCAP) ***"
	rm -rf go $(GOLANG).tar.gz
	rm -rf $(GO_SRCDIR)/$(GOPACKETDIR) gopacket-$(GOPACKET_REV) gopacket-$(GOPACKET) gopacket $(GO_SRCDIR)/code.google.com v$(GOPACKET).tar.gz $(GO_SRCDIR)/OSAG/capture/$(GO_PRODUCT) $(GO_SRCDIR)/OSAG/query/$(GO_QUERY)
	rm -rf $(PCAP) $(PCAP).tar.gz

	rm -rf $(PKG).tar.bz2

all: clean fetch unpack patch configure compile install

.SILENT:
