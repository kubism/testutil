package kube

import (
	"context"
	"fmt"
	"time"

	"github.com/kubism/testutil/pkg/helm"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
		ctx := context.Background()
		pods := &corev1.PodList{}
		Expect(k8sClient.List(ctx, pods, client.InNamespace(rls.Namespace),
			client.MatchingLabels{"release": rls.Name})).To(Succeed())
		Expect(len(pods.Items)).To(BeNumerically(">", 0))
		pod := pods.Items[0]
		Expect(WaitUntilReady(restConfig, &pod, 60*time.Second)).To(Succeed())
	})
})
