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
	"net/http"
	"testing"
	"time"

	"github.com/kubism/testutil/internal/flags"
	"github.com/kubism/testutil/pkg/helm"
	"github.com/kubism/testutil/pkg/kind"
	"github.com/kubism/testutil/pkg/rand"

	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	timeout = 5 * time.Minute
)

var (
	cluster      *kind.Cluster
	helmClient   *helm.Client
	k8sClient    *Client
	restConfig   *rest.Config
	nginxRelease *helm.Release
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
	restConfig, err = cluster.GetRESTConfig()
	Expect(err).To(Succeed())
	Expect(restConfig).ToNot(BeNil())
	By("setup helm client")
	kubeConfig, err := cluster.GetKubeConfig()
	Expect(err).To(Succeed())
	helmClient, err = helm.NewClient(kubeConfig)
	Expect(err).To(Succeed())
	Expect(helmClient).ToNot(BeNil())
	Expect(helmClient.AddRepository(&helm.RepositoryEntry{
		Name: "bitnami",
		URL:  "https://charts.bitnami.com/bitnami",
	})).To(Succeed())
	By("setup k8s client")
	k8sClient, err = NewClient(restConfig)
	Expect(err).To(Succeed())
	Expect(k8sClient).ToNot(BeNil())
	By("setup prepared nginx release")
	nginxRelease = mustInstallNginx()
	close(done)
}, 300)

var _ = AfterSuite(func() {
	By("uninstalling nginx release")
	if nginxRelease != nil {
		_ = helmClient.Uninstall(nginxRelease.Name)
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

func mustInstallNginx() *helm.Release {
	By("helm install bitnami/nginx")
	rls, err := helmClient.Install("bitnami/nginx", "", helm.ValuesOptions{})
	Expect(err).ToNot(HaveOccurred())
	Expect(rls).ToNot(BeNil())
	By(fmt.Sprintf("helm installed %s", rls.Name))
	return rls
}

func mustGetReadyNginxPod(rls *helm.Release) *corev1.Pod {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	pods := &corev1.PodList{}
	deployment := k8sClient.MustGetDeployment(ctx, rls.Namespace, rls.Name+"-nginx")
	By(fmt.Sprintf("waiting until deployment %s-nginx is scheduled", rls.Name))
	Expect(k8sClient.WaitUntil(ctx, DeploymentIsScheduled(deployment))).To(Succeed())
	Expect(k8sClient.List(ctx, pods, client.InNamespace(rls.Namespace),
		client.MatchingLabels{"app.kubernetes.io/instance": rls.Name})).To(Succeed())
	Expect(len(pods.Items)).To(BeNumerically(">", 0))
	pod := pods.Items[0]
	By("waiting until pod is ready")
	Expect(k8sClient.WaitUntil(ctx, PodIsReady(&pod))).To(Succeed())
	return &pod
}

func checkNginxServer(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Expected 200 got %d", resp.StatusCode)
	}
	return nil
}

func genPiJob() *batchv1.Job {
	backoffLimit := int32(3)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "pi-" + rand.String(5),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "pi",
							Image:   "perl",
							Command: []string{"perl", "-Mbignum=bpi", "-wle", "print bpi(2000)"},
						},
					},
				},
			},
		},
	}
}

func mustCreatePiJob() *batchv1.Job {
	job := genPiJob()
	Expect(k8sClient.Create(context.Background(), job)).To(Succeed())
	return job
}

func genPiCronJob() *batchv1beta1.CronJob {
	job := genPiJob()
	return &batchv1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "pi-cron-" + rand.String(5),
		},
		Spec: batchv1beta1.CronJobSpec{
			Schedule: "* * * * *",
			JobTemplate: batchv1beta1.JobTemplateSpec{
				Spec: job.Spec,
			},
		},
	}
}

func mustCreatePiCronJob() *batchv1beta1.CronJob {
	cronJob := genPiCronJob()
	Expect(k8sClient.Create(context.Background(), cronJob)).To(Succeed())
	return cronJob
}
