package helm

import (
	"testing"
	"time"

	"github.com/kubism/testutil/pkg/kind"
	"github.com/kubism/testutil/pkg/rand"
	"k8s.io/client-go/rest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	config  *rest.Config
	cluster *kind.Cluster
)

func TestHelm(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "helm")
}

var _ = BeforeSuite(func(done Done) {
	var err error
	By("setup kind cluster")
	cluster, err = kind.NewCluster(rand.String(8), kind.ClusterWithWaitForReady(3*time.Minute))
	Expect(err).To(Succeed())
	By("setup rest client")
	config, err = cluster.GetRESTConfig()
	Expect(err).To(Succeed())
	close(done)
}, 120)

var _ = AfterSuite(func() {
	By("tearing down kind cluster")
	if cluster != nil {
		cluster.Close()
	}
})
