// SPDX-License-Identifier: GPL-2.0-only

/*
 * Copyright (C) 2024 Axcient Inc.
 */

#include "includes.h"
MODULE_LICENSE("GPL");

static inline void dummy(void){
	struct bdev_handle *bd;
	bd = bdev_open_by_path("path", 0, NULL, NULL);
}
