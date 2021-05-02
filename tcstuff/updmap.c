#include <linux/bpf.h>
#include <stdio.h>
#include <errno.h>
#include <memory.h>
#include <stdlib.h>
#include <linux/swab.h>
#include <sys/syscall.h>
#include <unistd.h>
#include <arpa/inet.h>

static inline __u64 ptr_to_u64(const void* ptr) {
    return (__u64) (unsigned long) ptr;
}

struct bpf_map_detail {
    unsigned int type;
    unsigned int size_key;
    unsigned int size_value;
    unsigned int max_elem;
    unsigned int flags;
    unsigned int owner_type;
    unsigned int owner_jited;
};


static int get_bpf_info_from_fd(int fd, struct bpf_map_detail *map)
{
	unsigned int val, owner_type = 0, owner_jited = 0;
	char file[1024], buff[4096];
	FILE *fp;

	snprintf(file, sizeof(file), "/proc/%d/fdinfo/%d", getpid(), fd);
	memset(map, 0, sizeof(*map));

	fp = fopen(file, "r");
	if (!fp) {
		fprintf(stderr, "No procfs support?!\n");
		return -EIO;
	}

	while (fgets(buff, sizeof(buff), fp)) {
        printf("line: %s", buff);
		if (sscanf(buff, "map_type:\t%u", &val) == 1)
			map->type = val;
		else if (sscanf(buff, "key_size:\t%u", &val) == 1)
			map->size_key = val;
		else if (sscanf(buff, "value_size:\t%u", &val) == 1)
			map->size_value = val;
		else if (sscanf(buff, "max_entries:\t%u", &val) == 1)
			map->max_elem = val;
		else if (sscanf(buff, "map_flags:\t%i", &val) == 1)
			map->flags = val;
		else if (sscanf(buff, "owner_prog_type:\t%i", &val) == 1)
			map->owner_type = val;
		else if (sscanf(buff, "owner_jited:\t%i", &val) == 1)
			map->owner_jited = val;
	}

    

	fclose(fp);
	return 0;
}

// Inexplicably (kinda explicably - tied closely to kernel version), glibc doesn't wrap these.

// Wrapper around BPF_OBJ_GET syscall.
// Get an FD from a pinned pathname
int sc_bpf_obj_get(char* pathname) {
    union bpf_attr attr = {};
	attr.pathname = ptr_to_u64(pathname);

	return syscall(__NR_bpf, BPF_OBJ_GET, &attr, sizeof(attr));
}

// Wrapper around BPF_MAP_UPDATE_ELEM.
// flags should be BPF_ANY, BPF_NOEXIST, BPF_EXIST
int sc_bpf_update_elem(int fd, const void *key, const void *value, __u64 flags) {
    union bpf_attr attr = {
        .map_fd = fd,
        .key    = ptr_to_u64(key),
        .value  = ptr_to_u64(value),
        .flags  = flags,
    };

    return syscall(__NR_bpf, BPF_MAP_UPDATE_ELEM, &attr, sizeof(attr));
}

int sc_bpf_get_next_key(int fd, const void *key, void *next_key) {
    union bpf_attr attr = {
        .map_fd   = fd,
        .key      = ptr_to_u64(key),
        .next_key = ptr_to_u64(next_key),
    };

    return syscall(__NR_bpf, BPF_MAP_GET_NEXT_KEY, &attr, sizeof(attr));
}

int sc_bpf_lookup_elem(int fd, const void *key, void *value) {
    union bpf_attr attr = {
        .map_fd = fd,
        .key    = ptr_to_u64(key),
        .value  = ptr_to_u64(value),
    };

    return syscall(__NR_bpf, BPF_MAP_LOOKUP_ELEM, &attr, sizeof(attr));
}

int sc_bpf_delete_elem(int fd, const void *key) {
    union bpf_attr attr = {
        .map_fd = fd,
        .key    = ptr_to_u64(key),
    };

    return syscall(__NR_bpf, BPF_MAP_DELETE_ELEM, &attr, sizeof(attr));
}

