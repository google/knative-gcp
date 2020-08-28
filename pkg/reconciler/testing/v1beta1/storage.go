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

package v1beta1

import (
	reconcilertesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/knative-gcp/pkg/apis/duck"
	"github.com/google/knative-gcp/pkg/apis/events/v1beta1"
	"github.com/google/knative-gcp/pkg/gclient/metadata/testing"
)

// CloudStorageSourceOption enables further configuration of a CloudStorageSource.
type CloudStorageSourceOption func(*v1beta1.CloudStorageSource)

// NewCloudStorageSource creates a CloudStorageSource with CloudStorageSourceOptions
func NewCloudStorageSource(name, namespace string, so ...CloudStorageSourceOption) *v1beta1.CloudStorageSource {
	s := &v1beta1.CloudStorageSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-storage-uid",
			Annotations: map[string]string{
				duck.ClusterNameAnnotation: testing.FakeClusterName,
			},
		},
	}
	for _, opt := range so {
		opt(s)
	}
	return s
}

func WithCloudStorageSourceBucket(bucket string) CloudStorageSourceOption {
	return func(s *v1beta1.CloudStorageSource) {
		s.Spec.Bucket = bucket
	}
}

func WithCloudStorageSourceSink(gvk metav1.GroupVersionKind, name string) CloudStorageSourceOption {
	return func(s *v1beta1.CloudStorageSource) {
		s.Spec.Sink = duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: reconcilertesting.ApiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithCloudStorageSourceServiceAccount(kServiceAccount string) CloudStorageSourceOption {
	return func(ps *v1beta1.CloudStorageSource) {
		ps.Spec.ServiceAccountName = kServiceAccount
	}
}
