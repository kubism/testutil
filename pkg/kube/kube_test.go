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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
			_ = k8sClient.MustGetPod(context.Background(), pod.Namespace, pod.Name)
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

var _ = Describe("GetPodLogs", func() {
	It("can get logs of existing pod", func() {
		pod := mustGetReadyNginxPod(nginxRelease)
		logs, err := k8sClient.GetPodLogsString(context.Background(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(logs)).To(BeNumerically(">", 0))
	})
	It("fails if pod does not exist", func() {
		var pod corev1.Pod
		pod.Namespace = "default"
		pod.Name = "doesnotexist"
		_, err := k8sClient.GetPodLogsString(context.Background(), &pod)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("MustGetDeployment", func() {
	It("works for existing deployment", func() {
		Expect(func() {
			_ = k8sClient.MustGetDeployment(context.Background(), nginxRelease.Namespace, nginxRelease.Name+"-nginx")
		}).ShouldNot(Panic())
	})
	It("panics if deployment does not exist", func() {
		Expect(func() {
			_ = k8sClient.MustGetDeployment(context.Background(), nginxRelease.Namespace, nginxRelease.Name+"-thiscantexist")
		}).Should(Panic())
	})
})

var _ = Describe("WaitUntil", func() {
	It("waits until deployment ready", func() {
		rls := mustInstallNginx()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		deployment := k8sClient.MustGetDeployment(ctx, rls.Namespace, rls.Name+"-nginx")
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
		deployment := k8sClient.MustGetDeployment(ctx, rls.Namespace, rls.Name+"-nginx")
		Expect(k8sClient.WaitUntil(ctx, DeploymentIsScheduled(deployment))).To(Succeed())
		Expect(IsDeploymentScheduled(deployment)).To(Equal(true))
	})
	It("waits until deployment updated", func() {
		rls := mustInstallNginx()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		deployment := k8sClient.MustGetDeployment(ctx, rls.Namespace, rls.Name+"-nginx")
		Expect(k8sClient.WaitUntil(ctx, DeploymentIsUpdated(deployment))).To(Succeed())
		Expect(IsDeploymentUpdated(deployment)).To(Equal(true))
	})
	It("waits until replicaset available and ready", func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		deployment := k8sClient.MustGetDeployment(ctx, nginxRelease.Namespace, nginxRelease.Name+"-nginx")
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

var _ = Describe("MustGetReplicaSet", func() {
	It("works for existing replicaset", func() {
		Expect(func() {
			deployment := k8sClient.MustGetDeployment(context.Background(), nginxRelease.Namespace, nginxRelease.Name+"-nginx")
			replicaSets, err := k8sClient.GetReplicaSetsForDeployment(context.Background(), deployment)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(replicaSets)).To(BeNumerically(">", 0))
			rs := &replicaSets[0]
			_ = k8sClient.MustGetReplicaSet(context.Background(), rs.Namespace, rs.Name)
		}).ShouldNot(Panic())
	})
	It("panics if replicaset does not exist", func() {
		Expect(func() {
			_ = k8sClient.MustGetReplicaSet(context.Background(), nginxRelease.Namespace, nginxRelease.Name+"-thiscantexist")
		}).Should(Panic())
	})
})

var _ = Describe("MustGetJob", func() {
	It("works for existing job", func() {
		Expect(func() {
			job := mustCreatePiJob()
			defer func() {
				_ = k8sClient.Delete(context.Background(), job)
			}()
			_ = k8sClient.MustGetJob(context.Background(), job.Namespace, job.Name)
		}).ShouldNot(Panic())
	})
	It("panics if job does not exist", func() {
		Expect(func() {
			_ = k8sClient.MustGetJob(context.Background(), "default", "thiscantexist")
		}).Should(Panic())
	})
})

var _ = Describe("MustGetCronJob", func() {
	It("works for existing cronjob", func() {
		Expect(func() {
			cronJob := mustCreatePiCronJob()
			defer func() {
				_ = k8sClient.Delete(context.Background(), cronJob)
			}()
			_ = k8sClient.MustGetCronJob(context.Background(), cronJob.Namespace, cronJob.Name)
		}).ShouldNot(Panic())
	})
	It("panics if cronjob does not exist", func() {
		Expect(func() {
			_ = k8sClient.MustGetCronJob(context.Background(), "default", "thiscantexist")
		}).Should(Panic())
	})
})

var _ = Describe("GetEvents", func() {
	It("can get events for existing pod", func() {
		pod := mustGetReadyNginxPod(nginxRelease)
		events, err := k8sClient.GetEvents(context.Background(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(events)).To(BeNumerically(">", 0))
	})
})
