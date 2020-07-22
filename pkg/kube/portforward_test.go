package kube

import (
	"fmt"

	"github.com/kubism/testutil/pkg/helm"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	accessKeyID     = "TESTACCESSKEY"
	secretAccessKey = "TESTSECRETKEY"
)

var _ = Describe("PortForward", func() {
	It("can portforward pod", func() {
		rls, err := helmClient.Install("stable/minio", "", helm.ValuesOptions{
			StringValues: []string{
				fmt.Sprintf("accessKey=%s", accessKeyID),
				fmt.Sprintf("secretKey=%s", secretAccessKey),
				"readinessProbe.initialDelaySeconds=10",
			},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(rls).ToNot(BeNil())
		// defer helmClient.Uninstall(rls.Name)
		// k8sClient.List()
	})
})
