// SPDX-License-Identifier: GPL-2.0-only

/*
 * Copyright (C) 2024 Axcient Inc.
 */

#include "includes.h"
MODULE_LICENSE("GPL");

static inline void dummy(void){
	struct file *bdev_file;
	bdev_file = bdev_file_open_by_path("path", 0, NULL, NULL);
}
