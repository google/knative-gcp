/*
Copyright 2020 Google LLC

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

package upgrader

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	duckapis "knative.dev/pkg/apis"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/logging"
)

// Upgrade deletes legacy resources {pullsubscription,topic}.pubsub.cloud.google.com in all namespaces.
// It's equivalent to:
// kubectl delete pullsubscriptions.pubsub.cloud.google.com --all-namespaces --all
// kubectl delete topics.pubsub.cloud.google.com --all-namespaces --all
func Upgrade(ctx context.Context) error {
	nsClient := kubeclient.Get(ctx).CoreV1().Namespaces()
	namespaces, err := nsClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}
	for _, ns := range namespaces.Items {
		if err := processNamespace(ctx, ns.Name); err != nil {
			return err
		}
	}
	return nil
}

func processNamespace(ctx context.Context, ns string) error {
	logger := logging.FromContext(ctx)
	logger.Infof("Processing namespace: %q", ns)

	tc, err := topicClient(ctx, ns)
	if err != nil {
		return err
	}

	psc, err := pullSubClient(ctx, ns)
	if err != nil {
		return err
	}

	logger.Debug("Deleting topics", zap.String("namespace", ns))
	if err := tc.DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil && !apierrs.IsNotFound(err) {
		return fmt.Errorf("failed to delete topics in namespace %q: %w", ns, err)
	}

	logger.Debug("Deleting pullsubscriptions", zap.String("namespace", ns))
	if err := psc.DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil && !apierrs.IsNotFound(err) {
		return fmt.Errorf("failed to delete pullsubscriptions in namespace %q: %w", ns, err)
	}

	return nil
}

func topicClient(ctx context.Context, namespace string) (dynamic.ResourceInterface, error) {
	return resourceInterface(dynamicclient.Get(ctx), namespace, schema.FromAPIVersionAndKind("pubsub.cloud.google.com/v1alpha1", "Topic"))
}

func pullSubClient(ctx context.Context, namespace string) (dynamic.ResourceInterface, error) {
	return resourceInterface(dynamicclient.Get(ctx), namespace, schema.FromAPIVersionAndKind("pubsub.cloud.google.com/v1alpha1", "PullSubscription"))
}

func resourceInterface(dynamicClient dynamic.Interface, namespace string, gvk schema.GroupVersionKind) (dynamic.ResourceInterface, error) {
	rc := dynamicClient.Resource(duckapis.KindToResource(gvk))

	if rc == nil {
		return nil, fmt.Errorf("failed to create dynamic client for gvk: %s", gvk)
	}
	return rc.Namespace(namespace), nil
}
