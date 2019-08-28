#!/bin/bash

sed -i -e '/#include "testpmd.h"/a #include "dpdk-args.h"' testpmd.c

sed -i '/diag = rte_eal_init(argc, argv);/{
s/diag = rte_eal_init(argc, argv);//g
r testpmd_eal_init.txt
}' testpmd.c

sed -i '/argc -= diag;/{
:a;N;/launch_args_parse(argc, argv);/!ba;N;s/.*\n//g
r testpmd_launch_args_parse.txt
}' testpmd.c

sed -i -e 's/SRCS-y += parameters.c/SRCS-y += parameters.c dpdk-args.c/' Makefile

sed -i -e '/SRCS-y += util.c/a LDLIBS += -lnetutil_api' Makefile

