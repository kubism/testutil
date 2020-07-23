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

package fs

import (
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	expectedFileContent = []byte("TESTCONTENT")
)

func checkPathExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	} else { // Schrodinger's file...
		return false
	}
}

var _ = Describe("TempFile", func() {
	It("creates the file with expected content", func() {
		tf, err := NewTempFile(expectedFileContent)
		Expect(err).ToNot(HaveOccurred())
		Expect(tf).ToNot(BeNil())
		defer tf.Close()
		Expect(checkPathExists(tf.Path)).To(Equal(true))
		content, err := ioutil.ReadFile(tf.Path)
		Expect(err).ToNot(HaveOccurred())
		Expect(content).To(Equal(expectedFileContent))
	})
	It("is properly cleaned up", func() {
		tf, err := NewTempFile(expectedFileContent)
		Expect(err).ToNot(HaveOccurred())
		Expect(tf).ToNot(BeNil())
		tf.Close()
		Expect(checkPathExists(tf.Path)).To(Equal(false))
	})
})

var _ = Describe("TempDir", func() {
	It("creates the dir", func() {
		td, err := NewTempDir()
		Expect(err).ToNot(HaveOccurred())
		Expect(td).ToNot(BeNil())
		defer td.Close()
		Expect(checkPathExists(td.Path)).To(Equal(true))
	})
	It("is properly cleaned up", func() {
		td, err := NewTempDir()
		Expect(err).ToNot(HaveOccurred())
		Expect(td).ToNot(BeNil())
		td.Close()
		Expect(checkPathExists(td.Path)).To(Equal(false))
	})
})
