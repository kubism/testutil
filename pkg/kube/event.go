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
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/reference"
)

func filterEvents(in []corev1.Event, scheme *runtime.Scheme, obj runtime.Object) ([]corev1.Event, error) {
	out := []corev1.Event{}
	ref, err := reference.GetReference(scheme, obj)
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

type getEventsOptions struct {
	scheme *runtime.Scheme
}

type GetEventsOption interface {
	apply(*getEventsOptions)
}

type getEventsOptionAdapter func(*getEventsOptions)

func (c getEventsOptionAdapter) apply(o *getEventsOptions) {
	c(o)
}

func GetEventsWithScheme(scheme *runtime.Scheme) GetEventsOption {
	return getEventsOptionAdapter(func(o *getEventsOptions) {
		o.scheme = scheme
	})
}

func GetEvents(restConfig *rest.Config, obj runtime.Object, opts ...GetEventsOption) ([]corev1.Event, error) {
	options := getEventsOptions{scheme.Scheme}
	for _, opt := range opts {
		opt.apply(&options)
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	listOptions := metav1.ListOptions{}
	list, err := clientset.CoreV1().Events(accessor.GetNamespace()).List(context.Background(), listOptions)
	if err != nil {
		return nil, err
	}
	return filterEvents(list.Items, options.scheme, obj)
}
