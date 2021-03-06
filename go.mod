module github.com/kubism/testutil

go 1.14

require (
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/pkg/errors v0.9.1
	helm.sh/helm/v3 v3.2.4
	k8s.io/api v0.18.4
	k8s.io/apimachinery v0.18.8
	k8s.io/cli-runtime v0.18.4
	k8s.io/client-go v0.18.4
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/controller-runtime v0.6.1
	sigs.k8s.io/kind v0.9.0
)
