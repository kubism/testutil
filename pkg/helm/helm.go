package helm

import (
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

// restClientGetter based on: https://github.com/helm/helm/issues/6910#issuecomment-601277026

type restClientGetter struct {
	Namespace  string
	RESTConfig *rest.Config
}

func (c *restClientGetter) ToRESTConfig() (*rest.Config, error) {
	return c.RESTConfig, nil
}

func (c *restClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	config, err := c.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	discoveryClient, _ := discovery.NewDiscoveryClientForConfig(config)
	return memory.NewMemCacheClient(discoveryClient), nil
}

func (c *restClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := c.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return expander, nil
}

func (c *restClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
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
	restConfig *rest.Config
	options    options
}

func NewClient(restConfig *rest.Config, opts ...Option) (*Client, error) {
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
		restConfig: restConfig,
		options:    options,
	}, nil
}

func (c *Client) getActionConfig() (*action.Configuration, error) {
	ac := new(action.Configuration)
	cg := &restClientGetter{
		Namespace:  c.options.Namespace,
		RESTConfig: c.restConfig,
	}
	if err := ac.Init(cg, c.options.Namespace, c.options.Driver, c.options.DebugLog); err != nil {
		return nil, err
	}
	return ac, nil
}

func (c *Client) List() ([]*release.Release, error) {
	ac, err := c.getActionConfig()
	if err != nil {
		return nil, err
	}
	list := action.NewList(ac)
	return list.Run()
}
