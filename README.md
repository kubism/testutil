# `testutil`

[![Go Documentation](https://img.shields.io/badge/go-doc-blue.svg?style=flat)](https://pkg.go.dev/github.com/kubism/testutil/pkg)
[![Build Status](https://travis-ci.org/kubism/testutil.svg?branch=master)](https://travis-ci.org/kubism/testutil)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubism/testutil)](https://goreportcard.com/report/github.com/kubism/testutil)
[![Coverage Status](https://coveralls.io/repos/github/kubism/testutil/badge.svg?branch=master)](https://coveralls.io/github/kubism/testutil?branch=master)
[![Maintainability](https://api.codeclimate.com/v1/badges/b75c438fc0c263a21024/maintainability)](https://codeclimate.com/github/kubism/testutil/maintainability)

TODO: write introduction

## Capabilities

* kind.Cluster
* helm.Client
    * AddRepository
    * Install
    * Uninstall
* kube.WaitUntilPodReady
* kube.WaitUntilDeploymentScheduled
* kube.WaitUntilDeploymentReady
* kube.GetEvents
* kube.GetPodLogs
* kube.PortForward
* Jobs, CronJobs, ReplicaSets, (TODO: StatefulSets, DaemonSets)

## Notes (temporary)

* uses panic do not use in live code just tests
* `make TEST_FLAGS="-kind-cluster=testutil" test`

