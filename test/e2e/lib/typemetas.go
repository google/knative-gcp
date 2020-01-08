/*
Copyright 2019 Google LLC

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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

var JobTypeMeta = batchTypeMeta("Job")

func batchTypeMeta(kind string) *metav1.TypeMeta {
    return &metav1.TypeMeta{
        Kind:       kind,
        APIVersion: "batch/v1",
    }
}

var KsvcTypeMeta = servingTypeMeta("Service")

func servingTypeMeta(kind string) *metav1.TypeMeta {
    return &metav1.TypeMeta{
        Kind:       kind,
        APIVersion: "serving.knative.dev/v1",
    }
}

var ChannelTypeMeta = gcloudMessagingTypeMeta("Channel")

func gcloudMessagingTypeMeta(kind string) *metav1.TypeMeta {
    return &metav1.TypeMeta{
        Kind:       kind,
        APIVersion: "messaging.cloud.google.com/v1alpha1",
    }
}

var StorageTypeMeta = gcloudEventsTypeMeta("Storage")

var PubsubTypeMeta = gcloudEventsTypeMeta("PubSub")

func gcloudEventsTypeMeta(kind string) *metav1.TypeMeta {
    return &metav1.TypeMeta{
        Kind:       kind,
        APIVersion: "events.cloud.google.com/v1alpha1",
    }
}

var PullSubscriptionTypeMeta = gcloudPubsubTypeMeta("PullSubscription")

func gcloudPubsubTypeMeta(kind string) *metav1.TypeMeta {
    return &metav1.TypeMeta{
        Kind:       kind,
        APIVersion: "pubsub.cloud.google.com/v1alpha1",
    }
}
