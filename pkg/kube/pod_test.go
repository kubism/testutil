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
		rls := mustInstallNginx()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		pod := mustGetReadyNginxPod(rls)
		By("creating port-forward")
		pf, err := NewPortForward(restConfig, pod, PortAny, 8080)
		Expect(err).ToNot(HaveOccurred())
		defer pf.Close()
		Expect(checkNginxServer(fmt.Sprintf("http://localhost:%d", pf.LocalPort))).To(Succeed())
	})
	It("fails with invalid host port", func() {
		rls := mustInstallNginx()
		defer helmClient.Uninstall(rls.Name) // nolint:errcheck
		pod := mustGetReadyNginxPod(rls)
		pf, err := NewPortForward(restConfig, pod, 999999, 8080)
		Expect(err).To(HaveOccurred())
		Expect(pf).To(BeNil())
	})
	It("fails for non-existing pod", func() {
		var pod corev1.Pod
		pod.ObjectMeta.Namespace = "default"
		pod.ObjectMeta.Name = "doesnotexist"
		pf, err := NewPortForward(restConfig, &pod, PortAny, 8080)
		Expect(err).To(HaveOccurred())
		Expect(pf).To(BeNil())
	})
	It("fails with invalid REST config", func() {
		Context("empty host", func() {
			pod := mustGetReadyNginxPod(nginxRelease)
			brokenRESTConfig, err := cluster.GetRESTConfig()
			Expect(err).ToNot(HaveOccurred())
			brokenRESTConfig.Host = ""
			pf, err := NewPortForward(brokenRESTConfig, pod, PortAny, 8080)
			Expect(err).To(HaveOccurred())
			Expect(pf).To(BeNil())
		})
		Context("missing CA", func() {
			pod := mustGetReadyNginxPod(nginxRelease)
			brokenRESTConfig, err := cluster.GetRESTConfig()
			Expect(err).ToNot(HaveOccurred())
			brokenRESTConfig.CAFile = ""
			brokenRESTConfig.CAData = []byte{}
			pf, err := NewPortForward(brokenRESTConfig, pod, PortAny, 8080)
			Expect(err).To(HaveOccurred())
			Expect(pf).To(BeNil())
		})
	})
})

var _ = Describe("GetPodLogs", func() {
	It("can get logs of existing pod", func() {
		pod := mustGetReadyNginxPod(nginxRelease)
		logs, err := GetPodLogsString(restConfig, pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(logs)).To(BeNumerically(">", 0))
	})
	It("fails with invalid REST config", func() {
		pod := mustGetReadyNginxPod(nginxRelease)
		brokenRESTConfig, err := cluster.GetRESTConfig()
		Expect(err).ToNot(HaveOccurred())
		brokenRESTConfig.CAFile = ""
		brokenRESTConfig.CAData = []byte{}
		_, err = GetPodLogsString(brokenRESTConfig, pod)
		Expect(err).To(HaveOccurred())
	})
})
