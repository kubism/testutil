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
type NoopLogger = log.NoopLogger

type clusterOptions struct {
	ProviderOpts []cluster.ProviderOption
	ClusterOpts  []cluster.CreateOption
	Name         string
	UseExisting  bool
	DoNotDelete  bool
} // TODO: add options: "use existing option" and "do not delete cluster"

type ClusterOption interface {
	apply(*clusterOptions) error
}

type clusterOptionAdapter func(*clusterOptions) error

func (c clusterOptionAdapter) apply(o *clusterOptions) error {
	return c(o)
}

func ClusterWithName(name string) ClusterOption {
	return clusterOptionAdapter(func(o *clusterOptions) error {
		o.Name = name
		return nil
	})
}

func ClusterWithWaitForReady(waitTime time.Duration) ClusterOption {
	return clusterOptionAdapter(func(o *clusterOptions) error {
		o.ClusterOpts = append(o.ClusterOpts, cluster.CreateWithWaitForReady(waitTime))
		return nil
	})
}

func ClusterWithConfig(config *Config) ClusterOption {
	return clusterOptionAdapter(func(o *clusterOptions) error {
		o.ClusterOpts = append(o.ClusterOpts, cluster.CreateWithV1Alpha4Config(config))
		return nil
	})
}

func ClusterWithDocker() ClusterOption {
	return clusterOptionAdapter(func(o *clusterOptions) error {
		o.ProviderOpts = append(o.ProviderOpts, cluster.ProviderWithDocker())
		return nil
	})
}

func ClusterWithPodman() ClusterOption {
	return clusterOptionAdapter(func(o *clusterOptions) error {
		o.ProviderOpts = append(o.ProviderOpts, cluster.ProviderWithPodman())
		return nil
	})
}

func ClusterWithLogger(logger log.Logger) ClusterOption {
	return clusterOptionAdapter(func(o *clusterOptions) error {
		o.ProviderOpts = append(o.ProviderOpts, cluster.ProviderWithLogger(logger))
		return nil
	})
}

type Cluster struct {
	Name     string
	provider *cluster.Provider
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
		err := opt.apply(&o)
		if err != nil {
			return nil, err
		}
	}
	provider := cluster.NewProvider(o.ProviderOpts...)
	err := provider.Create(o.Name, o.ClusterOpts...)
	if err != nil {
		return nil, err
	}
	return &Cluster{
		Name:     o.Name,
		provider: provider,
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
	return c.provider.Delete(c.Name, "")
}
