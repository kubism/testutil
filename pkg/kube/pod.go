package kube

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
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
		localPort, err = getPort()
		if err != nil {
			return nil, err
		}
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
	hostIP := strings.TrimLeft(pf.restConfig.Host, "https://")

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

func getPort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func IsReady(pod *corev1.Pod) bool {
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

func WaitUntilReady(restConfig *rest.Config, pod *corev1.Pod, timeout time.Duration) error {
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
	for !IsReady(pod) {
		err := k8sClient.Get(ctx, objectKey, pod)
		if err != nil {
			return err
		}
	}
	return nil
}
