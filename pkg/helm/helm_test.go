package helm

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	stableRepository = &RepositoryEntry{
		Name: "stable",
		URL:  "https://kubernetes-charts.storage.googleapis.com",
	}
)

func mustNewClient(opts ...ClientOption) *Client {
	client, err := NewClient(kubeConfig, opts...)
	Expect(err).To(Succeed())
	return client
}

var _ = Describe("Client", func() {
	It("can be used with kind cluster", func() {
		client := mustNewClient()
		defer client.Close()
		_, err := client.List()
		Expect(err).ToNot(HaveOccurred())
	})
	It("can add repository and install chart", func() {
		client := mustNewClient()
		// defer client.Close()
		Expect(client.AddRepository(stableRepository)).To(Succeed())
		_, err := client.Install("stable/minio", "", ValuesOptions{})
		Expect(err).ToNot(HaveOccurred())
	})
})
