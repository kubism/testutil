package kind

import (
	"github.com/kubism/testutil/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func mustCreateCluster(opts ...CreateOption) *Cluster {
	cluster, err := Create(rand.String(8))
	Expect(err).To(Succeed())
	return cluster
}

var _ = Describe("Cluster", func() {
	It("can create Clusters", func() {
		Context("with no options", func() {
			cluster := mustCreateCluster()
			Expect(cluster.Delete()).To(Succeed())
		})
	})
})
