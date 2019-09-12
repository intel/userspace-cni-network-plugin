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
#include <unistd.h>
#include "libnetutil_api.h"
#include "dpdk-args.h"

bool debugArgs = true;

#define DPDK_ARGS_MAX_ARGS (30)
#define DPDK_ARGS_MAX_ARG_STRLEN (100)
char myArgsArray[DPDK_ARGS_MAX_ARGS][DPDK_ARGS_MAX_ARG_STRLEN];
char* myArgv[DPDK_ARGS_MAX_ARGS];

//#define DPDK_ARGS_MAX_NUM_DIR (30)
//static const char DEFAULT_DIR[] = "/var/lib/cni/";

static char STR_MASTER[] = "master";
static char STR_SLAVE[] = "slave";
static char STR_ETHERNET[] = "ethernet";

/* Large enough to hold: ",mac=aa:bb:cc:dd:ee:ff" */
#define DPDK_ARGS_MAX_MAC_STRLEN (25)


static int getInterfaces(int argc, int *pPortCnt, int *pPortMask) {
	int i = 0;
	int j;
	int vhostCnt = 0;
	int memifCnt = 1;
	int sriovCnt = 0;
	int err;
	struct InterfaceResponse ifaceRsp;
	char macStr[DPDK_ARGS_MAX_MAC_STRLEN];
#if 0
	// Refactor code later to find JSON file. Needs work so commented out for now.
	DIR *d;
	struct dirent *dir;
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
#endif

	ifaceRsp.numIfaceAllocated = NETUTIL_NUM_NETWORKINTERFACE;
	ifaceRsp.numIfacePopulated = 0;
	ifaceRsp.pIface = malloc(ifaceRsp.numIfaceAllocated * sizeof(struct InterfaceData));
	if (ifaceRsp.pIface) {
		memset(ifaceRsp.pIface, 0, (ifaceRsp.numIfaceAllocated * sizeof(struct InterfaceData)));
		err = GetInterfaces(&ifaceRsp);
		if ((err == NETUTIL_ERRNO_SUCCESS) || (err == NETUTIL_ERRNO_SIZE_ERROR)) {
			for (i = 0; i < ifaceRsp.numIfacePopulated; i++) {
				if (debugArgs) {
					printf("  Interface[%d]:\n", i);

					printf("  ");
					if (ifaceRsp.pIface[i].IfName) {
						printf("  IfName=\"%s\"", ifaceRsp.pIface[i].IfName);
					}
					if (ifaceRsp.pIface[i].Name) {
						printf("  Name=\"%s\"", ifaceRsp.pIface[i].Name);
					}
					printf("  Type=%s",
						(ifaceRsp.pIface[i].Type == NETUTIL_TYPE_KERNEL) ? "kernel" :
						(ifaceRsp.pIface[i].Type == NETUTIL_TYPE_SRIOV) ? "SR-IOV" :
						(ifaceRsp.pIface[i].Type == NETUTIL_TYPE_VHOST) ? "vHost" :
						(ifaceRsp.pIface[i].Type == NETUTIL_TYPE_MEMIF) ? "memif" :
						(ifaceRsp.pIface[i].Type == NETUTIL_TYPE_VDPA) ? "vDPA" :
						(ifaceRsp.pIface[i].Type == NETUTIL_TYPE_UNKNOWN) ? "unknown" : "error");
					printf("\n");

					printf("  ");
					if (ifaceRsp.pIface[i].Network.Mac) {
						printf("  MAC=\"%s\"", ifaceRsp.pIface[i].Network.Mac);
					}
					for (j = 0; j < NETUTIL_NUM_IPS; j++) {
						if (ifaceRsp.pIface[i].Network.IPs[j]) {
							printf("  IP=\"%s\"", ifaceRsp.pIface[i].Network.IPs[j]);
						}
					}
					printf("\n");
				}

				switch (ifaceRsp.pIface[i].Type) {
					case NETUTIL_TYPE_SRIOV:
						if (debugArgs) {
							printf("  ");
							if (ifaceRsp.pIface[i].Sriov.PCIAddress) {
								printf("  PCIAddress=%s", ifaceRsp.pIface[i].Sriov.PCIAddress);
							}
							printf("\n");
						}

						if (ifaceRsp.pIface[i].Sriov.PCIAddress) {
							snprintf(&myArgsArray[argc++][0], DPDK_ARGS_MAX_ARG_STRLEN-1,
									 "-w %s", ifaceRsp.pIface[i].Sriov.PCIAddress);
							sriovCnt++;

							free(ifaceRsp.pIface[i].Sriov.PCIAddress);

							*pPortMask = *pPortMask | 1 << *pPortCnt;
							*pPortCnt  = *pPortCnt + 1;
						}
						break;
					case NETUTIL_TYPE_VHOST:
						if (debugArgs) {
							printf("  ");
							printf("  Mode=%s",
								(ifaceRsp.pIface[i].Vhost.Mode == NETUTIL_VHOST_MODE_CLIENT) ? "client" :
								(ifaceRsp.pIface[i].Vhost.Mode == NETUTIL_VHOST_MODE_SERVER) ? "server" : "error");
							if (ifaceRsp.pIface[i].Vhost.Socketpath) {
								printf("  Socketpath=\"%s\"", ifaceRsp.pIface[i].Vhost.Socketpath);
							}
							printf("\n");
						}

						if (ifaceRsp.pIface[i].Vhost.Socketpath) {
							if (ifaceRsp.pIface[i].Vhost.Mode == NETUTIL_VHOST_MODE_SERVER) {
								snprintf(&myArgsArray[argc++][0], DPDK_ARGS_MAX_ARG_STRLEN-1,
										 "--vdev=virtio_user%d,path=%s", vhostCnt, ifaceRsp.pIface[i].Vhost.Socketpath);

								vhostCnt++;
								*pPortMask = *pPortMask | 1 << *pPortCnt;
								*pPortCnt  = *pPortCnt + 1;
							}
							else if (ifaceRsp.pIface[i].Vhost.Mode == NETUTIL_VHOST_MODE_CLIENT) {
								snprintf(&myArgsArray[argc++][0], DPDK_ARGS_MAX_ARG_STRLEN-1,
										 "--vdev=virtio_user%d,path=%s,queues=1", vhostCnt, ifaceRsp.pIface[i].Vhost.Socketpath);

								vhostCnt++;
								*pPortMask = *pPortMask | 1 << *pPortCnt;
								*pPortCnt  = *pPortCnt + 1;
							} else {
								printf("ERROR: Unknown vHost Mode=%d\n", ifaceRsp.pIface[i].Vhost.Mode);
							}
							free(ifaceRsp.pIface[i].Vhost.Socketpath);
						}
						break;
					case NETUTIL_TYPE_MEMIF:
						if (debugArgs) {
							printf("  ");
							printf("  Role=%s",
								(ifaceRsp.pIface[i].Memif.Role == NETUTIL_MEMIF_ROLE_MASTER) ? "master" :
								(ifaceRsp.pIface[i].Memif.Role == NETUTIL_MEMIF_ROLE_SLAVE) ? "slave" : "error");
							printf("  Mode=%s",
								(ifaceRsp.pIface[i].Memif.Mode == NETUTIL_MEMIF_MODE_ETHERNET) ? "ethernet" :
								(ifaceRsp.pIface[i].Memif.Mode == NETUTIL_MEMIF_MODE_IP) ? "ip" :
								(ifaceRsp.pIface[i].Memif.Mode == NETUTIL_MEMIF_MODE_INJECT_PUNT) ? "inject-punt" : "error");
							if (ifaceRsp.pIface[i].Memif.Socketpath) {
								printf("  Socketpath=\"%s\"", ifaceRsp.pIface[i].Memif.Socketpath);
							}
							printf("\n");
						}

						if (ifaceRsp.pIface[i].Memif.Socketpath) {
							char *pRole = NULL;
							char *pMode = NULL;

							if (ifaceRsp.pIface[i].Memif.Role == NETUTIL_MEMIF_ROLE_MASTER) {
								pRole = STR_MASTER;
							}
							else if (ifaceRsp.pIface[i].Memif.Role == NETUTIL_MEMIF_ROLE_SLAVE) {
								pRole = STR_SLAVE;
							}
							else {
								printf("ERROR: Unknown memif Role=%d\n", ifaceRsp.pIface[i].Memif.Role);
							}

							if (ifaceRsp.pIface[i].Memif.Mode == NETUTIL_MEMIF_MODE_ETHERNET) {
								pMode = STR_ETHERNET;
							}
							else if (ifaceRsp.pIface[i].Memif.Mode == NETUTIL_MEMIF_MODE_IP) {
								//pMode = "ip";
								printf("ERROR: memif Mode=%d - Not Supported in DPDK!\n", ifaceRsp.pIface[i].Memif.Mode);
							}
							else if (ifaceRsp.pIface[i].Memif.Mode == NETUTIL_MEMIF_MODE_INJECT_PUNT) {
								//pMode = "inject-punt"";
								printf("ERROR: memif Mode=%d - Not Supported in DPDK!\n", ifaceRsp.pIface[i].Memif.Mode);
							}
							else {
								printf("ERROR: Unknown memif Mode=%d\n", ifaceRsp.pIface[i].Memif.Mode);
							}

							if ((ifaceRsp.pIface[i].Network.Mac) &&
							    (strcmp(ifaceRsp.pIface[i].Network.Mac,"") != 0)) {
								snprintf(&macStr[0], DPDK_ARGS_MAX_MAC_STRLEN-1,
										 ",mac=%s", ifaceRsp.pIface[i].Network.Mac);
							}
							else {
								macStr[0] = '\0';
							}

							if ((pRole) && (pMode)) {
								snprintf(&myArgsArray[argc++][0], DPDK_ARGS_MAX_ARG_STRLEN-1,
										 "--vdev=net_memif%d,socket=%s,role=%s%s", memifCnt, ifaceRsp.pIface[i].Memif.Socketpath, pRole, &macStr[0]);

								memifCnt++;
								*pPortMask = *pPortMask | 1 << *pPortCnt;
								*pPortCnt  = *pPortCnt + 1;
							}

							free(ifaceRsp.pIface[i].Memif.Socketpath);
						}
						break;
				}

				if (ifaceRsp.pIface[i].Network.Mac) {
					free(ifaceRsp.pIface[i].Network.Mac);
				}
				for (j = 0; j < NETUTIL_NUM_IPS; j++) {
					if (ifaceRsp.pIface[i].Network.IPs[j]) {
						free(ifaceRsp.pIface[i].Network.IPs[j]);
					}
				}

				if (ifaceRsp.pIface[i].IfName) {
					free(ifaceRsp.pIface[i].IfName);
				}
				if (ifaceRsp.pIface[i].Name) {
					free(ifaceRsp.pIface[i].Name);
				}
			} /* END of FOR EACH Interface */


			if (sriovCnt == 0) {
				strncpy(&myArgsArray[argc++][0], "--no-pci", DPDK_ARGS_MAX_ARG_STRLEN-1);
			}
		}
		else {
			printf("Couldn't get network interface, err code: %d\n", err);
		}
	}

	return(argc);
}

