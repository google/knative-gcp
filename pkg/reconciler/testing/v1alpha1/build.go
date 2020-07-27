/*
Copyright 2019 Google LLC

Licensed under the Apache License, Veroute.on 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"github.com/google/knative-gcp/pkg/reconciler/testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

// CloudBuildSourceOption enables further configuration of a CloudBuildSource.
type CloudBuildSourceOption func(*v1alpha1.CloudBuildSource)

// NewCloudBuildSource creates a CloudBuildSource with CloudBuildSourceOptions
func NewCloudBuildSource(name, namespace string, so ...CloudBuildSourceOption) *v1alpha1.CloudBuildSource {
	bs := &v1alpha1.CloudBuildSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-build-uid",
		},
	}
	for _, opt := range so {
		opt(bs)
	}
	return bs
}

func WithCloudBuildSourceSink(gvk metav1.GroupVersionKind, name string) CloudBuildSourceOption {
	return func(bs *v1alpha1.CloudBuildSource) {
		bs.Spec.Sink = duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: testing.ApiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithCloudBuildSourceServiceAccount(kServiceAccount string) CloudBuildSourceOption {
	return func(bs *v1alpha1.CloudBuildSource) {
		bs.Spec.ServiceAccountName = kServiceAccount
	}
}
