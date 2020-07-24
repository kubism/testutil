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
	appsv1 "k8s.io/api/apps/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MustGetDeployment", func() {
	It("works for existing deployment", func() {
		Expect(func() {
			_ = MustGetDeployment(restConfig, nginxRelease.Namespace, nginxRelease.Name+"-nginx")
		}).ShouldNot(Panic())
	})
	It("panics if deployment does not exist", func() {
		Expect(func() {
			_ = MustGetDeployment(restConfig, nginxRelease.Namespace, nginxRelease.Name+"-thiscantexist")
		}).Should(Panic())
	})
	It("panics with invalid REST config", func() {
		brokenRESTConfig, err := cluster.GetRESTConfig()
		Expect(err).ToNot(HaveOccurred())
		brokenRESTConfig.CAFile = ""
		brokenRESTConfig.CAData = []byte{}
		Expect(func() {
			_ = MustGetDeployment(brokenRESTConfig, nginxRelease.Namespace, nginxRelease.Name+"-nginx")
		}).Should(Panic())
	})
})

var _ = Describe("WaitUntilDeploymentReady", func() {
	It("waits until ready", func() {
		rls := mustInstallNginx()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		deployment := MustGetDeployment(restConfig, rls.Namespace, rls.Name+"-nginx")
		Expect(WaitUntilDeploymentReady(restConfig, deployment, timeout)).To(Succeed())
		Expect(IsDeploymentReady(deployment)).To(Equal(true))
	})
	It("fails with invalid REST config", func() {
		brokenRESTConfig, err := cluster.GetRESTConfig()
		Expect(err).ToNot(HaveOccurred())
		brokenRESTConfig.CAFile = ""
		brokenRESTConfig.CAData = []byte{}
		deployment := MustGetDeployment(restConfig, nginxRelease.Namespace, nginxRelease.Name+"-nginx")
		Expect(WaitUntilDeploymentReady(brokenRESTConfig, deployment, timeout)).NotTo(Succeed())
	})
	It("fails for non-existing deployment", func() {
		Expect(WaitUntilDeploymentReady(restConfig, &appsv1.Deployment{}, timeout)).NotTo(Succeed())
	})
})

var _ = Describe("WaitUntilDeploymentScheduled", func() {
	It("waits until scheduled", func() {
		rls := mustInstallNginx()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		deployment := MustGetDeployment(restConfig, rls.Namespace, rls.Name+"-nginx")
		Expect(WaitUntilDeploymentScheduled(restConfig, deployment, timeout)).To(Succeed())
		Expect(IsDeploymentScheduled(deployment)).To(Equal(true))
	})
})
