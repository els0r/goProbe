# Version numbers
VERSION=121
export RELEASE=r$(VERSION)
LIBVER_MAJOR=`sed -n '/define LZ4_VERSION_MAJOR/s/.*[[:blank:]]\([0-9][0-9]*\).*/\1/p' < lz4.h`
LIBVER_MINOR=`sed -n '/define LZ4_VERSION_MINOR/s/.*[[:blank:]]\([0-9][0-9]*\).*/\1/p' < lz4.h`
LIBVER_PATCH=`sed -n '/define LZ4_VERSION_RELEASE/s/.*[[:blank:]]\([0-9][0-9]*\).*/\1/p' < lz4.h`
LIBVER=$(LIBVER_MAJOR).$(LIBVER_MINOR).$(LIBVER_PATCH)

GOOS := $(shell uname | tr '[:upper:]' '[:lower:]')
GOARCH := $(shell uname -m)

STATIC_LIB := liblz4_$(GOARCH)_$(GOOS).a

DESTDIR=
PREFIX = /usr
CC    := $(CC)
CFLAGS?= -O3
CFLAGS+= -arch $(GOARCH) -I. -std=c99 -Wall -Wextra -Wundef -Wshadow -Wstrict-prototypes -fPIC -DLZ4_VERSION=\"$(RELEASE)\"


LIBDIR?= $(PREFIX)/lib
INCLUDEDIR=$(PREFIX)/include
DISTRIBNAME=lz4-$(RELEASE).tar.gz

# Define *.exe as extension for Windows systems
ifneq (,$(filter Windows%,$(OS)))
EXT =.exe
else
EXT =
endif

TEXT = lz4.c lz4.h lz4hc.c lz4hc.h \
	liblz4.pc.in Makefile \
	lz4_format_description.txt NEWS LICENSE README.md \
	cmake_unofficial/CMakeLists.txt
NONTEXT = LZ4_Streaming_Format.odt
SOURCES = $(TEXT) $(NONTEXT)


default: liblz4

all: liblz4

liblz4: lz4.c lz4hc.c
	@echo compiling static library
	$(CC) $(CPPFLAGS) $(CFLAGS) -c $^ 
	@$(AR) rcs $(STATIC_LIB) lz4.o lz4hc.o

clean:
	@rm -f core *.o $(DISTRIBNAME) *.sha1 liblz4.pc
	@echo Cleaning completed


#------------------------------------------------------------------------
#make install is validated only for Linux, OSX, kFreeBSD and Hurd targets
ifneq (,$(filter $(shell uname),Linux Darwin GNU/kFreeBSD GNU))

liblz4.pc: liblz4.pc.in Makefile
	@echo creating pkgconfig
	@sed -e 's|@PREFIX@|$(PREFIX)|' \
            -e 's|@LIBDIR@|$(LIBDIR)|' \
            -e 's|@INCLUDEDIR@|$(INCLUDEDIR)|' \
            -e 's|@VERSION@|$(VERSION)|' \
             $< >$@

install: liblz4 liblz4.pc
	@install -d -m 755 $(DESTDIR)$(LIBDIR)/pkgconfig/ $(DESTDIR)$(INCLUDEDIR)/
	@install -m 755 liblz4.$(SHARED_EXT_VER) $(DESTDIR)$(LIBDIR)/liblz4.$(SHARED_EXT_VER)
	@cp -a liblz4.$(SHARED_EXT_MAJOR) $(DESTDIR)$(LIBDIR)
	@cp -a liblz4.$(SHARED_EXT) $(DESTDIR)$(LIBDIR)
	@cp -a liblz4.pc $(DESTDIR)$(LIBDIR)/pkgconfig/
	@install -m 644 liblz4_linux.a $(DESTDIR)$(LIBDIR)/liblz4_linux.a
	@install -m 644 lz4.h $(DESTDIR)$(INCLUDEDIR)/lz4.h
	@install -m 644 lz4hc.h $(DESTDIR)$(INCLUDEDIR)/lz4hc.h
	@echo lz4 static and shared library installed
	@cd $(PRGDIR); $(MAKE) -e install

uninstall:
	rm -f $(DESTDIR)$(LIBDIR)/liblz4.$(SHARED_EXT)
	rm -f $(DESTDIR)$(LIBDIR)/liblz4.$(SHARED_EXT_MAJOR)
	rm -f $(DESTDIR)$(LIBDIR)/pkgconfig/liblz4.pc
	[ -x $(DESTDIR)$(LIBDIR)/liblz4.$(SHARED_EXT_VER) ] && rm -f $(DESTDIR)$(LIBDIR)/liblz4.$(SHARED_EXT_VER)
	[ -f $(DESTDIR)$(LIBDIR)/liblz4_linux.a ] && rm -f $(DESTDIR)$(LIBDIR)/liblz4_linux.a
	[ -f $(DESTDIR)$(INCLUDEDIR)/lz4.h ] && rm -f $(DESTDIR)$(INCLUDEDIR)/lz4.h
	[ -f $(DESTDIR)$(INCLUDEDIR)/lz4hc.h ] && rm -f $(DESTDIR)$(INCLUDEDIR)/lz4hc.h
	@echo lz4 libraries successfully uninstalled
	@cd $(PRGDIR); $(MAKE) uninstall

dist: clean
	@install -dD -m 700 lz4-$(RELEASE)/cmake_unofficial/
	@for f in $(TEXT); do \
		tr -d '\r' < $$f > .tmp; \
		install -m 600 .tmp lz4-$(RELEASE)/$$f; \
	done
	@rm .tmp
	@for f in $(NONTEXT); do \
		install -m 600 $$f lz4-$(RELEASE)/$$f; \
	done
	@tar -czf $(DISTRIBNAME) lz4-$(RELEASE)/
	@rm -rf lz4-$(RELEASE)
	@sha1sum $(DISTRIBNAME) > $(DISTRIBNAME).sha1
	@echo Distribution $(DISTRIBNAME) built

endif
