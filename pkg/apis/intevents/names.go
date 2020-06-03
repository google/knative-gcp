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

package intevents

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"
)

// GenerateK8sName generates a K8s-complaint name based on obj properties.
//  Should be typically used to generate receive adapter names or other k8s resources we may want
//  to somehow link to its creator.
//  It uses the object labels to see whether it's from a source, channel, or ps to construct the name.
//  Format is: "cre-<ownertype>-<name>-<uid>" or a shortened hash if it exceeds the k8s limit.
//  For example: cre-src-mysource-8478931
func GenerateK8sName(obj v1.ObjectMetaAccessor) string {
	meta := obj.GetObjectMeta()
	labels := meta.GetLabels()
	name := meta.GetName()
	suffix := "-" + string(meta.GetUID())
	if _, ok := labels[SourceLabelKey]; ok {
		return kmeta.ChildName(fmt.Sprintf("cre-src-%s", name), suffix)
	} else if _, ok := labels[ChannelLabelKey]; ok {
		return kmeta.ChildName(fmt.Sprintf("cre-chan-%s", name), suffix)
	}
	return kmeta.ChildName(fmt.Sprintf("cre-ps-%s", name), suffix)
}

// GenerateName generates a name based on obj properties.
//  Should be typically used to generate Pub/Sub subscriptions.
//  It uses the object labels to see whether it's from a source, channel, or ps to construct the name.
//  Format is: "cre-<ownertype>_<namespace>_<name>_<uid>".
//  For example: cre-src_mynamespace_mysource_8478931
func GenerateName(obj v1.ObjectMetaAccessor) string {
	meta := obj.GetObjectMeta()
	labels := meta.GetLabels()
	name := meta.GetName()
	namespace := meta.GetNamespace()
	uid := meta.GetUID()
	if _, ok := labels[SourceLabelKey]; ok {
		return fmt.Sprintf("cre-src_%s_%s_%s", namespace, name, uid)
	} else if _, ok := labels[ChannelLabelKey]; ok {
		return fmt.Sprintf("cre-chan_%s_%s_%s", namespace, name, uid)
	}
	return fmt.Sprintf("cre-ps_%s_%s_%s", namespace, name, uid)
}
