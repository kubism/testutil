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

package kind

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func mustNewCluster(opts ...ClusterOption) *Cluster {
	cluster, err := NewCluster(opts...)
	Expect(err).To(Succeed())
	return cluster
}

var _ = Describe("Cluster", func() {
	It("can be created", func() {
		Context("with no options", func() {
			cluster := mustNewCluster()
			Expect(cluster.Close()).To(Succeed())
		})
	})
	It("is functional", func() {
		cluster := mustNewCluster(ClusterWithWaitForReady(3 * time.Minute))
		defer cluster.Close()
		config, err := cluster.GetRESTConfig()
		Expect(err).NotTo(HaveOccurred())
		Expect(config).ToNot(BeNil())
		k8sClient, err := client.New(config, client.Options{Scheme: scheme.Scheme})
		Expect(err).ToNot(HaveOccurred())
		Expect(k8sClient).ToNot(BeNil())
		var ns corev1.Namespace
		Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: "kube-system"}, &ns)).To(Succeed())
	})
})
