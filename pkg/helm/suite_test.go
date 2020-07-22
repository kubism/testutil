package helm

import (
	"os"
	"testing"
	"time"

	"github.com/kubism/testutil/pkg/kind"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	kubeConfig string
	cluster    *kind.Cluster
)

func TestHelm(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "helm")
}

var _ = BeforeSuite(func(done Done) {
	var err error
	By("setup kind cluster")
	clusterOptions := []kind.ClusterOption{
		kind.ClusterWithWaitForReady(3 * time.Minute),
	}
	if os.Getenv("TEST_USE_EXISTING") != "" {
		clusterOptions = append(clusterOptions,
			kind.ClusterWithName("testutil"),
			kind.ClusterUseExisting(),
			kind.ClusterDoNotDelete(),
		)
	}
	cluster, err = kind.NewCluster(clusterOptions...)
	Expect(err).To(Succeed())
	By("setup kubeconfig")
	kubeConfig, err = cluster.GetKubeConfig()
	Expect(err).To(Succeed())
	close(done)
}, 120)

var _ = AfterSuite(func() {
	By("tearing down kind cluster")
	if cluster != nil {
		cluster.Close()
	}
})
