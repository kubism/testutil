package kind

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestKind(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "kind")
}