int main() {
    // BPF_OBJ_GET to get the FD
    // 

    int bpfFd = sc_bpf_obj_get("/sys/fs/bpf/tc/globals/ifce_allowed_macs");
    if (bpfFd == -1) {
        printf("BPF failed to get obj: %s\n", strerror(errno));
        return -1;
    }
    printf("Got BPF FD: %d\n", bpfFd);

    struct bpf_map_detail info = {};
	int err = get_bpf_info_from_fd(bpfFd, &info);
    if (err == -1) {
        printf("BPF info failed: %s\n", strerror(errno));
        return -1;
    }

    printf("Current entries: \n");
    __u32 key = 0;
    __u32 nextKey = 0;
    int rc = sc_bpf_get_next_key(bpfFd, &key, &nextKey);
    key = nextKey;
    while (rc == 0) {
        __u64 val = 0;
        int ret = sc_bpf_lookup_elem(bpfFd, &key, &val);
        if (ret != 0) {
            printf("Failed to look up an element from GET_NEXT_KEY... concurrency gremlins... %s\n", strerror(errno));
        }
        printf("\t%lu: %llx\n", key, val);
        rc = sc_bpf_get_next_key(bpfFd, &key, &nextKey);
        if (rc != 0) {
            break;
        }
        key = nextKey;
    } 
    if (errno != ENOENT) {
        printf("Failed to iterate over current entries...: %s\n", strerror(errno));
    }
    printf("Done listing entries. \n");

    key = 16;
    __u64 val = 0xaafc00000001;
    rc = sc_bpf_update_elem(bpfFd, &key, &val, BPF_ANY);
    if (rc != 0) {
        printf("Failed to set element%s\n", strerror(errno));
    }
    // then
    // BPF_MAP_UPDATE_ELEM to set an entry in the map.
    // BPF_MAP_GET_NEXT_KEY & BPF_MAP_LOOKUP_ELEM to iterate over map.
    // BPF_MAP_DELETE_ELEM to remove.

    int bpfFd2 = sc_bpf_obj_get("/sys/fs/bpf/tc/globals/ifce_allowed_ip");
    if (bpfFd2 == -1) {
        printf("BPF failed to get obj: %s\n", strerror(errno));
        return -1;
    }
    printf("Got BPF FD2: %d\n", bpfFd2);

    struct in_addr resAddr = {0};
    rc = inet_pton(AF_INET, "172.19.0.2", &resAddr);
    if (rc != 1) {
        printf("Failed to convert...?");
        return -1;
    }
    printf("Res: %lx / %lx / %lu\n", resAddr.s_addr, ___constant_swab32(resAddr.s_addr), resAddr.s_addr);

    __u32 ipVal = resAddr.s_addr;// 172.19.0.2
    rc = sc_bpf_update_elem(bpfFd2, &key, &ipVal, BPF_ANY);
    if (rc != 0) {
        printf("Failed to set element in IP map %s\n", strerror(errno));
    }
    printf("Updated IP map.\n");
    printf("Current entries: \n");
    key = 0;
    nextKey = 0;
    rc = sc_bpf_get_next_key(bpfFd2, &key, &nextKey);
    key = nextKey;
    while (rc == 0) {
        __u32 val = 0;
        int ret = sc_bpf_lookup_elem(bpfFd2, &key, &val);
        if (ret != 0) {
            printf("Failed to look up an element from GET_NEXT_KEY... concurrency gremlins... %s\n", strerror(errno));
        }
        printf("\t%lu: %lx/%ld\n", key, val, val);
        rc = sc_bpf_get_next_key(bpfFd2, &key, &nextKey);
        if (rc != 0) {
            break;
        }
        key = nextKey;
    } 
    if (errno != ENOENT) {
        printf("Failed to iterate over current entries...: %s\n", strerror(errno));
    }
    printf("Done listing entries. \n");
}
