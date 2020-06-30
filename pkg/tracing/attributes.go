/*
Copyright 2020 Google LLC.

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

package tracing

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/tracing"
)

const (
	PubSubProtocol = "Pub/Sub"
)

var (
	PubSubProtocolAttribute = tracing.MessagingProtocolAttribute(PubSubProtocol)
)

func SubscriptionDestination(s string) string {
	return fmt.Sprintf("subscription:%s", s)
}

func SourceDestination(resourceGroup string, src types.NamespacedName) string {
	// resourceGroup is of the form <resource>.events.cloud.google.com,
	// where resource can be for example cloudpubsubsources.
	// We keep with the resource piece.
	gr := schema.ParseGroupResource(resourceGroup)
	return fmt.Sprintf("%s:%s.%s", gr.Resource, src.Name, src.Namespace)
}
