package flags

import (
	"flag"
)

var KindCluster string

func init() {
	flag.StringVar(&KindCluster, "kind-cluster", "", "define pre-existing cluster to use for tests")
}
