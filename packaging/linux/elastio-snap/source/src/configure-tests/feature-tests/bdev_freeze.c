// SPDX-License-Identifier: GPL-2.0-only

/*
 * Copyright (C) 2024 Axcient Inc.
 */

#include "includes.h"
MODULE_LICENSE("GPL");

static inline void dummy(void){
	int ret;
	struct block_device *bdev;

	ret = bdev_freeze(bdev);
}
