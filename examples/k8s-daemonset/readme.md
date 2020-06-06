## Installing Userspace CNI in k8s cluster

```kubectl apply -f userspace-daemonset.yml```

<br>

For building docker image:

Step 1: Run ```make``` and [build userspace cni](../../README.md#build--clean) 

Step 2: Go to ```/userspace``` directory and build docker image: ```docker build . -t registry:tag```

Step 3: Update image in [userspace-daemonset.yml](userspace-daemonset.yml)