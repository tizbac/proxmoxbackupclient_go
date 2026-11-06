// SPDX-License-Identifier: GPL-2.0-only

/*
 * Copyright (C) 2024 Axcient Inc.
 */

// kernel_version < 5.0

#include "includes.h"
MODULE_LICENSE("GPL");

static inline void dummy(void){
	struct request_queue *q;
	blk_stop_queue(q);
}

