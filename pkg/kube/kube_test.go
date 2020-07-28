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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {
	It("is successful with valid configuration", func() {
		c, err := NewClient(restConfig, ClientWithScheme(scheme.Scheme))
		Expect(err).To(Succeed())
		Expect(c).ToNot(BeNil())
	})
	It("fails with invalid REST config", func() {
		Context("empty host", func() {
			brokenRESTConfig, err := cluster.GetRESTConfig()
			Expect(err).ToNot(HaveOccurred())
			brokenRESTConfig.Host = ""
			c, err := NewClient(brokenRESTConfig)
			Expect(err).To(HaveOccurred())
			Expect(c).To(BeNil())
		})
		Context("missing CA", func() {
			brokenRESTConfig, err := cluster.GetRESTConfig()
			Expect(err).ToNot(HaveOccurred())
			brokenRESTConfig.CAFile = ""
			brokenRESTConfig.CAData = []byte{}
			c, err := NewClient(brokenRESTConfig)
			Expect(err).To(HaveOccurred())
			Expect(c).To(BeNil())
		})
	})
})

var _ = Describe("PortForward", func() {
	It("can portforward existing pod", func() {
		rls := mustInstallNginx()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		pod := mustGetReadyNginxPod(rls)
		Expect(func() {
			Expect(k8sClient.Get(context.Background(), NamespacedName(pod), pod)).To(Succeed())
		}).ShouldNot(Panic())
		By("creating port-forward")
		pf, err := k8sClient.PortForward(pod, PortAny, 8080)
		Expect(err).ToNot(HaveOccurred())
		defer pf.Close()
		Expect(checkNginxServer(fmt.Sprintf("http://localhost:%d", pf.LocalPort))).To(Succeed())
	})
	It("fails with invalid host port", func() {
		rls := mustInstallNginx()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		pod := mustGetReadyNginxPod(rls)
		pf, err := k8sClient.PortForward(pod, 999999, 8080)
		Expect(err).To(HaveOccurred())
		Expect(pf).To(BeNil())
	})
	It("fails for non-existing pod", func() {
		var pod corev1.Pod
		pod.ObjectMeta.Namespace = "default"
		pod.ObjectMeta.Name = "doesnotexist"
		pf, err := k8sClient.PortForward(&pod, PortAny, 8080)
		Expect(err).To(HaveOccurred())
		Expect(pf).To(BeNil())
	})
})

