/* SPDX-License-Identifier: BSD-3-Clause
 * Copyright(c) 2019 Red Hat
 */

#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <signal.h>
#include <string.h>
#include <dirent.h>
#include <sys/stat.h>

#define DPDK_ARGS_MAX_ARGS (30)
#define DPDK_ARGS_MAX_ARG_STRLEN (100)
char myArgsArray[DPDK_ARGS_MAX_ARGS][DPDK_ARGS_MAX_ARG_STRLEN];
char* myArgv[DPDK_ARGS_MAX_ARGS];

#define DPDK_ARGS_MAX_NUM_DIR (30)
static const char DEFAULT_DIR[] = "/var/lib/cni/";

extern char** GetArgs(int *pArgc);


static int getInterfaces(int argc) {
	DIR *d;
	struct dirent *dir;
	int i = 0;
	int currIndex = 0;
	int freeIndex = 0;
	char* dirList[DPDK_ARGS_MAX_NUM_DIR];
	char* fileExt;

	memset(dirList, 0, sizeof(char*)*DPDK_ARGS_MAX_NUM_DIR);

	dirList[freeIndex] = malloc(sizeof(char) * (strlen(DEFAULT_DIR)+1));
	strcpy(dirList[freeIndex++], DEFAULT_DIR);

	while (dirList[currIndex] != NULL) {
		printf("  Directory:%s\n", dirList[currIndex]);
		d = opendir(dirList[currIndex]);
		if (d)
		{
			while ((dir = readdir(d)) != NULL)
			{
				if ((dir->d_name) &&
					(strcmp(dir->d_name, ".") != 0) &&
					(strcmp(dir->d_name, "..") != 0))
				{
					printf("  Name:%s %d\n", dir->d_name, dir->d_type);
					if (dir->d_type == DT_DIR) {
						printf("    Add to Dir List:%s\n", dir->d_name);
						dirList[freeIndex] = malloc(sizeof(char) * (strlen(DEFAULT_DIR)+strlen(dir->d_name)+1));
						sprintf(dirList[freeIndex++], "%s%s/", DEFAULT_DIR, dir->d_name);
					}
					else
					{
						if (strstr(dir->d_name, "net") != NULL)
						{
							fileExt = strrchr(dir->d_name, '.');
							if ((fileExt == NULL) || (strcmp(fileExt, ".json") != 0)) {
								printf("    Adding to vdev list:%s\n", dir->d_name);
								snprintf(&myArgsArray[argc++][0], DPDK_ARGS_MAX_ARG_STRLEN-1,
										 "--vdev=virtio_user%d,path=%s%s", i, dirList[currIndex], dir->d_name);
								i++;
							}
							else {
								printf("    Invalid FileExt\n");
							}
						}
					}
				}
			}
			closedir(d);
		}
		free(dirList[currIndex]);
		currIndex++;
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