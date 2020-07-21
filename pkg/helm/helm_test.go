package helm

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func mustNewClient(opts ...Option) *Client {
	client, err := NewClient(config, opts...)
	Expect(err).To(Succeed())
	return client
}

var _ = Describe("Client", func() {
	It("can be used with kind cluster", func() {
		client := mustNewClient()
		_, err := client.List()
		Expect(err).ToNot(HaveOccurred())
	})
})
