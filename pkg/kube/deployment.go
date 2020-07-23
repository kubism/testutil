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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func MustGetDeployment(restConfig *rest.Config, namespace, name string) *appsv1.Deployment {
	deployment := &appsv1.Deployment{}
	k8sClient, err := client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		panic(err)
	}
	err = k8sClient.Get(context.Background(), client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, deployment)
	if err != nil {
		panic(err)
	}
	return deployment
}

func IsDeploymentScheduled(deployment *appsv1.Deployment) bool {
	replicas := int32(1)
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}
	return deployment.Status.Replicas >= replicas
}

func IsDeploymentReady(deployment *appsv1.Deployment) bool {
	replicas := int32(1)
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}
	return deployment.Status.ReadyReplicas == replicas &&
		deployment.Status.UpdatedReplicas == replicas
}

func WaitUntilDeploymentScheduled(restConfig *rest.Config, deployment *appsv1.Deployment, timeout time.Duration) error {
	k8sClient, err := client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err != nil {
		return err
	}
	objectKey, err := client.ObjectKeyFromObject(deployment)
	if err != nil {
		return err
	}
	for !IsDeploymentScheduled(deployment) {
		err := k8sClient.Get(ctx, objectKey, deployment)
		if err != nil {
			return err
		}
	}
	return nil
}

func WaitUntilDeploymentReady(restConfig *rest.Config, deployment *appsv1.Deployment, timeout time.Duration) error {
	k8sClient, err := client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err != nil {
		return err
	}
	objectKey, err := client.ObjectKeyFromObject(deployment)
	if err != nil {
		return err
	}
	for !IsDeploymentReady(deployment) {
		err := k8sClient.Get(ctx, objectKey, deployment)
		if err != nil {
			return err
		}
	}
	return nil
}
