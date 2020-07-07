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

// Package duck contains Cloud Run Events API versions for duck components
package duck

const (
	GroupName = "duck.cloud.google.com"

	// Autoscaling refers to the autoscaling group.
	Autoscaling = "autoscaling.knative.dev"

	// AutoscalingClassAnnotation is the annotation for the explicit class of
	// scaler that a particular resource has opted into.
	AutoscalingClassAnnotation = Autoscaling + "/class"
	// ClusterNameAnnotation is the annotation for the cluster Name.
	ClusterNameAnnotation = "cluster-name"

	// AutoscalingMinScaleAnnotation is the annotation to specify the minimum number of pods to scale to.
	AutoscalingMinScaleAnnotation = Autoscaling + "/minScale"
	// AutoscalingMaxScaleAnnotation is the annotation to specify the maximum number of pods to scale to.
	AutoscalingMaxScaleAnnotation = Autoscaling + "/maxScale"

	// KEDA is Keda autoscaler.
	KEDA = "keda.autoscaling.knative.dev"

	// KedaAutoscalingPollingIntervalAnnotation is the annotation that refers to the interval in seconds Keda
	// uses to poll metrics in order to inform its scaling decisions.
	KedaAutoscalingPollingIntervalAnnotation = KEDA + "/pollingInterval"
	// KedaAutoscalingCooldownPeriodAnnotation is the annotation that refers to the period Keda waits until it
	// scales a Deployment down.
	KedaAutoscalingCooldownPeriodAnnotation = KEDA + "/cooldownPeriod"
	// KedaAutoscalingSubscriptionSizeAnnotation is the annotation that refers to the size of unacked messages in a
	// Pub/Sub subscription that Keda uses in order to decide when and by how much to scale out.
	KedaAutoscalingSubscriptionSizeAnnotation = KEDA + "/subscriptionSize"

	// defaultMinScale is the default minimum set of Pods the scaler should
	// downscale the resource to.
	defaultMinScale = "0"
	// defaultMaxScale is the default maximum set of Pods the scaler should
	// upscale the resource to.
	defaultMaxScale = "1"

	// defaultKedaPollingInterval is the default polling interval in seconds Keda uses to poll metrics.
	defaultKedaPollingInterval = "15"
	// defaultKedaCooldownPeriod is the default cooldown period in seconds Keda uses to downscale resources.
	defaultKedaCooldownPeriod = "120"
	// defaultKedaSubscriptionSize is the default number of unacked messages in a Pub/Sub subscription that Keda
	// uses to scale out resources.
	defaultKedaSubscriptionSize = "100"

	// minimumMinScale is the minimum allowed value for the AutoscalingMinScaleAnnotation annotation.
	minimumMinScale = 0
	// minimumMaxScale is the minimum allowed value for the AutoscalingMaxScaleAnnotation annotation.
	minimumMaxScale = 1

	// minimumKedaPollingInterval is the minimum allowed value for the KedaAutoscalingPollingIntervalAnnotation annotation.
	minimumKedaPollingInterval = 5
	// minimumKedaCooldownPeriod is the minimum allowed value for the KedaAutoscalingCooldownPeriodAnnotation annotation.
	minimumKedaCooldownPeriod = 15
	// minimumKedaSubscriptionSize is the minimum allowed value for the KedaAutoscalingSubscriptionSizeAnnotation annotation.
	minimumKedaSubscriptionSize = 5
)
