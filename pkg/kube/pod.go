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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const PortAny = 0

type PortForward struct {
	LocalPort  int
	restConfig *rest.Config
	streams    genericclioptions.IOStreams
	stopCh     chan struct{}
	in         *bytes.Buffer
	out        *bytes.Buffer
	errout     *bytes.Buffer
}

func NewPortForward(restConfig *rest.Config, pod *corev1.Pod, localPort, podPort int) (*PortForward, error) {
	var err error
	if localPort == PortAny {
		localPort = misc.GetFreePort()
	}
	pf := &PortForward{
		LocalPort:  localPort,
		restConfig: restConfig,
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

func WaitUntilPodReady(restConfig *rest.Config, pod *corev1.Pod, timeout time.Duration) error {
	k8sClient, err := client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err != nil {
		return err
	}
	objectKey, err := client.ObjectKeyFromObject(pod)
	if err != nil {
		return err
	}
	for !IsPodReady(pod) {
		err := k8sClient.Get(ctx, objectKey, pod)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetPodLogs(restConfig *rest.Config, pod *corev1.Pod) (io.ReadCloser, error) {
	opts := corev1.PodLogOptions{}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &opts)
	return req.Stream(context.Background())
}

func GetPodLogsString(restConfig *rest.Config, pod *corev1.Pod) (string, error) {
	readCloser, err := GetPodLogs(restConfig, pod)
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
