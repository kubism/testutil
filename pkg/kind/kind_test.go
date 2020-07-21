package kind

import (
	"context"
	"time"

	"github.com/kubism/testutil/pkg/rand"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func mustNewCluster(opts ...ClusterOption) *Cluster {
	cluster, err := NewCluster(rand.String(8), opts...)
	Expect(err).To(Succeed())
	return cluster
}

var _ = Describe("Cluster", func() {
	It("can be created", func() {
		Context("with no options", func() {
			cluster := mustNewCluster()
			Expect(cluster.Close()).To(Succeed())
		})
	})
	It("is functional", func() {
		cluster := mustNewCluster(ClusterWithWaitForReady(3 * time.Minute))
		defer cluster.Close()
		config, err := cluster.GetRESTConfig()
		Expect(err).NotTo(HaveOccurred())
		Expect(config).ToNot(BeNil())
		k8sClient, err := client.New(config, client.Options{Scheme: scheme.Scheme})
		Expect(err).ToNot(HaveOccurred())
		Expect(k8sClient).ToNot(BeNil())
		var ns corev1.Namespace
		Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: "kube-system"}, &ns)).To(Succeed())
	})
})
