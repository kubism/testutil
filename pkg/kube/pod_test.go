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
	"fmt"

	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

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
		rls := mustInstallMinio()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		pod := mustGetReadyMinioPod(rls)
		Context("empty host", func() {
			brokenRESTConfig, err := cluster.GetRESTConfig()
			Expect(err).ToNot(HaveOccurred())
			brokenRESTConfig.Host = ""
			pf, err := NewPortForward(brokenRESTConfig, pod, PortAny, 9000)
			Expect(err).To(HaveOccurred())
			Expect(pf).To(BeNil())
		})
		Context("missing CA", func() {
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
