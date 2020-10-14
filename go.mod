module github.com/intel/userspace-cni-network-plugin

go 1.14

require (
	git.fd.io/govpp.git v0.3.5
	github.com/containernetworking/cni v0.8.0
	github.com/containernetworking/plugins v0.8.0
	github.com/coreos/go-iptables v0.4.5 // indirect
	github.com/go-logfmt/logfmt v0.5.0
	github.com/lunixbochs/struc v0.0.0-20200707160740-784aaebc1d40
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/pkg/errors v0.9.1
	github.com/safchain/ethtool v0.0.0-20200804214954-8f958a28363a // indirect
	github.com/sirupsen/logrus v1.7.0
	github.com/stretchr/testify v1.6.1
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/sys v0.0.0-20201005065044-765f4ea38db3
	gopkg.in/check.v1 v1.0.0-20200902074654-038fdea0a05b // indirect
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v0.17.0
)
