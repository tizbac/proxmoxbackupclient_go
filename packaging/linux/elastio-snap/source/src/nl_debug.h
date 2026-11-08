#ifdef KERNEL_MODULE

#include <net/sock.h>
#include <linux/netlink.h>
#include <linux/skbuff.h>

#endif

#include "kernel-config.h"
#include "elastio-snap.h"

#define NL_MCAST_GROUP 1
#define NL_TO_STR(_type) #_type

enum nl_msg_type {
	NL_EVENT_DRIVER_INIT = NLMSG_MIN_TYPE,
	NL_EVENT_DRIVER_DEINIT,
	NL_EVENT_DRIVER_ERROR,
	NL_EVENT_SETUP_SNAPSHOT,
	NL_EVENT_SETUP_UNVERIFIED_SNAP,
	NL_EVENT_SETUP_UNVERIFIED_INC,
	NL_EVENT_TRANSITION_INC,
	NL_EVENT_TRANSITION_SNAP,
	NL_EVENT_TRANSITION_DORMANT,
	NL_EVENT_TRANSITION_ACTIVE,
	NL_EVENT_TRACING_STARTED,
	NL_EVENT_TRACING_FINISHED,
	NL_EVENT_BIO_INCOMING_TRACING_MRF,
	NL_EVENT_BIO_INCOMING_SNAP_MRF,
	NL_EVENT_BIO_CALL_ORIG,
	NL_EVENT_BIO_SNAP,
	NL_EVENT_BIO_INC,
	NL_EVENT_BIO_CLONED,
	NL_EVENT_BIO_READ_COMPLETE,
	NL_EVENT_BIO_QUEUED,		// cloned bio enqueued for the cow thread
	NL_EVENT_BIO_RELEASED,		// parent bio released
	NL_EVENT_BIO_HANDLE_READ_BASE,
	NL_EVENT_BIO_HANDLE_READ_COW,
	NL_EVENT_BIO_HANDLE_READ_DONE,
	NL_EVENT_BIO_HANDLE_WRITE,
	NL_EVENT_BIO_HANDLE_WRITE_DONE,
	NL_EVENT_BIO_FREE,
	NL_EVENT_COW_READ_MAPPING,
	NL_EVENT_COW_WRITE_MAPPING,
	NL_EVENT_COW_READ_DATA,
	NL_EVENT_COW_WRITE_DATA,
	NL_EVENT_LAST
};

struct nl_params {
	uint64_t id;
	uint32_t size;	// in sectors
	uint64_t sector;
	uint8_t  flags;
	uint64_t priv1;
	uint64_t priv2;
} __attribute__((packed));

struct nl_code_info {
	char	 func[32];
	uint16_t line;
} __attribute__((packed));

struct nl_msg_header {
	uint8_t  type;
	uint64_t seq_num;
	uint64_t timestamp;

	struct nl_params	params;
	struct nl_code_info	source;
} __attribute__((packed));


#ifdef KERNEL_MODULE

#define nl_trace_event_bio(_type, _bio, _priv)			\
({								\
	struct nl_params params = { 0 };			\
								\
	if (_bio) { 						\
		params.id	= (uint64_t)(_bio); 		\
		params.size	= bio_size(_bio); 		\
		params.flags	= bio_data_dir(_bio);		\
		params.sector	= bio_sector(_bio); 		\
	} 							\
								\
	params.priv1 = (_priv); 				\
	params.priv2 = 0;					\
	nl_send_event(_type, __func__, __LINE__, &params);	\
})

#define nl_trace_event_generic(_type, _priv)			\
({ 								\
	struct nl_params params = { 0 };			\
								\
	params.priv1 = (_priv); 				\
	params.priv2 = 0; 					\
	nl_send_event(_type, __func__, __LINE__, &params);	\
})

#define nl_trace_event_cow(_type, _priv1, _priv2)		\
({ 								\
	struct nl_params params = { 0 }; 			\
								\
	params.priv1 = (_priv1); 				\
	params.priv2 = (_priv2); 				\
	nl_send_event(_type, __func__, __LINE__, &params);	\
})

int nl_send_event(enum nl_msg_type type, const char *func, int line, struct nl_params *params);
int nl_debug_init(void);
void nl_debug_release(void);

#endif