char** GetArgs(int *pArgc, eDpdkAppType appType)
{
	int argc = 0;
	int i;
	struct CPUResponse cpuRsp;
	int err;
	int portMask = 0;
	int portCnt = 0;
	int lcoreBase = 0;
	int port;
	int length = 0;

	sleep(2);

	memset(&cpuRsp, 0, sizeof(cpuRsp));
	err = GetCPUInfo(&cpuRsp);
	if (err) {
		printf("Couldn't get CPU info, err code: %d\n", err);
	}
	if (cpuRsp.CPUSet) {
		printf("  cpuRsp.CPUSet = %s\n", cpuRsp.CPUSet);

		// Free the string
		free(cpuRsp.CPUSet);
	}


	memset(&myArgsArray[0][0], 0, sizeof(char)*DPDK_ARGS_MAX_ARG_STRLEN*DPDK_ARGS_MAX_ARGS);
	memset(&myArgv[0], 0, sizeof(char)*DPDK_ARGS_MAX_ARGS);

	if (pArgc) {
		/*
		 * Initialize EAL Options
		 */
		strncpy(&myArgsArray[argc++][0], "dpdk-app", DPDK_ARGS_MAX_ARG_STRLEN-1);

		//strncpy(&myArgsArray[argc++][0], "-m", DPDK_ARGS_MAX_ARG_STRLEN-1);
		//strncpy(&myArgsArray[argc++][0], "1024", DPDK_ARGS_MAX_ARG_STRLEN-1);

		strncpy(&myArgsArray[argc++][0], "-n", DPDK_ARGS_MAX_ARG_STRLEN-1);
		strncpy(&myArgsArray[argc++][0], "4", DPDK_ARGS_MAX_ARG_STRLEN-1);

		//strncpy(&myArgsArray[argc++][0], "--file-prefix=dpdk-app_", DPDK_ARGS_MAX_ARG_STRLEN-1);

		if (appType == DPDK_APP_TESTPMD) {
			strncpy(&myArgsArray[argc++][0], "-l", DPDK_ARGS_MAX_ARG_STRLEN-1);
			strncpy(&myArgsArray[argc++][0], "1-3", DPDK_ARGS_MAX_ARG_STRLEN-1);

			strncpy(&myArgsArray[argc++][0], "--master-lcore", DPDK_ARGS_MAX_ARG_STRLEN-1);
			strncpy(&myArgsArray[argc++][0], "1", DPDK_ARGS_MAX_ARG_STRLEN-1);

			argc = getInterfaces(argc, &portCnt, &portMask);

			/*
			 * Initialize APP Specific Options
			 */
			strncpy(&myArgsArray[argc++][0], "--", DPDK_ARGS_MAX_ARG_STRLEN-1);

			strncpy(&myArgsArray[argc++][0], "--auto-start", DPDK_ARGS_MAX_ARG_STRLEN-1);
			strncpy(&myArgsArray[argc++][0], "--tx-first", DPDK_ARGS_MAX_ARG_STRLEN-1);
			strncpy(&myArgsArray[argc++][0], "--no-lsc-interrupt", DPDK_ARGS_MAX_ARG_STRLEN-1);

			/* testpmd exits if there is not user enteraction, so print stats */
			/* every so often to keep program running. */
			strncpy(&myArgsArray[argc++][0], "--stats-period", DPDK_ARGS_MAX_ARG_STRLEN-1);
			strncpy(&myArgsArray[argc++][0], "60", DPDK_ARGS_MAX_ARG_STRLEN-1);
		}
		else if (appType == DPDK_APP_L3FWD) {
			/* NOTE: The l3fwd app requires a TX Queue per lcore. So seeting lcore to 1 */
			/*       until additional queues are added to underlying interface.         */
			strncpy(&myArgsArray[argc++][0], "-l", DPDK_ARGS_MAX_ARG_STRLEN-1);
			strncpy(&myArgsArray[argc++][0], "1", DPDK_ARGS_MAX_ARG_STRLEN-1);

			strncpy(&myArgsArray[argc++][0], "--master-lcore", DPDK_ARGS_MAX_ARG_STRLEN-1);
			strncpy(&myArgsArray[argc++][0], "1", DPDK_ARGS_MAX_ARG_STRLEN-1);
			lcoreBase = 1;

			argc = getInterfaces(argc, &portCnt, &portMask);

			/*
			 * Initialize APP Specific Options
			 */
			strncpy(&myArgsArray[argc++][0], "--", DPDK_ARGS_MAX_ARG_STRLEN-1);

			/* Set the PortMask, Hexadecimal bitmask of ports used by app. */ 
			strncpy(&myArgsArray[argc++][0], "-p", DPDK_ARGS_MAX_ARG_STRLEN-1);
			snprintf(&myArgsArray[argc++][0], DPDK_ARGS_MAX_ARG_STRLEN-1,
					"0x%x", portMask);

			/* Set all ports to promiscuous mode so that packets are accepted */
			/* regardless of the packet’s Ethernet MAC destination address.   */
			strncpy(&myArgsArray[argc++][0], "-P", DPDK_ARGS_MAX_ARG_STRLEN-1);

#if 1
			/* Determines which queues from which ports are mapped to which cores. */
			/* Usage: --config="(port,queue,lcore)[,(port,queue,lcore)]" */
			length = 0;
			length += snprintf(&myArgsArray[argc][length], DPDK_ARGS_MAX_ARG_STRLEN-length,
							"--config=\"");
			for (port = 0; port < portCnt; port++) {
				/* If not the first port, add a ',' to string. */
				if (port != 0) {
					length += snprintf(&myArgsArray[argc][length], DPDK_ARGS_MAX_ARG_STRLEN-length, ",");
				}

				/* Add each port data */
				length += snprintf(&myArgsArray[argc][length], DPDK_ARGS_MAX_ARG_STRLEN-length,
					"(%d,%d,%d)", port, 0 /* queue */, lcoreBase /*+port*/);

				/* If the last port, add a trailing " to string. */
				if (port == portCnt-1) {
					length += snprintf(&myArgsArray[argc][length], DPDK_ARGS_MAX_ARG_STRLEN-length, "\"");
				}
			}
			argc++;
#else
			/* Determines which queues from which ports are mapped to which cores. */
			/* Usage: --config (port,queue,lcore)[,(port,queue,lcore)] */
			strncpy(&myArgsArray[argc++][0], "--config", DPDK_ARGS_MAX_ARG_STRLEN-1);
			length = 0;
			for (port = 0; port < portCnt; port++) {
				/* If not the first port, add a ',' to string. */
				if (port != 0) {
					length += snprintf(&myArgsArray[argc][length], DPDK_ARGS_MAX_ARG_STRLEN-length, ",");
				}

				/* Add each port data */
				length += snprintf(&myArgsArray[argc][length], DPDK_ARGS_MAX_ARG_STRLEN-length,
					"(%d,%d,%d)", port, 0 /* queue */, lcoreBase /*+port*/);
			}
			argc++;
#endif

			/* Set to use software to analyze packet type. Without this option, */
			/* hardware will check the packet type. Not sure if vHost supports. */
			strncpy(&myArgsArray[argc++][0], "--parse-ptype", DPDK_ARGS_MAX_ARG_STRLEN-1);

		}

		for (i = 0; i < argc; i++) {
			myArgv[i] = &myArgsArray[i][0];
		}
		*pArgc = argc;
	}

	return(myArgv);
}