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

// Code generated by informer-gen. DO NOT EDIT.

package externalversions

import (
	"fmt"

	v1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	v1 "github.com/google/knative-gcp/pkg/apis/events/v1"
	v1alpha1 "github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	eventsv1beta1 "github.com/google/knative-gcp/pkg/apis/events/v1beta1"
	inteventsv1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	inteventsv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	inteventsv1beta1 "github.com/google/knative-gcp/pkg/apis/intevents/v1beta1"
	messagingv1alpha1 "github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
	messagingv1beta1 "github.com/google/knative-gcp/pkg/apis/messaging/v1beta1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	cache "k8s.io/client-go/tools/cache"
)

// GenericInformer is type of SharedIndexInformer which will locate and delegate to other
// sharedInformers based on type
type GenericInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() cache.GenericLister
}

type genericInformer struct {
	informer cache.SharedIndexInformer
	resource schema.GroupResource
}

// Informer returns the SharedIndexInformer.
func (f *genericInformer) Informer() cache.SharedIndexInformer {
	return f.informer
}

// Lister returns the GenericLister.
func (f *genericInformer) Lister() cache.GenericLister {
	return cache.NewGenericLister(f.Informer().GetIndexer(), f.resource)
}

// ForResource gives generic access to a shared informer of the matching type
// TODO extend this to unknown resources with a client pool
func (f *sharedInformerFactory) ForResource(resource schema.GroupVersionResource) (GenericInformer, error) {
	switch resource {
	// Group=eventing.knative.dev, Version=v1beta1
	case v1beta1.SchemeGroupVersion.WithResource("brokers"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Eventing().V1beta1().Brokers().Informer()}, nil
	case v1beta1.SchemeGroupVersion.WithResource("triggers"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Eventing().V1beta1().Triggers().Informer()}, nil

		// Group=events.cloud.google.com, Version=v1
	case v1.SchemeGroupVersion.WithResource("cloudauditlogssources"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Events().V1().CloudAuditLogsSources().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("cloudpubsubsources"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Events().V1().CloudPubSubSources().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("cloudschedulersources"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Events().V1().CloudSchedulerSources().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("cloudstoragesources"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Events().V1().CloudStorageSources().Informer()}, nil

		// Group=events.cloud.google.com, Version=v1alpha1
	case v1alpha1.SchemeGroupVersion.WithResource("cloudauditlogssources"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Events().V1alpha1().CloudAuditLogsSources().Informer()}, nil
	case v1alpha1.SchemeGroupVersion.WithResource("cloudbuildsources"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Events().V1alpha1().CloudBuildSources().Informer()}, nil
	case v1alpha1.SchemeGroupVersion.WithResource("cloudpubsubsources"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Events().V1alpha1().CloudPubSubSources().Informer()}, nil
	case v1alpha1.SchemeGroupVersion.WithResource("cloudschedulersources"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Events().V1alpha1().CloudSchedulerSources().Informer()}, nil
	case v1alpha1.SchemeGroupVersion.WithResource("cloudstoragesources"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Events().V1alpha1().CloudStorageSources().Informer()}, nil

		// Group=events.cloud.google.com, Version=v1beta1
	case eventsv1beta1.SchemeGroupVersion.WithResource("cloudauditlogssources"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Events().V1beta1().CloudAuditLogsSources().Informer()}, nil
	case eventsv1beta1.SchemeGroupVersion.WithResource("cloudbuildsources"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Events().V1beta1().CloudBuildSources().Informer()}, nil
	case eventsv1beta1.SchemeGroupVersion.WithResource("cloudpubsubsources"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Events().V1beta1().CloudPubSubSources().Informer()}, nil
	case eventsv1beta1.SchemeGroupVersion.WithResource("cloudschedulersources"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Events().V1beta1().CloudSchedulerSources().Informer()}, nil
	case eventsv1beta1.SchemeGroupVersion.WithResource("cloudstoragesources"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Events().V1beta1().CloudStorageSources().Informer()}, nil

		// Group=internal.events.cloud.google.com, Version=v1
	case inteventsv1.SchemeGroupVersion.WithResource("pullsubscriptions"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Internal().V1().PullSubscriptions().Informer()}, nil
	case inteventsv1.SchemeGroupVersion.WithResource("topics"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Internal().V1().Topics().Informer()}, nil

		// Group=internal.events.cloud.google.com, Version=v1alpha1
	case inteventsv1alpha1.SchemeGroupVersion.WithResource("brokercells"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Internal().V1alpha1().BrokerCells().Informer()}, nil
	case inteventsv1alpha1.SchemeGroupVersion.WithResource("pullsubscriptions"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Internal().V1alpha1().PullSubscriptions().Informer()}, nil
	case inteventsv1alpha1.SchemeGroupVersion.WithResource("topics"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Internal().V1alpha1().Topics().Informer()}, nil

		// Group=internal.events.cloud.google.com, Version=v1beta1
	case inteventsv1beta1.SchemeGroupVersion.WithResource("pullsubscriptions"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Internal().V1beta1().PullSubscriptions().Informer()}, nil
	case inteventsv1beta1.SchemeGroupVersion.WithResource("topics"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Internal().V1beta1().Topics().Informer()}, nil

		// Group=messaging.cloud.google.com, Version=v1alpha1
	case messagingv1alpha1.SchemeGroupVersion.WithResource("channels"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Messaging().V1alpha1().Channels().Informer()}, nil

		// Group=messaging.cloud.google.com, Version=v1beta1
	case messagingv1beta1.SchemeGroupVersion.WithResource("channels"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Messaging().V1beta1().Channels().Informer()}, nil

	}

	return nil, fmt.Errorf("no informer found for %v", resource)
}
