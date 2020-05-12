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

package resources

import (
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
)

func GetPullSubscriptionAnnotations(channel, clusterName string) map[string]string {
	annotation := map[string]string{
		"metrics-resource-group": "channels.messaging.cloud.google.com",
		"metrics-resource-name":  channel,
	}
	if clusterName != "" {
		annotation[duckv1alpha1.ClusterNameAnnotation] = clusterName
	}
	return annotation
}

func GetTopicAnnotations(clusterName string) map[string]string {
	annotation := map[string]string{}
	if clusterName != "" {
		annotation[duckv1alpha1.ClusterNameAnnotation] = clusterName
	}
	return annotation
}
