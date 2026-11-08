// SPDX-License-Identifier: GPL-2.0-only

/*
 * Copyright (C) 2025 Axcient Software Inc.
 */

// kernel_version >= 6.16

#include "includes.h"
MODULE_LICENSE("GPL");

static inline void dummy(void){
    struct super_block *sb;
	freeze_super(sb, 0, NULL);
}