var _ = Describe("Logs", func() {
	It("can get logs of existing pod", func() {
		pod := mustGetReadyNginxPod(nginxRelease)
		logs, err := k8sClient.LogsString(context.Background(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(logs)).To(BeNumerically(">", 0))
	})
	It("fails if pod does not exist", func() {
		var pod corev1.Pod
		pod.Namespace = "default"
		pod.Name = "doesnotexist"
		_, err := k8sClient.LogsString(context.Background(), &pod)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("Events", func() {
	It("can get events for existing pod", func() {
		pod := mustGetReadyNginxPod(nginxRelease)
		events, err := k8sClient.Events(context.Background(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(events)).To(BeNumerically(">", 0))
	})
})

var _ = Describe("GetPodsByOwner", func() {
	// for successful usage, see `WaitUntilJobActive`-tests
	It("fails for non-existing object", func() {
		job := genPiJob()
		job.Namespace = "doesnotexist"
		_, err := k8sClient.GetPodsForOwner(context.Background(), job)
		Expect(err).To(HaveOccurred())
	})
	It("fails for malformed object", func() {
		job := genPiJob()
		job.Namespace = "+"
		_, err := k8sClient.GetPodsForOwner(context.Background(), job)
		Expect(err).To(HaveOccurred())
	})
})

type testCondition struct {
	Object runtime.Object
}

func (c testCondition) subject() runtime.Object {
	return c.Object
}

func (_ testCondition) check() bool {
	return false
}

var _ = Describe("WaitUntil", func() {
	It("fails for condition with nil subject", func() {
		Expect(k8sClient.WaitUntil(context.Background(), testCondition{})).ToNot(Succeed())
	})
	It("can timeout if check does not become true", func() {
		deployment := DeploymentWithNamespacedName(nginxRelease.Namespace, nginxRelease.Name+"-nginx")
		Expect(k8sClient.Get(context.Background(), NamespacedName(deployment), deployment)).To(Succeed())
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		Expect(k8sClient.WaitUntil(ctx, testCondition{deployment})).ToNot(Succeed())
		defer cancel()
	})
	It("waits until deployment ready", func() {
		rls := mustInstallNginx()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		deployment := DeploymentWithNamespacedName(rls.Namespace, rls.Name+"-nginx")
		Expect(k8sClient.Get(ctx, NamespacedName(deployment), deployment)).To(Succeed())
		Expect(k8sClient.WaitUntil(ctx, DeploymentIsReady(deployment))).To(Succeed())
		Expect(IsDeploymentReady(deployment)).To(Equal(true))
	})
	It("fails for non-existing deployment", func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		Expect(k8sClient.WaitUntil(ctx, DeploymentIsReady(&appsv1.Deployment{}))).NotTo(Succeed())
	})
	It("waits until deployment scheduled", func() {
		rls := mustInstallNginx()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		deployment := DeploymentWithNamespacedName(rls.Namespace, rls.Name+"-nginx")
		Expect(k8sClient.Get(ctx, NamespacedName(deployment), deployment)).To(Succeed())
		Expect(k8sClient.WaitUntil(ctx, DeploymentIsScheduled(deployment))).To(Succeed())
		Expect(IsDeploymentScheduled(deployment)).To(Equal(true))
	})
	It("waits until deployment updated", func() {
		rls := mustInstallNginx()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		deployment := DeploymentWithNamespacedName(rls.Namespace, rls.Name+"-nginx")
		Expect(k8sClient.Get(ctx, NamespacedName(deployment), deployment)).To(Succeed())
		Expect(k8sClient.WaitUntil(ctx, DeploymentIsUpdated(deployment))).To(Succeed())
		Expect(IsDeploymentUpdated(deployment)).To(Equal(true))
	})
	It("waits until replicaset available and ready", func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		deployment := DeploymentWithNamespacedName(nginxRelease.Namespace, nginxRelease.Name+"-nginx")
		Expect(k8sClient.Get(ctx, NamespacedName(deployment), deployment)).To(Succeed())
		replicaSets, err := k8sClient.GetReplicaSetsForDeployment(context.Background(), deployment)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(replicaSets)).To(BeNumerically(">", 0))
		rs := &replicaSets[0]
		Expect(k8sClient.WaitUntil(ctx, ReplicaSetIsAvailable(rs), ReplicaSetIsReady(rs))).To(Succeed())
		Expect(IsReplicaSetAvailable(rs)).To(Equal(true))
		Expect(IsReplicaSetReady(rs)).To(Equal(true))
	})
	It("waits until job is active", func() {
		job := mustCreatePiJob()
		defer func() {
			_ = k8sClient.Delete(context.Background(), job)
		}()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		Expect(k8sClient.WaitUntil(ctx, JobIsActive(job))).To(Succeed())
		Expect(IsJobActive(job)).To(Equal(true))
		pods, err := k8sClient.GetPodsForJob(ctx, job)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(pods)).To(Equal(1))
	})
	It("waits until job is active", func() {
		cronJob := mustCreatePiCronJob()
		defer func() {
			_ = k8sClient.Delete(context.Background(), cronJob)
		}()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		Expect(k8sClient.WaitUntil(ctx, CronJobIsActive(cronJob))).To(Succeed())
		Expect(IsCronJobActive(cronJob)).To(Equal(true))
		jobs, err := k8sClient.GetJobsForCronJob(ctx, cronJob)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(jobs)).To(Equal(1))
	})
})

var _ = Describe("NamespacedName", func() {
	It("can retrieve namespace and name", func() {
		Expect(func() {
			pod := PodWithNamespacedName("a", "b")
			namespacedName := NamespacedName(pod)
			Expect(namespacedName.Namespace).To(Equal(pod.Namespace))
			Expect(namespacedName.Name).To(Equal(pod.Name))
		}).ShouldNot(Panic())
	})
	It("fails for invalid types", func() {
		Expect(func() {
			_ = NamespacedName(nil)
		}).Should(Panic())
	})
	It("works for existing replicaset", func() {
		Expect(func() {
			deployment := DeploymentWithNamespacedName(nginxRelease.Namespace, nginxRelease.Name+"-nginx")
			Expect(k8sClient.Get(context.Background(), NamespacedName(deployment), deployment)).To(Succeed())
			replicaSets, err := k8sClient.GetReplicaSetsForDeployment(context.Background(), deployment)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(replicaSets)).To(BeNumerically(">", 0))
			rs := &replicaSets[0]
			tmp := ReplicaSetWithNamespacedName(rs.Namespace, rs.Name)
			Expect(k8sClient.Get(context.Background(), NamespacedName(tmp), tmp)).To(Succeed())
		}).ShouldNot(Panic())
	})
	It("works for existing job", func() {
		Expect(func() {
			job := mustCreatePiJob()
			defer func() {
				_ = k8sClient.Delete(context.Background(), job)
			}()
			tmp := JobWithNamespacedName(job.Namespace, job.Name)
			Expect(k8sClient.Get(context.Background(), NamespacedName(tmp), tmp)).To(Succeed())
		}).ShouldNot(Panic())
	})
	It("works for existing cronjob", func() {
		Expect(func() {
			cronJob := mustCreatePiCronJob()
			defer func() {
				_ = k8sClient.Delete(context.Background(), cronJob)
			}()
			tmp := CronJobWithNamespacedName(cronJob.Namespace, cronJob.Name)
			Expect(k8sClient.Get(context.Background(), NamespacedName(tmp), tmp)).To(Succeed())
		}).ShouldNot(Panic())
	})
})
