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

package helm

import (
	"testing"
	"time"

	"github.com/kubism/testutil/internal/flags"
	"github.com/kubism/testutil/pkg/kind"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	kubeConfig string
	cluster    *kind.Cluster
)

func TestHelm(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "helm")
}

var _ = BeforeSuite(func(done Done) {
	var err error
	By("setup kind cluster")
	clusterOptions := []kind.ClusterOption{
		kind.ClusterWithWaitForReady(3 * time.Minute),
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
	By("setup kubeconfig")
	kubeConfig, err = cluster.GetKubeConfig()
	Expect(err).To(Succeed())
	close(done)
}, 240)

var _ = AfterSuite(func() {
	By("tearing down kind cluster")
	if cluster != nil {
		cluster.Close()
	}
})
