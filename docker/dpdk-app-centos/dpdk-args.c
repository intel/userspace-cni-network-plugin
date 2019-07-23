/* SPDX-License-Identifier: BSD-3-Clause
 * Copyright(c) 2019 Red Hat
 */

#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <signal.h>
#include <string.h>
#include <dirent.h>

#define DPDK_ARGS_MAX_ARGS (30)
#define DPDK_ARGS_MAX_ARG_STRLEN (100)
char myArgsArray[DPDK_ARGS_MAX_ARGS][DPDK_ARGS_MAX_ARG_STRLEN];
char* myArgv[DPDK_ARGS_MAX_ARGS];

extern char** GetArgs(int *pArgc);

static int getInterfaces(int argc) {
    DIR *d;
    struct dirent *dir;
    int i = 0;

    d = opendir("/var/lib/cni/usrspcni/");
    if (d)
    {
        while ((dir = readdir(d)) != NULL)
        {
        	if ((dir->d_name) && (dir->d_name[0] != '.')) {
	            snprintf(&myArgsArray[argc++][0], DPDK_ARGS_MAX_ARG_STRLEN-1,
    	        	"--vdev=virtio_user%d,path=/var/lib/cni/usrspcni/%s", i, dir->d_name);
	            i++;
	        }
        }
        closedir(d);
    }

    return(argc);
}

char** GetArgs(int *pArgc)
{
	int argc = 0;
	int i;

	memset(&myArgsArray[0][0], 0, sizeof(char)*DPDK_ARGS_MAX_ARG_STRLEN*DPDK_ARGS_MAX_ARGS);
	memset(&myArgv[0], 0, sizeof(char)*DPDK_ARGS_MAX_ARGS);

	if (pArgc) {
		strncpy(&myArgsArray[argc++][0], "dpdk-app", DPDK_ARGS_MAX_ARG_STRLEN-1);

		strncpy(&myArgsArray[argc++][0], "-m", DPDK_ARGS_MAX_ARG_STRLEN-1);
		strncpy(&myArgsArray[argc++][0], "1024", DPDK_ARGS_MAX_ARG_STRLEN-1);

		strncpy(&myArgsArray[argc++][0], "-l", DPDK_ARGS_MAX_ARG_STRLEN-1);
		strncpy(&myArgsArray[argc++][0], "1-3", DPDK_ARGS_MAX_ARG_STRLEN-1);

		strncpy(&myArgsArray[argc++][0], "--master-lcore", DPDK_ARGS_MAX_ARG_STRLEN-1);
		strncpy(&myArgsArray[argc++][0], "1", DPDK_ARGS_MAX_ARG_STRLEN-1);

		strncpy(&myArgsArray[argc++][0], "-n", DPDK_ARGS_MAX_ARG_STRLEN-1);
		strncpy(&myArgsArray[argc++][0], "4", DPDK_ARGS_MAX_ARG_STRLEN-1);

		//strncpy(&myArgsArray[argc++][0], "--file-prefix=dpdk-app_", DPDK_ARGS_MAX_ARG_STRLEN-1);

		argc = getInterfaces(argc);

		strncpy(&myArgsArray[argc++][0], "--no-pci", DPDK_ARGS_MAX_ARG_STRLEN-1);

		strncpy(&myArgsArray[argc++][0], "--", DPDK_ARGS_MAX_ARG_STRLEN-1);

		strncpy(&myArgsArray[argc++][0], "--auto-start", DPDK_ARGS_MAX_ARG_STRLEN-1);
		strncpy(&myArgsArray[argc++][0], "--tx-first", DPDK_ARGS_MAX_ARG_STRLEN-1);
		strncpy(&myArgsArray[argc++][0], "--no-lsc-interrupt", DPDK_ARGS_MAX_ARG_STRLEN-1);

		strncpy(&myArgsArray[argc++][0], "--stats-period", DPDK_ARGS_MAX_ARG_STRLEN-1);
		strncpy(&myArgsArray[argc++][0], "60", DPDK_ARGS_MAX_ARG_STRLEN-1);

		for (i = 0; i < argc; i++) {
			myArgv[i] = &myArgsArray[i][0];
		}
		*pArgc = argc;
	}

	return(myArgv);
}