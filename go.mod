module github.com/intel/userspace-cni-network-plugin
replace cloud.google.com/go => cloud.google.com/go v0.54.0

go 1.20

require (
	go.fd.io/govpp v0.7.0
	github.com/containernetworking/cni v1.1.2
	github.com/containernetworking/plugins v1.3.0
	github.com/go-logfmt/logfmt v0.6.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.9.2
	github.com/stretchr/testify v1.8.3
	github.com/vishvananda/netlink v1.2.1-beta.2
	golang.org/x/sys v0.8.0
	k8s.io/api v0.27.2
	k8s.io/apimachinery v0.27.2
	k8s.io/client-go v0.27.2
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/lunixbochs/struc v0.0.0-20200707160740-784aaebc1d40 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/safchain/ethtool v0.3.0 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
