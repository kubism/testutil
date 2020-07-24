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

// Package helm provides utility to leverage helm in tests without requiring
// the CLI.
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

// TODO: Currently the implementation lacks a lot of helm functionality.
//       One interesting one would be: support for plugins

// We are not using the CLI environment, so we make the relevant providers
// available here.
var getters = getter.Providers{
	getter.Provider{
		Schemes: []string{"http", "https"},
		New:     getter.NewHTTPGetter,
	},
}

// DebugLog is the function declaration required to capture log output from
// the Client.
type DebugLog = action.DebugLog

// RepositoryEntry represents the collection of parameters to define a new
// entry of a helm repository.
type RepositoryEntry = repo.Entry

// Chart is a helm package that contains metadata, a default config, zero or
// more optionally parameterizable templates, and zero or more charts (dependencies).
type Chart = chart.Chart

// ValuesOptions provides several options to provide values configuration for
// helm. All contained values can then be conveniently merged.
type ValuesOptions = values.Options

// Release describes a deployment of a chart, together with the chart and
// the variables used to deploy that chart.
type Release = release.Release

// restClientGetter a simple wrapper providing the RESTClientGetter-interface
// using only the raw kubeconfig.
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

// clientOptions represents the internal configuration state.
type clientOptions struct {
	Namespace string
	Driver    string
	DebugLog  DebugLog
}

// ClientOption interface is implemented by all possible options to instantiate
// a new helm client.
type ClientOption interface {
	apply(*clientOptions)
}

type clientOptionAdapter func(*clientOptions)

func (c clientOptionAdapter) apply(o *clientOptions) {
	c(o)
}

// ClientWithNamespace will adapt the default namespace used as a fallback
// by the helm client.
func ClientWithNamespace(namespace string) ClientOption {
	return clientOptionAdapter(func(o *clientOptions) {
		o.Namespace = namespace
	})
}

// ClientWithDriver will change the helm storage driver. By default it will
// be 'secret', but 'configmap' and 'memory' are other sensible values.
func ClientWithDriver(driver string) ClientOption {
	return clientOptionAdapter(func(o *clientOptions) {
		o.Driver = driver
	})
}

// ClientWithDebugLog will use the provided function to output debug logs of helm.
func ClientWithDebugLog(debugLog DebugLog) ClientOption {
	return clientOptionAdapter(func(o *clientOptions) {
		o.DebugLog = debugLog
	})
}

// Client represents the temporary helm environment.
type Client struct {
	kubeConfig   string
	options      clientOptions
	actionConfig *action.Configuration
	tempDir      *fs.TempDir
	repoFile     *repo.File
}

// NewClient will create a new helm client providing an isolated helm environment.
// Only a valid kubeconfig is required. Make sure to properly clean up temporary
// files by calling Close once finished.
func NewClient(kubeConfig string, opts ...ClientOption) (*Client, error) {
	options := clientOptions{ // Default options
		Namespace: "default",
		Driver:    "secrets",
		DebugLog:  func(format string, v ...interface{}) {},
	}
	for _, opt := range opts {
		opt.apply(&options)
	}
	// Create actionConfig, which will be used for all actions of this helm client.
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
		repoFile:     repo.NewFile(),
	}
	if err := c.setupDirs(); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// setupDirs is a small helper, which will setup all directories and files used
// by this instance. Should only be called once by NewClient!
func (c *Client) setupDirs() error {
	var err error
	c.tempDir, err = fs.NewTempDir()
	if err != nil {
		return err
	}
	if err := os.Mkdir(c.getCacheDir(), 0755); err != nil {
		return err
	}
	if err := c.writeRepoFile(); err != nil {
		return err
	}
	return nil
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

// List will retrieve all installed releases from the cluster.
func (c *Client) List() ([]*release.Release, error) {
	list := action.NewList(c.actionConfig)
	return list.Run()
}

// AddRepository will append the RepositoryEntry to the temporary
// repositories-file. It will also block until the index-file was downloaded.
func (c *Client) AddRepository(cfg *RepositoryEntry) error {
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return errors.Errorf("invalid chart URL format: %s", cfg.URL)
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

// There is currently some helm functionality, where the environment variables
// can not be avoided. This function sets the necessary env-variables to get a
// valid cli.EnvSettings.
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

// ClientOption interface is implemented by all possible options to install charts.
type InstallOption interface {
	apply(*installOptions)
}

type installOptionAdapter func(*installOptions)

func (c installOptionAdapter) apply(o *installOptions) {
	c(o)
}

// InstallWithReleaseName will override the release name, which will be a
// random string by default.
func InstallWithReleaseName(name string) InstallOption {
	return installOptionAdapter(func(o *installOptions) {
		o.ReleaseName = name
	})
}

// Install will try to locate the chart and download the specified version.
// If the chartName is a local path, it will try to load the local chart instead.
// With the chart locally available it will install it using the provided
// values and options.
func (c *Client) Install(chartName, version string, valuesOptions ValuesOptions, opts ...InstallOption) (*Release, error) {
	options := installOptions{action.NewInstall(c.actionConfig)}
	options.ReleaseName = rand.String(5)
	options.Namespace = "default"
	options.Version = version
	for _, opt := range opts {
		opt.apply(&options)
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
	values, err := valuesOptions.MergeValues(getters)
	if err != nil {
		return nil, err
	}
	return options.Run(chart, values)
}

// Uninstall will remove the release from the cluster.
func (c *Client) Uninstall(releaseName string) error {
	uninstall := action.NewUninstall(c.actionConfig)
	_, err := uninstall.Run(releaseName)
	return err
}

// Close will release all filesystem resources of the helm client instance.
// Make sure to always call this function if the helm client is not required
// anymore.
func (c *Client) Close() error {
	if c.tempDir != nil {
		return c.tempDir.Close()
	}
	return nil
}
