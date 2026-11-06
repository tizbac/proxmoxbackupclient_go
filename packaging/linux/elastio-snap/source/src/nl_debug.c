#include "nl_debug.h"

static uint64_t seq_num = 1;
struct sock *nl_sock = NULL;
spinlock_t nl_spinlock;

static void nl_recv_msg(struct sk_buff *skb)
{
	nlmsg_free(skb);
}


int nl_send_event(enum nl_msg_type type, const char *func, int line, struct nl_params *params)
{
	struct sk_buff *skb;
	struct nl_msg_header *msg;
	struct nlmsghdr *nlsk_mh;
	struct timespec64 tspec;

	skb = nlmsg_new(NLMSG_DEFAULT_SIZE, GFP_ATOMIC);
	nlsk_mh = nlmsg_put(skb, 0, 0, NLMSG_DONE, sizeof(struct nl_msg_header), 0);
	NETLINK_CB(skb).portid = 0;
	NETLINK_CB(skb).dst_group = NL_MCAST_GROUP;

	msg = nlmsg_data(nlsk_mh);
	msg->type = type;
	ktime_get_ts64(&tspec);
	msg->timestamp = timespec64_to_ns(&tspec);
	msg->seq_num = seq_num;
	seq_num++;

	if (func) {
		msg->source.line = line;
		strncpy(msg->source.func, func, sizeof(msg->source.func));
	}

	memcpy(&msg->params, params, sizeof(*params));

	nlmsg_multicast(nl_sock, skb, 0, NL_MCAST_GROUP, GFP_ATOMIC);
	return 0;
}

int nl_debug_init(void)
{
	struct netlink_kernel_cfg cfg = {
		.input = nl_recv_msg,
	};

	printk("netlink init\n");
	spin_lock_init(&nl_spinlock);

	nl_sock = netlink_kernel_create(&init_net, NETLINK_USERSOCK, &cfg);
	if (!nl_sock) {
		printk("netlink: error creating socket\n");
		return -ENOTSUPP;
	}

	return 0;
}

void nl_debug_release(void)
{
	printk("netlink release\n");
	sock_release(nl_sock->sk_socket);
}
