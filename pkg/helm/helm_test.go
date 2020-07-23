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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	stableRepository = &RepositoryEntry{
		Name: "stable",
		URL:  "https://kubernetes-charts.storage.googleapis.com",
	}
)

func mustNewClient(opts ...ClientOption) *Client {
	client, err := NewClient(kubeConfig, opts...)
	Expect(err).To(Succeed())
	return client
}

var _ = Describe("Client", func() {
	It("can be used with kind cluster", func() {
		client := mustNewClient()
		defer client.Close()
		_, err := client.List()
		Expect(err).ToNot(HaveOccurred())
	})
	It("can add repository and install chart", func() {
		client := mustNewClient()
		defer client.Close()
		Expect(client.AddRepository(stableRepository)).To(Succeed())
		rls, err := client.Install("stable/minio", "", ValuesOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(rls).ToNot(BeNil())
		defer client.Uninstall(rls.Name) // nolint:errcheck
	})
	It("can be used with options", func() {
		client := mustNewClient(
			ClientWithDebugLog(func(format string, v ...interface{}) {}),
			ClientWithDriver("secret"),
			ClientWithNamespace("default"),
		)
		Expect(client.AddRepository(stableRepository)).To(Succeed())
		name := "predefined"
		rls, err := client.Install("stable/minio", "", ValuesOptions{},
			InstallWithReleaseName(name),
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(rls).ToNot(BeNil())
		Expect(client.Uninstall(name)).To(Succeed())
	})
})
