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

// Package authcheck provides utilities to check authentication configuration for data plane resources.
// enqueue.go contains customized EventHandlers to enqueue resources for authentication check.
package authcheck

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"

	listers "github.com/google/knative-gcp/pkg/client/listers/intevents/v1"
	"github.com/google/knative-gcp/pkg/client/listers/intevents/v1alpha1"
)

// EnqueuePullSubscription returns an event handler for resources which are not created/owned by pullsubscription.
// It is used for serviceAccountInformer.
func EnqueuePullSubscription(impl *controller.Impl, pullSubscriptionLister listers.PullSubscriptionLister) cache.ResourceEventHandler {
	return controller.HandleAll(func(obj interface{}) {
		object, err := kmeta.DeletionHandlingAccessor(obj)
		if err != nil {
			return
		}
		// Get pullsubscriptions which are in the same namespace as the object.
		psList, err := pullSubscriptionLister.PullSubscriptions(object.GetNamespace()).List(labels.Everything())
		if err != nil {
			return
		}
		// Convert pullsubscriptions with qualified serviceAccountName into namespace/name strings, and pass them to EnqueueKey.
		for _, ps := range psList {
			if ps.Spec.ServiceAccountName == object.GetName() {
				impl.EnqueueKey(types.NamespacedName{Namespace: object.GetNamespace(), Name: ps.Name})
			}
		}
	})
}

// EnqueueTopic returns an event handler for resources which are not created/owned by topic.
// It is used for serviceAccountInformer.
func EnqueueTopic(impl *controller.Impl, topicLister listers.TopicLister) cache.ResourceEventHandler {
	return controller.HandleAll(func(obj interface{}) {
		object, err := kmeta.DeletionHandlingAccessor(obj)
		if err != nil {
			return
		}
		// Get topics which are in the same namespace as the object.
		topicList, err := topicLister.Topics(object.GetNamespace()).List(labels.Everything())
		if err != nil {
			return
		}
		// Convert topics with qualified serviceAccountName into namespace/name strings, and pass them to EnqueueKey.
		for _, t := range topicList {
			if t.Spec.ServiceAccountName == object.GetName() {
				impl.EnqueueKey(types.NamespacedName{Namespace: object.GetNamespace(), Name: t.Name})
			}
		}
	})
}

// EnqueueBrokerCell returns an event handler for resources which are not created/owned by brokercell.
// It is used for serviceAccountInformer and secretinformer.
func EnqueueBrokerCell(impl *controller.Impl, brokerCellLister v1alpha1.BrokerCellLister) cache.ResourceEventHandler {
	return controller.HandleAll(func(obj interface{}) {
		object, err := kmeta.DeletionHandlingAccessor(obj)
		if err != nil {
			return
		}
		// Get brokercells which are in the same namespace as the object.
		brokerCellList, err := brokerCellLister.BrokerCells(object.GetNamespace()).List(labels.Everything())
		if err != nil {
			return
		}
		// Convert brokercells into namespace/name strings, and pass them to EnqueueKey.
		for _, bc := range brokerCellList {
			impl.EnqueueKey(types.NamespacedName{Namespace: object.GetNamespace(), Name: bc.Name})
		}
	})
}
