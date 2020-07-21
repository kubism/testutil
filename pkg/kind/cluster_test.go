package kind

import (
	"context"
	"time"

	"github.com/kubism/testutil/pkg/rand"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	It("is functional", func() {
		cluster := mustCreateCluster(CreateWithWaitForReady(3 * time.Minute))
		defer cluster.Delete()
		tmpFile, err := cluster.GetKubeConfigAsTempFile()
		Expect(err).NotTo(HaveOccurred())
		defer tmpFile.Close()
		config, err := clientcmd.BuildConfigFromFlags("", tmpFile.Path)
		Expect(err).ToNot(HaveOccurred())
		Expect(config).ToNot(BeNil())
		k8sClient, err := client.New(config, client.Options{Scheme: scheme.Scheme})
		Expect(err).ToNot(HaveOccurred())
		Expect(k8sClient).ToNot(BeNil())
		var ns corev1.Namespace
		Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: "kube-system"}, &ns)).To(Succeed())
	})
})
