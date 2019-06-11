/*
Copyright 2019 The Knative Authors

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

package testing

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/events/v1alpha1"
)

// PubSubSourceOption enables further configuration of a PubSubSource.
type PubSubSourceOption func(*v1alpha1.PubSubSource)

// NewPubSubSource creates a PubSubSource with PubSubSourceOptions
func NewPubSubSource(name, namespace string, so ...PubSubSourceOption) *v1alpha1.PubSubSource {
	s := &v1alpha1.PubSubSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range so {
		opt(s)
	}
	s.SetDefaults(context.Background())
	return s
}

// NewPubSubSourceWithoutNamespace creates a PubSubSource with PubSubSourceOptions but without a specific namespace
func NewPubSubSourceWithoutNamespace(name string, so ...PubSubSourceOption) *v1alpha1.PubSubSource {
	s := &v1alpha1.PubSubSource{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, opt := range so {
		opt(s)
	}
	s.SetDefaults(context.Background())
	return s
}

func WithPubSubSourceUID(uid types.UID) PubSubSourceOption {
	return func(s *v1alpha1.PubSubSource) {
		s.UID = uid
	}
}

func WithPubSubSourceGenerateName(generateName string) PubSubSourceOption {
	return func(c *v1alpha1.PubSubSource) {
		c.ObjectMeta.GenerateName = generateName
	}
}

// WithInitPubSubSourceConditions initializes the PubSubSources's conditions.
func WithInitPubSubSourceConditions(s *v1alpha1.PubSubSource) {
	s.Status.InitializeConditions()
}

func WithPubSubSourceSink(gvk metav1.GroupVersionKind, name string) PubSubSourceOption {
	return func(s *v1alpha1.PubSubSource) {
		s.Spec.Sink = &corev1.ObjectReference{
			APIVersion: apiVersion(gvk),
			Kind:       gvk.Kind,
			Name:       name,
		}
	}
}

func WithPubSubSourceSpec(spec v1alpha1.PubSubSourceSpec) PubSubSourceOption {
	return func(s *v1alpha1.PubSubSource) {
		s.Spec = spec
	}
}

func WithPubSubSourceReady(sink string) PubSubSourceOption {
	return func(s *v1alpha1.PubSubSource) {
		s.Status.InitializeConditions()
		s.Status.MarkSink(sink)
		s.Status.MarkDeployed()
		//s.Status.MarkSubscribed()
	}
}

//func WithPubSubSourceProjectResolved(projectID string) PubSubSourceOption {
//	return func(s *v1alpha1.PubSubSource) {
//		s.Status.ProjectID = projectID
//	}
//}

func WithPubSubSourceSinkNotFound() PubSubSourceOption {
	return func(s *v1alpha1.PubSubSource) {
		s.Status.MarkNoSink("NotFound", "")
	}
}

func WithPubSubSourceDeleted(s *v1alpha1.PubSubSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	s.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithPubSubSourceOwnerReferences(ownerReferences []metav1.OwnerReference) PubSubSourceOption {
	return func(c *v1alpha1.PubSubSource) {
		c.ObjectMeta.OwnerReferences = ownerReferences
	}
}

func WithPubSubSourceLabels(labels map[string]string) PubSubSourceOption {
	return func(c *v1alpha1.PubSubSource) {
		c.ObjectMeta.Labels = labels
	}
}

func WithPubSubSourceFinalizers(finalizers ...string) PubSubSourceOption {
	return func(s *v1alpha1.PubSubSource) {
		s.Finalizers = finalizers
	}
}
