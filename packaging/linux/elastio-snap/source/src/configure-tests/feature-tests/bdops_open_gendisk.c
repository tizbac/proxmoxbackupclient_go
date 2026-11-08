// SPDX-License-Identifier: GPL-2.0-only

/*
 * Copyright (C) 2024 Axcient Inc.
 */

#include "includes.h"
MODULE_LICENSE("GPL");

static int snap_open(struct gendisk *gd, blk_mode_t mode){
	return 0;
}

static void snap_release(struct gendisk *gd){
}

static inline void dummy(void){
	struct gendisk gd;
	struct block_device_operations bdo = {
		.open = snap_open,
		.release = snap_release,
	};

	bdo.open(&gd, FMODE_READ);
	bdo.release(&gd);
}
