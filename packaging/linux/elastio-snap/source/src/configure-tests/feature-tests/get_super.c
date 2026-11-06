// SPDX-License-Identifier: GPL-2.0-only

/*
 * Copyright (C) 2024 Axcient Inc.
 */

// kernel_version < 6.7

#include "includes.h"
MODULE_LICENSE("GPL");

static inline void dummy(void){
	struct block_device *bdev = NULL;
	get_super(bdev);
}
