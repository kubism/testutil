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
	"time"

	"github.com/kubism/testutil/pkg/fs"
	"github.com/kubism/testutil/pkg/rand"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/log"
)

// Re-export cluster configuration for easier use
type Config = v1alpha4.Cluster
type DebugLog = func(string, ...interface{})

// Helper to provide a simplified interface. Use just needs to pass in a printf
// function.
type debugLogger struct {
	Log DebugLog
}

func (d debugLogger) Warn(message string) {
	d.Log("WARN: %s", message)
}

func (d debugLogger) Warnf(format string, args ...interface{}) {
	d.Log("WARN: "+format, args...)
}

func (d debugLogger) Error(message string) {
	d.Log("ERROR: %s", message)
}

func (d debugLogger) Errorf(format string, args ...interface{}) {
	d.Log("ERROR: "+format, args...)
}

func (d debugLogger) Enabled() bool { return true }

func (d debugLogger) Info(message string) {
	d.Log("INFO: %s", message)
}

func (d debugLogger) Infof(format string, args ...interface{}) {
	d.Log("INFO: "+format, args...)
}

func (d debugLogger) V(level log.Level) log.InfoLogger {
	return d
}

type clusterOptions struct {
	ProviderOpts []cluster.ProviderOption
	ClusterOpts  []cluster.CreateOption
	Name         string
	UseExisting  bool
	DoNotDelete  bool
}

type ClusterOption interface {
	apply(*clusterOptions)
}

type clusterOptionAdapter func(*clusterOptions)

func (c clusterOptionAdapter) apply(o *clusterOptions) {
	c(o)
}

func ClusterWithName(name string) ClusterOption {
	return clusterOptionAdapter(func(o *clusterOptions) {
		o.Name = name
	})
}

func ClusterUseExisting() ClusterOption {
	return clusterOptionAdapter(func(o *clusterOptions) {
		o.UseExisting = true
	})
}

func ClusterDoNotDelete() ClusterOption {
	return clusterOptionAdapter(func(o *clusterOptions) {
		o.DoNotDelete = true
	})
}

func ClusterWithWaitForReady(waitTime time.Duration) ClusterOption {
	return clusterOptionAdapter(func(o *clusterOptions) {
		o.ClusterOpts = append(o.ClusterOpts, cluster.CreateWithWaitForReady(waitTime))
	})
}

func ClusterWithConfig(config *Config) ClusterOption {
	return clusterOptionAdapter(func(o *clusterOptions) {
		o.ClusterOpts = append(o.ClusterOpts, cluster.CreateWithV1Alpha4Config(config))
	})
}

func ClusterWithDocker() ClusterOption {
	return clusterOptionAdapter(func(o *clusterOptions) {
		o.ProviderOpts = append(o.ProviderOpts, cluster.ProviderWithDocker())
	})
}

func ClusterWithPodman() ClusterOption {
	return clusterOptionAdapter(func(o *clusterOptions) {
		o.ProviderOpts = append(o.ProviderOpts, cluster.ProviderWithPodman())
	})
}

func ClusterWithDebugLog(debugLog DebugLog) ClusterOption {
	return clusterOptionAdapter(func(o *clusterOptions) {
		o.ProviderOpts = append(o.ProviderOpts, cluster.ProviderWithLogger(debugLogger{debugLog}))
	})
}

type Cluster struct {
	Name        string
	provider    *cluster.Provider
	doNotDelete bool
}

func NewCluster(opts ...ClusterOption) (*Cluster, error) {
	o := clusterOptions{ // default options
		ProviderOpts: []cluster.ProviderOption{
			cluster.ProviderWithDocker(),
		},
		ClusterOpts: []cluster.CreateOption{
			cluster.CreateWithNodeImage("kindest/node:v1.16.4"),
		},
		Name: rand.String(10),
	}
	for _, opt := range opts {
		opt.apply(&o)
	}
	provider := cluster.NewProvider(o.ProviderOpts...)
	exists := false
	if o.UseExisting {
		names, err := provider.List()
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			if name == o.Name {
				exists = true
				break
			}
		}
	}
	if !exists {
		err := provider.Create(o.Name, o.ClusterOpts...)
		if err != nil {
			return nil, err
		}
	}
	return &Cluster{
		Name:        o.Name,
		provider:    provider,
		doNotDelete: o.DoNotDelete,
	}, nil
}

func (c *Cluster) GetKubeConfig() (string, error) {
	return c.provider.KubeConfig(c.Name, false)
}

func (c *Cluster) GetKubeConfigAsTempFile() (*fs.TempFile, error) {
	content, err := c.GetKubeConfig()
	if err != nil {
		return nil, err
	}
	return fs.NewTempFile([]byte(content))
}

func (c *Cluster) GetRESTConfig() (*rest.Config, error) {
	kubeConfig, err := c.GetKubeConfig()
	if err != nil {
		return nil, err
	}
	return clientcmd.RESTConfigFromKubeConfig([]byte(kubeConfig))
}

func (c *Cluster) GetClient() (client.Client, error) {
	config, err := c.GetRESTConfig()
	if err != nil {
		return nil, err
	}
	return client.New(config, client.Options{Scheme: scheme.Scheme})
}

func (c *Cluster) Close() error {
	if !c.doNotDelete {
		return c.provider.Delete(c.Name, "")
	} else {
		return nil
	}
}
