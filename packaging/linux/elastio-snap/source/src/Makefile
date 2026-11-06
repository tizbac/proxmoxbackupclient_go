# SPDX-License-Identifier: GPL-2.0-only

SELF_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
include $(SELF_DIR)/prehistoric_gcc.mk

obj-m := elastio-snap.o
elastio-snap-objs:= main.o

KVER=$(shell uname -r)
KMAJ = $(shell echo $(KVER) | \
sed -e 's/^\([0-9][0-9]*\)\.[0-9][0-9]*\.[0-9][0-9]*.*/\1/')

ifeq ($(shell test $(KMAJ) -gt 3; echo $$?),0)
NETLINK_DEBUG ?= y
endif

ifeq ($(NETLINK_DEBUG),y)
elastio-snap-objs += nl_debug.o
endif

KERNELVERSION ?= $(shell uname -r)
KDIR := /lib/modules/$(KERNELVERSION)/build
PWD := $(shell pwd)
INSTALL_MOD_DIR ?= extra
EXTRA_CFLAGS := -g -DKERNEL_MODULE

ifeq ($(NETLINK_DEBUG),y)
EXTRA_CFLAGS += -DNETLINK_DEBUG
endif

ccflags-y += $(EXTRA_CFLAGS)

FEATURE_TEST_BUILD_DIR := configure-tests/feature-tests/build

default:
	if [ ! -f kernel-config.h ] || tail -1 kernel-config.h | grep -qv '#endif'; then mkdir $(FEATURE_TEST_BUILD_DIR); ./genconfig.sh "$(KERNELVERSION)" "$(MFLAGS)"; fi;
	$(MAKE) -C $(KDIR) M=$(PWD) modules

clean:
	$(MAKE) -C $(KDIR) M=$(PWD) clean
	$(RM) kernel-config.h
	$(RM) -r $(FEATURE_TEST_BUILD_DIR)

install: default
	$(MAKE) -C $(KDIR) M=$(PWD) modules_install

uninstall:
	$(RM) /lib/modules/$(KERNELVERSION)/$(INSTALL_MOD_DIR)/elastio-snap.ko
