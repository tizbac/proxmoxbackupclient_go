# SPDX-License-Identifier: GPL-2.0-only

SELF_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
include $(SELF_DIR)/../../prehistoric_gcc.mk

SRC=$(TEST_NAME).c
OBJ=$(TEST_NAME).o

obj-m :=$(OBJ)

KERNELVERSION ?= $(shell uname -r)
KDIR := /lib/modules/$(KERNELVERSION)/build
PWD := $(shell pwd)
EXTRA_CFLAGS := -g -Werror -I$(src)/../..
BUILDDIR ?= $(PWD)/build/$(OBJ)
BUILDDIR_MAKEFILE ?= $(PWD)/build/$(OBJ)/Makefile
BUILDDIR_SOURCE ?= $(PWD)/build/$(OBJ)/$(SRC)


ccflags-y += $(EXTRA_CFLAGS)

default: $(BUILDDIR_MAKEFILE)
	$(MAKE) -C $(KDIR) M=$(BUILDDIR) src=$(PWD) modules

$(BUILDDIR):
	mkdir -p "$@"

$(BUILDDIR_MAKEFILE): $(BUILDDIR)
	touch "$@"
	ln -sf $(PWD)/$(SRC) $(BUILDDIR_SOURCE)

clean: $(BUILDDIR)
	$(MAKE) -C $(KDIR) M=$(BUILDDIR) src=$(PWD) clean
