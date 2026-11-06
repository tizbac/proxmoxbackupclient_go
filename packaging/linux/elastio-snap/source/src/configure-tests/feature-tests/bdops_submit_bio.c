// SPDX-License-Identifier: GPL-2.0-only

/*
 * Copyright (C) 2020 Elastio Software Inc.
 */

// kernel_version >= 5.16

#include "includes.h"
MODULE_LICENSE("GPL");

static void snap_submit_bio(struct bio *bio)
{
	(void)bio;
}

static inline void dummy(void){
	struct bio b;
	struct block_device_operations bdo = {
		.submit_bio = snap_submit_bio,
	};

	bdo.submit_bio(&b);
}
