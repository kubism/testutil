package helm

import (
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

// Based on: https://github.com/helm/helm/issues/6910#issuecomment-601277026

type simpleRESTClientGetter struct {
	Namespace  string
	RESTConfig *rest.Config
}

func NewRESTClientGetter(namespace string, restConfig *rest.Config) genericclioptions.RESTClientGetter {
	return &simpleRESTClientGetter{
		Namespace:  namespace,
		RESTConfig: restConfig,
	}
}

func (c *simpleRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return c.RESTConfig, nil
}

func (c *simpleRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	config, err := c.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	discoveryClient, _ := discovery.NewDiscoveryClientForConfig(config)
	return memory.NewMemCacheClient(discoveryClient), nil
}

func (c *simpleRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := c.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return expander, nil
}

func (c *simpleRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// use the standard defaults for this client command
	// DEPRECATED: remove and replace with something more accurate
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}
	overrides.Context.Namespace = c.Namespace
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
}

type DebugLog = action.DebugLog

type options struct {
	Namespace string
	Driver    string
	DebugLog  DebugLog
}

type Option interface {
	apply(*options) error
}

type optionAdapter func(*options) error

func (c optionAdapter) apply(o *options) error {
	return c(o)
}

func WithNamespace(namespace string) Option {
	return optionAdapter(func(o *options) error {
		o.Namespace = namespace
		return nil
	})
}

func WithDriver(driver string) Option {
	return optionAdapter(func(o *options) error {
		o.Driver = driver
		return nil
	})
}

func WithDebugLog(debugLog DebugLog) Option {
	return optionAdapter(func(o *options) error {
		o.DebugLog = debugLog
		return nil
	})
}

type Client struct {
	kubeConfig string
	options    options
}

func NewClient(kubeConfig string, opts ...Option) (*Client, error) {
	options := options{
		Namespace: "",
		Driver:    "secrets",
	}
	for _, opt := range opts {
		err := opt.apply(&options)
		if err != nil {
			return nil, err
		}
	}
	return &Client{
		kubeConfig: kubeConfig,
		options:    options,
	}, nil
}
