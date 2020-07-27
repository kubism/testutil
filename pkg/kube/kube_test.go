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

var _ = Describe("WaitUntilDeploymentReady", func() {
	It("waits until ready", func() {
		rls := mustInstallNginx()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		deployment := k8sClient.MustGetDeployment(ctx, rls.Namespace, rls.Name+"-nginx")
		Expect(k8sClient.WaitUntilDeploymentReady(ctx, deployment)).To(Succeed())
		Expect(IsDeploymentReady(deployment)).To(Equal(true))
	})
	It("fails for non-existing deployment", func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		Expect(k8sClient.WaitUntilDeploymentReady(ctx, &appsv1.Deployment{})).NotTo(Succeed())
	})
})

var _ = Describe("WaitUntilDeploymentScheduled", func() {
	It("waits until scheduled", func() {
		rls := mustInstallNginx()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		deployment := k8sClient.MustGetDeployment(ctx, rls.Namespace, rls.Name+"-nginx")
		Expect(k8sClient.WaitUntilDeploymentScheduled(ctx, deployment)).To(Succeed())
		Expect(IsDeploymentScheduled(deployment)).To(Equal(true))
	})
})

var _ = Describe("WaitUntilDeploymentUpdated", func() {
	It("waits until updated", func() {
		rls := mustInstallNginx()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		deployment := k8sClient.MustGetDeployment(ctx, rls.Namespace, rls.Name+"-nginx")
		Expect(k8sClient.WaitUntilDeploymentUpdated(ctx, deployment)).To(Succeed())
		Expect(IsDeploymentUpdated(deployment)).To(Equal(true))
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
