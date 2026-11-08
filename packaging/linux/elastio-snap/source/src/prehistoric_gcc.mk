ifeq ($(COMPAT_OLD_GCC),y)
UNSUPPORTED_OLD_GCC_FLAGS := \
	-fzero-call-used-regs=% \
	-ftrivial-auto-var-init=% \
	-Wno-alloc-size-larger-than \
	-pg
KBUILD_CFLAGS := $(filter-out $(UNSUPPORTED_OLD_GCC_FLAGS),$(KBUILD_CFLAGS))
KBUILD_CFLAGS += -Wno-pragmas -Wno-error=stringop-truncation
endif
