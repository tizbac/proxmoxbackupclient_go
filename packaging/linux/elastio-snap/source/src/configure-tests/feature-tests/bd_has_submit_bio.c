// SPDX-License-Identifier: GPL-2.0-only

/*
 * Copyright (C) 2024 Axcient Inc.
 */

#include "includes.h"
MODULE_LICENSE("GPL");

static inline void dummy(void){
	struct block_device *bdev;
	bdev->bd_has_submit_bio = false;
}
