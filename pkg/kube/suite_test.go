package kube

import (
	"testing"
	"time"

	"github.com/kubism/testutil/pkg/helm"
	"github.com/kubism/testutil/pkg/kind"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	cluster    *kind.Cluster
	helmClient *helm.Client
	k8sClient  client.Client
	restConfig *rest.Config
)

func TestHelm(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "kube")
}

var _ = BeforeSuite(func(done Done) {
	var err error
	By("setup kind cluster")
	cluster, err = kind.NewCluster(
		kind.ClusterWithName("testutil"),
		kind.ClusterUseExisting(), // TODO: make optional via env variable
		kind.ClusterDoNotDelete(),
		kind.ClusterWithWaitForReady(3*time.Minute),
	)
	Expect(err).To(Succeed())
	By("setup helm client")
	kubeConfig, err := cluster.GetKubeConfig()
	Expect(err).To(Succeed())
	helmClient, err = helm.NewClient(kubeConfig)
	Expect(err).To(Succeed())
	Expect(helmClient).ToNot(BeNil())
	Expect(helmClient.AddRepository(&helm.RepositoryEntry{
		Name: "stable",
		URL:  "https://kubernetes-charts.storage.googleapis.com",
	})).To(Succeed())
	By("setup k8s client")
	k8sClient, err = cluster.GetClient()
	Expect(err).To(Succeed())
	Expect(k8sClient).ToNot(BeNil())
	restConfig, err = cluster.GetRESTConfig()
	Expect(err).To(Succeed())
	Expect(restConfig).ToNot(BeNil())
	close(done)
}, 120)

var _ = AfterSuite(func() {
	By("cleaning up helm client")
	if helmClient != nil {
		helmClient.Close()
	}
	By("tearing down kind cluster")
	if cluster != nil {
		cluster.Close()
	}
})
