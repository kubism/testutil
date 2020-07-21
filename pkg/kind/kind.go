package kind

import (
	"time"

	"github.com/kubism/testutil/pkg/fs"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/log"
)

// Re-export cluster configuration for easier use
type Config = v1alpha4.Cluster
type NoopLogger = log.NoopLogger

type options struct {
	ProviderOpts []cluster.ProviderOption
	ClusterOpts  []cluster.CreateOption
}

type Option interface {
	apply(*options) error
}

type optionAdapter func(*options) error

func (c optionAdapter) apply(o *options) error {
	return c(o)
}

func WithWaitForReady(waitTime time.Duration) Option {
	return optionAdapter(func(o *options) error {
		o.ClusterOpts = append(o.ClusterOpts, cluster.CreateWithWaitForReady(waitTime))
		return nil
	})
}

func WithConfig(config *Config) Option {
	return optionAdapter(func(o *options) error {
		o.ClusterOpts = append(o.ClusterOpts, cluster.CreateWithV1Alpha4Config(config))
		return nil
	})
}

func WithDocker() Option {
	return optionAdapter(func(o *options) error {
		o.ProviderOpts = append(o.ProviderOpts, cluster.ProviderWithDocker())
		return nil
	})
}

func WithPodman() Option {
	return optionAdapter(func(o *options) error {
		o.ProviderOpts = append(o.ProviderOpts, cluster.ProviderWithPodman())
		return nil
	})
}

func WithLogger(logger log.Logger) Option {
	return optionAdapter(func(o *options) error {
		o.ProviderOpts = append(o.ProviderOpts, cluster.ProviderWithLogger(logger))
		return nil
	})
}

type Cluster struct {
	Name     string
	provider *cluster.Provider
}

func NewCluster(name string, opts ...Option) (*Cluster, error) {
	o := options{ // default options
		ProviderOpts: []cluster.ProviderOption{
			cluster.ProviderWithDocker(),
		},
		ClusterOpts: []cluster.CreateOption{
			cluster.CreateWithNodeImage("kindest/node:v1.16.4"),
		},
	}
	for _, opt := range opts {
		err := opt.apply(&o)
		if err != nil {
			return nil, err
		}
	}
	provider := cluster.NewProvider(o.ProviderOpts...)
	err := provider.Create(name, o.ClusterOpts...)
	if err != nil {
		return nil, err
	}
	return &Cluster{
		Name:     name,
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

func (c *Cluster) Close() error {
	return c.provider.Delete(c.Name, "")
}
