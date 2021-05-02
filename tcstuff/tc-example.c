// Build with: clang-9 -g -O2 -Wall -target bpf -c tc-example.c -o tc-example.o
// Install with:
//  tc qdisc add dev tap0 clsact
//  tc filter add dev tap0 ingress bpf da obj tc-example.o sec ingress
#include <linux/bpf.h>
#include <linux/pkt_cls.h>
#include <iproute2/bpf_elf.h>
#include <linux/if_ether.h>
#include <linux/swab.h>
//#include <linux/ip.h>
//#include <linux/in.h>

// From <linux/ip.h> - but without need for asm/byteorder.h
struct iphdr {
	__u8	ilhversion;
	__u8	tos;
	__u16	tot_len;
	__u16	id;
	__u16	frag_off;
	__u8	ttl;
	__u8	protocol;
	__u16	check;
	__u32	saddr;
	__u32	daddr;
};


/*typedef unsigned short __u16;  // NOLINT
typedef unsigned char __u8;
typedef unsigned int __u32;
typedef unsigned long long __u64;
typedef int __s32;
typedef unsigned long size_t;
typedef __u32 __be32;
typedef __u16 __be16;*/

#define htons(x) ((__be16)___constant_swab16((x)))
#define swaplong(x) ((__be32)___constant_swab32((x)))

#ifndef __section
# define __section(NAME)                  \
	__attribute__((section(NAME), used))
#endif

#ifndef __inline
# define __inline                         \
        inline __attribute__((always_inline))
#endif

#ifndef lock_xadd
# define lock_xadd(ptr, val)              \
        ((void)__sync_fetch_and_add(ptr, val))
#endif

#ifndef BPF_FUNC
# define BPF_FUNC(NAME, ...)              \
        (*NAME)(__VA_ARGS__) = (void *)BPF_FUNC_##NAME
#endif

static void *BPF_FUNC(map_lookup_elem, void *map, const void *key);

static void BPF_FUNC(trace_printk, const char *fmt, int fmt_size, ...);

#ifndef printk
# define printk(fmt, ...)                                      \
    ({                                                         \
        char ____fmt[] = fmt;                                  \
        trace_printk(____fmt, sizeof(____fmt), ##__VA_ARGS__); \
    })
#endif


struct bpf_elf_map ifce_allowed_macs __section("maps") = {
        .type           = BPF_MAP_TYPE_HASH,
        .size_key       = sizeof(__u32), // ifindex
        .size_value     = sizeof(__u64),
        .pinning        = PIN_GLOBAL_NS,
        .max_elem       = 2,
};

struct bpf_elf_map ifce_allowed_ip __section("maps") = {
        .type           = BPF_MAP_TYPE_HASH,
        .size_key       = sizeof(__u32), // ifindex
        .size_value     = sizeof(__u32), // an ipv4 addr
        .pinning        = PIN_GLOBAL_NS,
        .max_elem       = 2,
};

// Technically, ARP is variable-length since you can run it over anything, not just IPv4 over Ethernet.
// Our VMs are restricted to just IPv4 over Ethernet though... So we can simplify it as such.
struct arppkt {
   __u16 ar_hrd;
   __u16 ar_pro;
   __u8 ar_hln;
   __u8 ar_pln;
   __u16 ar_op;
   __u8 ar_sha[6];
   __u8 ar_spa[4];
};

__section("ingress")
int tc_ingress(struct __sk_buff *skb)
{
        __u32 ifindex = skb->ifindex;
        __u64 *allowedmac;
        __u32 *allowedip;
        allowedmac = map_lookup_elem(&ifce_allowed_macs, &ifindex);
        if (!allowedmac) {
                // We were attached to an interface but this interface isn't represented in the map.
                // Drop this packet - we shouldn't risk processing it incorrectly.
                //printk("failed to lookup for ifindex: %u", ifindex);
                return TC_ACT_SHOT;
        }

        allowedip = map_lookup_elem(&ifce_allowed_ip, &ifindex);
        if (!allowedip) {
                return TC_ACT_SHOT;
        }

        void *data = (void *)(long)skb->data;
	void *data_end = (void *)(long)skb->data_end;
        if (data + sizeof(struct ethhdr) > data_end) {
                // Weirdly small packet - not even an ethhdr. Must be something pulling funny business.
                // Drop it!
                //printk("pkt too small");
		return TC_ACT_SHOT;
        }

        struct ethhdr *ether  = data;

        __u64 macAs64 = ((__u64)0) | ((__u64)ether->h_source[0]) << 40 |
                  ((__u64)ether->h_source[1]) << 32 |
                  ((__u64)ether->h_source[2]) << 24 |
                  ((__u64)ether->h_source[3]) << 16 |
                  ((__u64)ether->h_source[4]) << 8 |
                  ((__u64)ether->h_source[5]) << 0;

        if (*allowedmac != macAs64) {
                //printk("Disallowed mac: %llx %llx", *allowedmac, macAs64);
                return TC_ACT_SHOT;
        }
        
        if (ether->h_proto == htons(0x0800)) {
                if (data + sizeof(struct ethhdr) + sizeof(struct iphdr) > data_end) {
                        // Weirdly small packet - ETH_P_IP but not large enough for an iphdr.
                        //printk("pkt too small");
                        return TC_ACT_SHOT;
                }
                struct iphdr *ip   = (data + sizeof(struct ethhdr));
                if (*allowedip == ip->saddr) {
                        return TC_ACT_OK;
                }
        } else if (ether->h_proto == htons(0x0806)) {
                if (data + sizeof(struct ethhdr) + sizeof(struct arppkt) > data_end) {
                        // Too small for a real ARP. Throw it away.
                        return TC_ACT_SHOT;
                }

                struct arppkt *arp  = (data + sizeof(struct ethhdr));
                // Arp HTYPE must be Ethernet.
                if (arp->ar_hrd != htons(1)) {
                        return TC_ACT_SHOT;
                }
                // Arp PTYPE must be IP (0x8000)
                if (arp->ar_pro != htons(0x0800)) {
                        return TC_ACT_SHOT;
                }
                
                // hlen == 6 bytes
                if (arp->ar_hln != 6) {
                        return TC_ACT_SHOT;
                }
                // plen == 4 bytes
                if (arp->ar_pln != 4) {
                        return TC_ACT_SHOT;
                }
                
                // Operation must be 1 (request) or 2 (reply)
                if (arp->ar_op != htons(1) && arp->ar_op != htons(2)) {
                        // Not an ARP request or reply. Garbage.
                        return TC_ACT_SHOT;
                }

                __u64 shaAs64 = ((__u64)0) | ((__u64)arp->ar_sha[0]) << 40 |
                  ((__u64)arp->ar_sha[1]) << 32 |
                  ((__u64)arp->ar_sha[2]) << 24 |
                  ((__u64)arp->ar_sha[3]) << 16 |
                  ((__u64)arp->ar_sha[4]) << 8 |
                  ((__u64)arp->ar_sha[5]) << 0;

                __u32 spaAs32 = ((__u32)0) | ((__u32)arp->ar_spa[3]) << 24 |
                        ((__u32)arp->ar_spa[2]) << 16 |
                        ((__u32)arp->ar_spa[1]) << 8 |
                        ((__u32)arp->ar_spa[0]) << 0;
                
                if ((*allowedmac == shaAs64) && (*allowedip == spaAs32)) {
                        return TC_ACT_OK;
                }
        }

        return TC_ACT_SHOT;
}


char __license[] __section("license") = "GPL";
