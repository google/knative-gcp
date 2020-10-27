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

package lib

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/knative-gcp/test/lib/resources"
)

var JobTypeMeta = batchTypeMeta(resources.JobKind)

func batchTypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: resources.BatchAPIVersion,
	}
}

var KsvcTypeMeta = servingTypeMeta(resources.KServiceKind)

func servingTypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: resources.ServingAPIVersion,
	}
}

var ChannelTypeMeta = messagingTypeMeta(resources.ChannelKind)

func messagingTypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: resources.MessagingV1beta1APIVersion,
	}
}

var CloudStorageSourceV1TypeMeta = eventsV1TypeMeta(resources.CloudStorageSourceKind)

var CloudStorageSourceV1beta1TypeMeta = eventsV1beta1TypeMeta(resources.CloudStorageSourceKind)

var CloudStorageSourceV1alpha1TypeMeta = eventsV1alpha1TypeMeta(resources.CloudStorageSourceKind)

var CloudPubSubSourceV1TypeMeta = eventsV1TypeMeta(resources.CloudPubSubSourceKind)

var CloudPubSubSourceV1beta1TypeMeta = eventsV1beta1TypeMeta(resources.CloudPubSubSourceKind)

var CloudPubSubSourceV1alpha1TypeMeta = eventsV1alpha1TypeMeta(resources.CloudPubSubSourceKind)

var CloudBuildSourceV1TypeMeta = eventsV1TypeMeta(resources.CloudBuildSourceKind)

var CloudBuildSourceV1beta1TypeMeta = eventsV1beta1TypeMeta(resources.CloudBuildSourceKind)

var CloudBuildSourceV1alpha1TypeMeta = eventsV1alpha1TypeMeta(resources.CloudBuildSourceKind)

var CloudAuditLogsSourceV1TypeMeta = eventsV1TypeMeta(resources.CloudAuditLogsSourceKind)

var CloudAuditLogsSourceV1beta1TypeMeta = eventsV1beta1TypeMeta(resources.CloudAuditLogsSourceKind)

var CloudAuditLogsSourceV1alpha1TypeMeta = eventsV1alpha1TypeMeta(resources.CloudAuditLogsSourceKind)

var CloudSchedulerSourceV1TypeMeta = eventsV1TypeMeta(resources.CloudSchedulerSourceKind)

var CloudSchedulerSourceV1beta1TypeMeta = eventsV1beta1TypeMeta(resources.CloudSchedulerSourceKind)

var CloudSchedulerSourceV1alpha1TypeMeta = eventsV1alpha1TypeMeta(resources.CloudSchedulerSourceKind)

func eventsV1TypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: resources.EventsV1APIVersion,
	}
}

func eventsV1beta1TypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: resources.EventsV1beta1APIVersion,
	}
}

func eventsV1alpha1TypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: resources.EventsV1alpha1APIVersion,
	}
}

var PullSubscriptionV1TypeMeta = inteventsV1TypeMeta(resources.PullSubscriptionKind)

var PullSubscriptionV1beta1TypeMeta = inteventsV1beta1TypeMeta(resources.PullSubscriptionKind)

var PullSubscriptionV1alpha1TypeMeta = inteventsV1alpha1TypeMeta(resources.PullSubscriptionKind)

func inteventsV1TypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: resources.IntEventsV1APIVersion,
	}
}

func inteventsV1beta1TypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: resources.IntEventsV1beta1APIVersion,
	}
}

func inteventsV1alpha1TypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: resources.IntEventsV1alpha1APIVersion,
	}
}
