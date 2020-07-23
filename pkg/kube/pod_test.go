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
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		ctx := context.Background()
		pods := &corev1.PodList{}
		Expect(k8sClient.List(ctx, pods, client.InNamespace(rls.Namespace),
			client.MatchingLabels{"release": rls.Name})).To(Succeed())
		Expect(len(pods.Items)).To(BeNumerically(">", 0))
		pod := pods.Items[0]
		Expect(WaitUntilReady(restConfig, &pod, 60*time.Second)).To(Succeed())
		pf, err := NewPortForward(restConfig, &pod, PortAny, 9000)
		Expect(err).ToNot(HaveOccurred())
		minioClient, err := minio.New(fmt.Sprintf("localhost:%d", pf.LocalPort), &minio.Options{
			Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
			Secure: false,
		})
		Expect(err).ToNot(HaveOccurred())
		_, err = minioClient.ListBuckets(context.Background())
		Expect(err).ToNot(HaveOccurred())
		defer pf.Close()
	})
})
