package helm

import (
	"net/url"
	"os"
	"path/filepath"

	"github.com/kubism/testutil/pkg/fs"
	"github.com/kubism/testutil/pkg/rand"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
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

type DebugLog = action.DebugLog

type RepositoryEntry = repo.Entry

type Chart = chart.Chart

type ValuesOptions = values.Options

type restClientGetter struct {
	Namespace  string
	KubeConfig string
}

func (c *restClientGetter) ToRESTConfig() (*rest.Config, error) {
	return clientcmd.RESTConfigFromKubeConfig([]byte(c.KubeConfig))
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
	clientConfig, err := clientcmd.NewClientConfigFromBytes([]byte(c.KubeConfig))
	if err != nil {
		panic(err) // TODO: is there a way to avoid this panic?
	}
	return clientConfig
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
	kubeConfig   string
	options      clientOptions
	actionConfig *action.Configuration
	tempDir      *fs.TempDir
	repoFile     *repo.File
}

func NewClient(kubeConfig string, opts ...ClientOption) (*Client, error) {
	options := clientOptions{
		Namespace: "default",
		Driver:    "secrets",
		DebugLog:  func(format string, v ...interface{}) {},
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
	actionConfig := new(action.Configuration)
	clientGetter := &restClientGetter{
		Namespace:  options.Namespace,
		KubeConfig: kubeConfig,
	}
	if err := actionConfig.Init(clientGetter, options.Namespace, options.Driver, options.DebugLog); err != nil {
		return nil, err
	}
	c := &Client{
		kubeConfig:   kubeConfig,
		options:      options,
		actionConfig: actionConfig,
		tempDir:      tempDir,
		repoFile:     repo.NewFile(),
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

func (c *Client) getCacheDir() string {
	return filepath.Join(c.tempDir.Path, "cache")
}

func (c *Client) getRepoFilePath() string {
	return filepath.Join(c.tempDir.Path, "repositories.yaml")
}

func (c *Client) writeRepoFile() error {
	return c.repoFile.WriteFile(c.getRepoFilePath(), 0755)
}

func (c *Client) List() ([]*release.Release, error) {
	list := action.NewList(c.actionConfig)
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
	_, err = r.DownloadIndexFile()
	if err != nil {
		return err
	}
	c.repoFile.Add(cfg)
	return c.writeRepoFile()
}

func (c *Client) createEnvSettings(namespace string) *cli.EnvSettings {
	os.Setenv("HELM_NAMESPACE", namespace)
	os.Setenv("HELM_PLUGINS", filepath.Join(c.tempDir.Path, "plugins"))
	os.Setenv("HELM_REGISTRY_CONFIG", filepath.Join(c.tempDir.Path, "registry.json"))
	os.Setenv("HELM_REPOSITORY_CONFIG", c.getRepoFilePath())
	os.Setenv("HELM_REPOSITORY_CACHE", c.getCacheDir())
	return cli.New()
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

func InstallWithReleaseName(name string) InstallOption {
	return installOptionAdapter(func(o *installOptions) error {
		o.ReleaseName = name
		return nil
	})
}

// TODO: proper options, e.g. InstallWithReleaseName, ...

func (c *Client) Install(chartName, version string, valuesOptions ValuesOptions, opts ...InstallOption) (*release.Release, error) {
	options := installOptions{action.NewInstall(c.actionConfig)}
	options.ReleaseName = rand.String(5)
	options.Namespace = "default"
	options.Version = version
	for _, opt := range opts {
		err := opt.apply(&options)
		if err != nil {
			return nil, err
		}
	}
	settings := c.createEnvSettings(options.Namespace)
	fname, err := options.LocateChart(chartName, settings)
	if err != nil {
		return nil, err
	}
	chart, err := loader.Load(fname)
	if err != nil {
		return nil, err
	}
	getters := getter.Providers{
		getter.Provider{
			Schemes: []string{"http", "https"},
			New:     getter.NewHTTPGetter,
		},
	}
	values, err := valuesOptions.MergeValues(getters)
	if err != nil {
		return nil, err
	}
	return options.Run(chart, values)
}

func (c *Client) Uninstall(releaseName string) error {
	uninstall := action.NewUninstall(c.actionConfig)
	_, err := uninstall.Run(releaseName)
	return err
}

func (c *Client) Close() error {
	return c.tempDir.Close()
}
