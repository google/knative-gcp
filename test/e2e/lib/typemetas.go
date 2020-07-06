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

	"github.com/google/knative-gcp/test/e2e/lib/resources"
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
		APIVersion: resources.MessagingAPIVersion,
	}
}

var CloudStorageSourceTypeMeta = eventsTypeMeta(resources.CloudStorageSourceKind)

var CloudPubSubSourceTypeMeta = eventsTypeMeta(resources.CloudPubSubSourceKind)

var CloudBuildSourceTypeMeta = eventsTypeMeta(resources.CloudBuildSourceKind)

var CloudAuditLogsSourceTypeMeta = eventsTypeMeta(resources.CloudAuditLogsSourceKind)

var CloudSchedulerSourceTypeMeta = eventsTypeMeta(resources.CloudSchedulerSourceKind)

func eventsTypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: resources.EventsAPIVersion,
	}
}

var PullSubscriptionTypeMeta = inteventsTypeMeta(resources.PullSubscriptionKind)

func inteventsTypeMeta(kind string) *metav1.TypeMeta {
	return &metav1.TypeMeta{
		Kind:       kind,
		APIVersion: resources.IntEventsAPIVersion,
	}
}
