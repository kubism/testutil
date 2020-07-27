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

package kube

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kubism/testutil/pkg/misc"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/reference"
	"k8s.io/client-go/transport/spdy"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const PortAny = 0

type clientOptions struct {
	Scheme *runtime.Scheme
}

// ClientOption interface is implemented by all possible options to instantiate
// a new kubernetes client.
type ClientOption interface {
	apply(*clientOptions)
}

type clientOptionAdapter func(*clientOptions)

func (c clientOptionAdapter) apply(o *clientOptions) {
	c(o)
}

// ClientWithScheme override the default scheme and provide your own.
func ClientWithScheme(scheme *runtime.Scheme) ClientOption {
	return clientOptionAdapter(func(o *clientOptions) {
		o.Scheme = scheme
	})
}

// Client is an extension to the controller-runtime Client, which provides
// additional capabilities including port-forward and more.
type Client struct {
	client.Client
	restConfig *rest.Config
	scheme     *runtime.Scheme
}

func NewClient(restConfig *rest.Config, opts ...ClientOption) (*Client, error) {
	options := clientOptions{ // Default options
		Scheme: scheme.Scheme,
	}
	for _, opt := range opts {
		opt.apply(&options)
	}
	k8sClient, err := client.New(restConfig, client.Options{
		Scheme: options.Scheme,
	})
	if err != nil {
		return nil, err
	}
	return &Client{
		Client:     k8sClient,
		restConfig: restConfig,
		scheme:     options.Scheme,
	}, nil
}

type PortForward struct {
	LocalPort  int
	restConfig *rest.Config
	streams    genericclioptions.IOStreams
	stopCh     chan struct{}
	in         *bytes.Buffer
	out        *bytes.Buffer
	errout     *bytes.Buffer
}

func (c *Client) PortForward(pod *corev1.Pod, localPort, podPort int) (*PortForward, error) {
	var err error
	if localPort == PortAny {
		localPort = misc.GetFreePort()
	}
	pf := &PortForward{
		LocalPort:  localPort,
		restConfig: c.restConfig,
		stopCh:     make(chan struct{}, 1),
	}
	readyCh := make(chan struct{})
	errorCh := make(chan error, 1)
	pf.streams, pf.in, pf.out, pf.errout = genericclioptions.NewTestIOStreams()
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", pod.Namespace, pod.Name)
	hostIP := strings.TrimLeft(pf.restConfig.Host, "htps:/")

	transport, upgrader, err := spdy.RoundTripperFor(pf.restConfig)
	if err != nil {
		return nil, err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport},
		http.MethodPost, &url.URL{Scheme: "https", Path: path, Host: hostIP})
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", localPort, podPort)},
		pf.stopCh, readyCh, pf.streams.Out, pf.streams.ErrOut)
	if err != nil {
		return nil, err
	}
	go func() {
		err := fw.ForwardPorts()
		errorCh <- err
	}()
	select {
	case <-readyCh:
		return pf, nil
	case err := <-errorCh:
		return nil, err
	case <-time.After(30 * time.Second):
		pf.Close()
		return nil, fmt.Errorf("port-forward did not become ready in time")
	}
}

func (pf *PortForward) Close() error {
	close(pf.stopCh)
	return nil
}

func (c *Client) MustGetPod(ctx context.Context, namespace, name string) *corev1.Pod {
	pod := &corev1.Pod{}
	err := c.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, pod)
	if err != nil {
		panic(err)
	}
	return pod
}

func IsPodReady(pod *corev1.Pod) bool {
	if pod.Status.ContainerStatuses == nil {
		return false
	}
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if !containerStatus.Ready {
			return false
		}
	}
	return true
}

func (c *Client) WaitUntilPodReady(ctx context.Context, pod *corev1.Pod) error {
	return c.waitUntil(ctx, pod, func() bool {
		return IsPodReady(pod)
	})
}

func (c *Client) GetPodLogs(ctx context.Context, pod *corev1.Pod) (io.ReadCloser, error) {
	opts := corev1.PodLogOptions{}
	clientset, err := kubernetes.NewForConfig(c.restConfig)
	if err != nil {
		return nil, err
	}
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &opts)
	return req.Stream(ctx)
}

func (c *Client) GetPodLogsString(ctx context.Context, pod *corev1.Pod) (string, error) {
	readCloser, err := c.GetPodLogs(ctx, pod)
	if err != nil {
		return "", err
	}
	defer readCloser.Close()
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, readCloser)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (c *Client) MustGetDeployment(ctx context.Context, namespace, name string) *appsv1.Deployment {
	deployment := &appsv1.Deployment{}
	err := c.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, deployment)
	if err != nil {
		panic(err)
	}
	return deployment
}

func getDeploymentReplicas(deployment *appsv1.Deployment) int32 {
	if deployment.Spec.Replicas != nil {
		return *deployment.Spec.Replicas
	}
	return 1
}

func IsDeploymentScheduled(deployment *appsv1.Deployment) bool {
	replicas := getDeploymentReplicas(deployment)
	return deployment.Status.Replicas >= replicas
}

func IsDeploymentReady(deployment *appsv1.Deployment) bool {
	replicas := getDeploymentReplicas(deployment)
	return deployment.Status.ReadyReplicas == replicas
}

func IsDeploymentUpdated(deployment *appsv1.Deployment) bool {
	replicas := getDeploymentReplicas(deployment)
	return deployment.Status.UpdatedReplicas == replicas
}

func (c *Client) WaitUntilDeploymentScheduled(ctx context.Context, deployment *appsv1.Deployment) error {
	return c.waitUntil(ctx, deployment, func() bool {
		return IsDeploymentScheduled(deployment)
	})
}

func (c *Client) WaitUntilDeploymentReady(ctx context.Context, deployment *appsv1.Deployment) error {
	return c.waitUntil(ctx, deployment, func() bool {
		return IsDeploymentReady(deployment)
	})
}

func (c *Client) WaitUntilDeploymentUpdated(ctx context.Context, deployment *appsv1.Deployment) error {
	return c.waitUntil(ctx, deployment, func() bool {
		return IsDeploymentUpdated(deployment)
	})
}

func (c *Client) waitUntil(ctx context.Context, obj runtime.Object, check func() bool) error {
	objectKey, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return err
	}
	for !check() {
		err := c.Get(ctx, objectKey, obj)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) filterEvents(in []corev1.Event, obj runtime.Object) ([]corev1.Event, error) {
	out := []corev1.Event{}
	ref, err := reference.GetReference(c.scheme, obj)
	if err != nil {
		return nil, err
	}
	ogvk := ref.GroupVersionKind()
	for _, e := range in {
		egvk := e.InvolvedObject.GroupVersionKind()
		if egvk.Kind != ogvk.Kind || egvk.Group != ogvk.Group || egvk.Version != ogvk.Version {
			continue // well different object, so skip it
		}
		if e.InvolvedObject.Name != ref.Name {
			continue // not the name we are looking for
		}
		out = append(out, e) // we found a related event
	}
	return out, nil
}

func (c *Client) GetEvents(ctx context.Context, obj runtime.Object) ([]corev1.Event, error) {
	clientset, err := kubernetes.NewForConfig(c.restConfig)
	if err != nil {
		return nil, err
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	listOptions := metav1.ListOptions{}
	list, err := clientset.CoreV1().Events(accessor.GetNamespace()).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}
	return c.filterEvents(list.Items, obj)
}
