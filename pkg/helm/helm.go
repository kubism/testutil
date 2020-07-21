package helm

import (
	"net/url"
	"os"
	"path/filepath"

	"github.com/kubism/testutil/pkg/fs"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	repoFileName = "repositories.yaml"
)

type DebugLog = action.DebugLog

type RepositoryEntry = repo.Entry

type Chart = chart.Chart

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

type clientOptions struct {
	Namespace string
	Driver    string
	DebugLog  DebugLog
}

type ClientOption interface {
	apply(*clientOptions) error
}

type clientOptionAdapter func(*clientOptions) error

func (c clientOptionAdapter) apply(o *clientOptions) error {
	return c(o)
}

func ClientWithNamespace(namespace string) ClientOption {
	return clientOptionAdapter(func(o *clientOptions) error {
		o.Namespace = namespace
		return nil
	})
}

func ClientWithDriver(driver string) ClientOption {
	return clientOptionAdapter(func(o *clientOptions) error {
		o.Driver = driver
		return nil
	})
}

func ClientWithDebugLog(debugLog DebugLog) ClientOption {
	return clientOptionAdapter(func(o *clientOptions) error {
		o.DebugLog = debugLog
		return nil
	})
}

type Client struct {
	restConfig *rest.Config
	options    clientOptions
	tempDir    *fs.TempDir
	repoFile   *repo.File
	indexFiles map[string]*repo.IndexFile
}

func NewClient(restConfig *rest.Config, opts ...ClientOption) (*Client, error) {
	options := clientOptions{
		Namespace: "",
		Driver:    "secrets",
	}
	for _, opt := range opts {
		err := opt.apply(&options)
		if err != nil {
			return nil, err
		}
	}
	tempDir, err := fs.NewTempDir()
	if err != nil {
		return nil, err
	}
	c := &Client{
		restConfig: restConfig,
		options:    options,
		tempDir:    tempDir,
		repoFile:   repo.NewFile(),
		indexFiles: map[string]*repo.IndexFile{},
	}
	if err := os.Mkdir(c.getCacheDir(), 0755); err != nil {
		c.Close()
		return nil, err
	}
	if err := c.writeRepoFile(); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
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

func (c *Client) getCacheDir() string {
	return filepath.Join(c.tempDir.Path, "cache")
}

func (c *Client) writeRepoFile() error {
	return c.repoFile.WriteFile(filepath.Join(c.tempDir.Path, "repositories.yaml"), 0755)
}

func (c *Client) List() ([]*release.Release, error) {
	ac, err := c.getActionConfig()
	if err != nil {
		return nil, err
	}
	list := action.NewList(ac)
	return list.Run()
}

func (c *Client) AddRepository(cfg *RepositoryEntry) error {
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return errors.Errorf("invalid chart URL format: %s", cfg.URL)
	}
	getters := getter.Providers{
		getter.Provider{
			Schemes: []string{"http", "https"},
			New:     getter.NewHTTPGetter,
		},
	}
	client, err := getters.ByScheme(u.Scheme)
	if err != nil {
		return errors.Errorf("could not find protocol handler for: %s", u.Scheme)
	}
	r := &repo.ChartRepository{
		Config:    cfg,
		IndexFile: repo.NewIndexFile(),
		Client:    client,
		CachePath: c.getCacheDir(),
	}
	fname, err := r.DownloadIndexFile()
	if err != nil {
		return err
	}
	indexFile, err := repo.LoadIndexFile(fname)
	if err != nil {
		return err
	}
	c.repoFile.Add(cfg)
	c.indexFiles[cfg.Name] = indexFile
	return c.writeRepoFile()
}

type installOptions struct {
	*action.Install
}

type InstallOption interface {
	apply(*installOptions) error
}

type installOptionAdapter func(*installOptions) error

func (c installOptionAdapter) apply(o *installOptions) error {
	return c(o)
}

func (c *Client) Install(chart, version string, values map[string]interface{}, opts ...InstallOption) (*release.Release, error) {
	return nil, nil
}

func (c *Client) Close() error {
	return c.tempDir.Close()
}
