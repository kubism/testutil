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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WaitUntilDeploymentReady", func() {
	It("waits until ready", func() {
		rls := mustInstallMinio()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		deployment := MustGetDeployment(restConfig, rls.Namespace, rls.Name+"-minio")
		Expect(WaitUntilDeploymentReady(restConfig, deployment, 60*time.Second)).To(Succeed())
		Expect(IsDeploymentReady(deployment)).To(Equal(true))
	})
})

var _ = Describe("WaitUntilDeploymentScheduled", func() {
	It("waits until scheduled", func() {
		rls := mustInstallMinio()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		deployment := MustGetDeployment(restConfig, rls.Namespace, rls.Name+"-minio")
		Expect(WaitUntilDeploymentScheduled(restConfig, deployment, 60*time.Second)).To(Succeed())
		Expect(IsDeploymentScheduled(deployment)).To(Equal(true))
	})
})
