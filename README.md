# `testutil`

[![Go Documentation](https://img.shields.io/badge/go-doc-blue.svg?style=flat)](https://pkg.go.dev/github.com/kubism/testutil/pkg)
[![Build Status](https://travis-ci.org/kubism/testutil.svg?branch=master)](https://travis-ci.org/kubism/testutil)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubism/testutil)](https://goreportcard.com/report/github.com/kubism/testutil)
[![Coverage Status](https://coveralls.io/repos/github/kubism/testutil/badge.svg?branch=master)](https://coveralls.io/github/kubism/testutil?branch=master)
[![Maintainability](https://api.codeclimate.com/v1/badges/b75c438fc0c263a21024/maintainability)](https://codeclimate.com/github/kubism/testutil/maintainability)

This library is a collection of helpers to ease implementing integration tests 
utilizing [kind](https://github.com/kubernetes-sigs/kind), [helm](https://github.com/helm/helm) 
and [kubernetes](https://github.com/kubernetes/kubernetes) without the need to install any CLI tools.

If it improves the code readability and ergonomics, the library will utilize
`panic` for rare errors, e.g. filesystem issues, and performance is not a
primary requirement.
Therefore it should not be used as part of a production service.

## Getting started

Before we can interact with a kubernetes cluster, we have to either create one
or connect to an existing one. If you just want to connect to an external cluster
feel free to skip the following section.

### Creating a kind cluster

Create a new kind cluster with a randomized name is as simple as running:
```go
cluster, err := kind.NewCluster()
if err != nil {}
defer cluster.Close()
```
However in most use-cases you want to provide more options to kind to
configure it as you require it. You might want to set a deadline for the creation
or use an existing cluster.
```go
clusterOptions := []kind.ClusterOption{
    kind.ClusterWithWaitForReady(2*time.Minute),
}
if existingClusterName != "" {
    clusterOptions = append(clusterOptions,
        kind.ClusterWithName(existingClusterName),
        kind.ClusterUseExisting(),
        kind.ClusterDoNotDelete(),
    )
}
cluster, err := kind.NewCluster(clusterOptions...)
if err != nil {}
defer cluster.Close() // If do not delete is set, the cluster will not be deleted
```

The node image used by kind can be specified via `kind.ClusterWithNodeImage`.
By default this will fallback to a sensible default, but should be set to the
kubernetes version of your choice.

If you require even deeper configurability, you can pass in a
[cluster definition](https://godoc.org/github.com/kubernetes-sigs/kind/pkg/apis/config/v1alpha4#Cluster)
via `kind.ClusterWithConfig`.

### Setting up helm

Before the helm-client can be setup you have to retrieve the raw kubeconfig.

__NOTE__:_This will most likely change to `rest.Config` at some point in the future._

For an existing `kind.Cluster` this will look as follows:
```go
kubeConfig, err := cluster.GetKubeConfig()
if err != nil {}
```
With the kubeconfig at our disposal the client can then be created:
```go
helmClient, err = helm.NewClient(kubeConfig)
if err != nil {}
defer helmClient.Close()
```
It is important to always clean up the `helm.Client` via `Close` as it manages
its own temporary cache and repository indices, which will otherwise leak.

To be able to install a remote chart, you will have to add your repositories, e.g.:
```go
err := helmClient.AddRepository(&helm.RepositoryEntry{
    Name: "bitnami",
    URL:  "https://charts.bitnami.com/bitnami",
})
if err != nil {}
```

### Installing a chart

You can install both remote and local charts. In context of this getting started
guide we will install the `bitnami/nginx`-chart:
```go
rls, err := helmClient.Install("bitnami/nginx", "", helm.ValuesOptions{})
if err != nil {}
```
The second parameter is the `version` to use. If it is an empty string, the most
recent version will be installed.

`helm.ValuesOptions` is a struct offering several ways to provide both from
filesystem and by definition. Before those are used they are merged according
to helm's conventions.

By default a release name will be generated, but can be retrieved from the
returned `helm.Release`.

The install procedure can be further customized using `InstallOption`s. For
example to use a predefined release name:
```go
rls, err := client.Install("bitnami/nginx", "", ValuesOptions{},
    InstallWithReleaseName(name),
)
if err != nil {}
```

### Interacting with kubernetes objects

For most tests you will not stop after installing a helm chart, but rather
you want to interact with the resulting objects.

So to show off some of the available helpers, let's create a `kube.Client`
(also embed the kubernetes controller-runtime client) and illustrate some
of its usage.
```go
restConfig, err = cluster.GetRESTConfig()
if err != nil {}
k8sClient, err = NewClient(restConfig)
if err != nil {}
```
Let's create a context with a timeout first, so that commands can not exceed
a deadline in our tests:
```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
defer cancel()
```
The installed `nginx` has a Deployment, so let's retrieve it first:
```go
deployment := kube.DeploymentWithNamespacedName(rls.Namespace, rls.Name+"-nginx")
err := k8sClient.Get(ctx, NamespacedName(deployment), deployment)
if err != nil {}
```
We want to retrieve the scheduled pod, but at the beginning it won't be available,
so let's wait until the deployment is scheduled.
```go
err := k8sClient.WaitUntil(ctx, kube.DeploymentIsScheduled(deployment))
if err != nil {}
```
Once we know the deployment was scheduled, we can either retrieve pods with the
required labels via `k8sClient.List` or for demonstration purposes find the
replicasets for the deployment and then the pods for the active replicaset
(using `k8sClient.ListForOwner` also particularly useful for jobs):
```go
replicaSetList := &appsv1.ReplicaSetList{}
err := k8sClient.ListForOwner(context.Background(), replicaSetList, deployment)
if err != nil {}
if len(replicaSetList) < 1 {}
replicaSet := &replicaSetList.Items[0]
err = k8sClient.WaitUntil(ctx, kube.ReplicaSetIsAvailable(replicaSet), kube.ReplicaSetIsReady(replicaSet))
if err != nil {}
podList := &corev1.PodList{}
err = k8sClient.ListForOwner(ctx, podList, replicaSet)
if err != nil {}
if len(podList) < 1 {}
pod := &podList.Items[0]
```
Now we actually retrieved our nginx pod. At this point we probably want to
interact with the pod, e.g. create a port forward, however let's wait until the
pod it ready to retrieve traffic first:
```go
err := k8sClient.WaitUntil(ctx, kube.PodIsReady(&pod))
if err != nil {}
```
The pod is ready, so we can create a port-forward:
```go
pf, err := k8sClient.PortForward(pod, PortAny, 8080)
if err != nil {}
defer pf.Close()
```
The above code will use an available port on the host rather than a predefined one.
So connecting to nginx might look as follows:
```go
_, err := http.Get(fmt.Sprintf("http://localhost:%d", pf.LocalPort))
```

For some integration tests it might make sense to retrieve logs of a pod,
conveniently `k8sClient.Logs` is here to help:
```go
logs, err := k8sClient.LogsString(ctx, pod)
```

Last but not least events for a specific object can be retrieved using
`k8sClient.Events`. This is particularly useful for operators which interact
with several resources and create events for their interactions:
```go
events, err := k8sClient.Events(ctx, pod)
```

## Notes (temporary)

* uses panic do not use in live code just tests
* `make TEST_FLAGS="-kind-cluster=testutil" test`
* still missing ds, statefulsets

