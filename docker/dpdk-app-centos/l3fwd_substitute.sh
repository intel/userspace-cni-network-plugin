#!/bin/bash

sed -i -e '/#include "l3fwd.h"/a #include "dpdk-args.h"' main.c

sed -i -e 's!.offloads = DEV_RX_OFFLOAD_CHECKSUM,!.offloads = 0, /*DEV_RX_OFFLOAD_CHECKSUM,*/!' main.c

sed -i '/ret = rte_eal_init(argc, argv);/{
:a;N;/argv += ret;/!ba;N;s/.*\n//g
r l3fwd_eal_init.txt
}' main.c

sed -i '/ret = parse_args(argc, argv);/{
s/ret = parse_args(argc, argv);//g
r l3fwd_parse_args.txt
}' main.c

sed -i -e '/SRCS-y :=/a SRCS-y += dpdk-args.c' Makefile
sed -i -e '/SRCS-y += dpdk-args.c/a LDLIBS += -lnetutil_api' Makefile
