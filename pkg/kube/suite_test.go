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
	"testing"
	"time"

	"github.com/kubism/testutil/internal/flags"
	"github.com/kubism/testutil/pkg/helm"
	"github.com/kubism/testutil/pkg/kind"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	accessKeyID     = "TESTACCESSKEY"
	secretAccessKey = "TESTSECRETKEY"
	timeout         = 10 * time.Minute
)

var (
	cluster      *kind.Cluster
	helmClient   *helm.Client
	k8sClient    client.Client
	restConfig   *rest.Config
	minioRelease *helm.Release
)

func TestHelm(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "kube")
}

var _ = BeforeSuite(func(done Done) {
	var err error
	By("setup kind cluster")
	clusterOptions := []kind.ClusterOption{
		kind.ClusterWithWaitForReady(timeout),
	}
	if flags.KindCluster != "" {
		clusterOptions = append(clusterOptions,
			kind.ClusterWithName(flags.KindCluster),
			kind.ClusterUseExisting(),
			kind.ClusterDoNotDelete(),
		)
	}
	cluster, err = kind.NewCluster(clusterOptions...)
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
	By("setup prepared minio release")
	minioRelease = mustInstallMinio()
	close(done)
}, 120)

var _ = AfterSuite(func() {
	By("uninstalling minio release")
	if minioRelease != nil {
		_ = helmClient.Uninstall(minioRelease.Name)
	}
	By("cleaning up helm client")
	if helmClient != nil {
		helmClient.Close()
	}
	By("tearing down kind cluster")
	if cluster != nil {
		cluster.Close()
	}
})

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
	deployment := MustGetDeployment(restConfig, rls.Namespace, rls.Name+"-minio")
	Expect(WaitUntilDeploymentScheduled(restConfig, deployment, timeout)).To(Succeed())
	Expect(k8sClient.List(ctx, pods, client.InNamespace(rls.Namespace),
		client.MatchingLabels{"release": rls.Name})).To(Succeed())
	Expect(len(pods.Items)).To(BeNumerically(">", 0))
	pod := pods.Items[0]
	Expect(WaitUntilPodReady(restConfig, &pod, timeout)).To(Succeed())
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
