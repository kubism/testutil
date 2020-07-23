/*
Copyright 2020 Testutil Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kube

import (
	"context"
	"fmt"
	"time"

	"github.com/kubism/testutil/pkg/helm"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	accessKeyID     = "TESTACCESSKEY"
	secretAccessKey = "TESTSECRETKEY"
)

func mustInstallMinio() *helm.Release {
	rls, err := helmClient.Install("stable/minio", "", helm.ValuesOptions{
		StringValues: []string{
			fmt.Sprintf("accessKey=%s", accessKeyID),
			fmt.Sprintf("secretKey=%s", secretAccessKey),
			"readinessProbe.initialDelaySeconds=10",
		},
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(rls).ToNot(BeNil())
	return rls
}

func mustGetReadyMinioPod(rls *helm.Release) *corev1.Pod {
	ctx := context.Background()
	pods := &corev1.PodList{}
	// TODO: The following is actually introducing a race condition!
	//       We should wait until all pods of deployment are schedule.
	Expect(k8sClient.List(ctx, pods, client.InNamespace(rls.Namespace),
		client.MatchingLabels{"release": rls.Name})).To(Succeed())
	Expect(len(pods.Items)).To(BeNumerically(">", 0))
	pod := pods.Items[0]
	Expect(WaitUntilPodReady(restConfig, &pod, 60*time.Second)).To(Succeed())
	return &pod
}

func checkMinioServer(addr string) error {
	minioClient, err := minio.New(addr, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: false,
	})
	if err != nil {
		return err
	}
	_, err = minioClient.ListBuckets(context.Background())
	return err
}

var _ = Describe("PortForward", func() {
	It("can portforward existing pod", func() {
		rls := mustInstallMinio()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		pod := mustGetReadyMinioPod(rls)
		pf, err := NewPortForward(restConfig, pod, PortAny, 9000)
		Expect(err).ToNot(HaveOccurred())
		defer pf.Close()
		Expect(checkMinioServer(fmt.Sprintf("localhost:%d", pf.LocalPort))).To(Succeed())
	})
	It("fails with invalid host port", func() {
		rls := mustInstallMinio()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		pod := mustGetReadyMinioPod(rls)
		pf, err := NewPortForward(restConfig, pod, 999999, 9000)
		Expect(err).To(HaveOccurred())
		Expect(pf).To(BeNil())
	})
	It("fails for non-existing pod", func() {
		var pod corev1.Pod
		pod.ObjectMeta.Namespace = "default"
		pod.ObjectMeta.Name = "doesnotexist"
		pf, err := NewPortForward(restConfig, &pod, PortAny, 9000)
		Expect(err).To(HaveOccurred())
		Expect(pf).To(BeNil())
	})
	It("fails with invalid REST config", func() {
		Context("empty host", func() {
			rls := mustInstallMinio()
			defer helmClient.Uninstall(rls.Name) // nolint:errcheck
			pod := mustGetReadyMinioPod(rls)
			brokenRESTConfig, err := cluster.GetRESTConfig()
			Expect(err).ToNot(HaveOccurred())
			brokenRESTConfig.Host = ""
			pf, err := NewPortForward(brokenRESTConfig, pod, PortAny, 9000)
			Expect(err).To(HaveOccurred())
			Expect(pf).To(BeNil())
		})
		Context("missing CA", func() {
			rls := mustInstallMinio()
			defer helmClient.Uninstall(rls.Name) // nolint:errcheck
			pod := mustGetReadyMinioPod(rls)
			brokenRESTConfig, err := cluster.GetRESTConfig()
			Expect(err).ToNot(HaveOccurred())
			brokenRESTConfig.CAFile = ""
			brokenRESTConfig.CAData = []byte{}
			pf, err := NewPortForward(brokenRESTConfig, pod, PortAny, 9000)
			Expect(err).To(HaveOccurred())
			Expect(pf).To(BeNil())
		})
	})
})
