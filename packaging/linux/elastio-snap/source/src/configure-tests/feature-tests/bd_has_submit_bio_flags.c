// SPDX-License-Identifier: GPL-2.0-only

/*
 * Copyright (C) 2024 Axcient Inc.
 */

#include "includes.h"
MODULE_LICENSE("GPL");

static inline void dummy(void){
	struct block_device *bdev;
	bdev_set_flag(bdev, BD_HAS_SUBMIT_BIO);
}
