// SPDX-License-Identifier: GPL-2.0-only

/*
 * Copyright (C) 2024 Axcient Inc.
 */

#include "includes.h"
MODULE_LICENSE("GPL");

static inline void dummy(void){
	struct gendisk *gd;
	gd = blk_alloc_disk(NULL, NUMA_NO_NODE);
}
