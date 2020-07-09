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

package upgrader

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
)

type obj struct {
	apiVersion string
	kind       string
	namespace  string
	name       string
}

type deletion struct {
	GroupVersionResource schema.GroupVersionResource
	Namespace            string
}

func TestUpgrade(t *testing.T) {
	ns1, ns2 := "ns1", "ns2"
	cases := []struct {
		name                    string
		objs                    []obj
		listNamespaceErr        bool
		deleteErrLegacyResource string
		wantDeletes             []deletion
		wantErr                 bool
	}{{
		name: "success",
		objs: []obj{
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "Topic", namespace: ns1, name: "t1"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "Topic", namespace: ns1, name: "t2"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "Topic", namespace: ns2, name: "t3"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "PullSubscription", namespace: ns1, name: "sub1"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "PullSubscription", namespace: ns2, name: "sub2"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "PullSubscription", namespace: ns2, name: "sub3"},
		},
		wantDeletes: []deletion{
			{GroupVersionResource: schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "topics"}, Namespace: ns1},
			{GroupVersionResource: schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "pullsubscriptions"}, Namespace: ns1},
			{GroupVersionResource: schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "topics"}, Namespace: ns2},
			{GroupVersionResource: schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "pullsubscriptions"}, Namespace: ns2},
		},
	}, {
		name: "list namespace failure",
		objs: []obj{
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "Topic", namespace: ns1, name: "t1"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "Topic", namespace: ns1, name: "t2"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "Topic", namespace: ns2, name: "t3"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "PullSubscription", namespace: ns1, name: "sub1"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "PullSubscription", namespace: ns2, name: "sub2"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "PullSubscription", namespace: ns2, name: "sub3"},
		},
		listNamespaceErr: true,
		wantDeletes:      make([]deletion, 0),
		wantErr:          true,
	}, {
		name: "delete topics failure",
		objs: []obj{
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "Topic", namespace: ns1, name: "t1"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "Topic", namespace: ns1, name: "t2"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "Topic", namespace: ns2, name: "t3"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "PullSubscription", namespace: ns1, name: "sub1"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "PullSubscription", namespace: ns2, name: "sub2"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "PullSubscription", namespace: ns2, name: "sub3"},
		},
		deleteErrLegacyResource: "topics",
		wantDeletes: []deletion{
			{GroupVersionResource: schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "topics"}, Namespace: ns1},
		},
		wantErr: true,
	}, {
		name: "delete pullsubscriptions failure",
		objs: []obj{
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "Topic", namespace: ns1, name: "t1"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "Topic", namespace: ns1, name: "t2"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "Topic", namespace: ns2, name: "t3"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "PullSubscription", namespace: ns1, name: "sub1"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "PullSubscription", namespace: ns2, name: "sub2"},
			{apiVersion: "pubsub.cloud.google.com/v1alpha1", kind: "PullSubscription", namespace: ns2, name: "sub3"},
		},
		deleteErrLegacyResource: "pullsubscriptions",
		wantDeletes: []deletion{
			{GroupVersionResource: schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "topics"}, Namespace: ns1},
			{GroupVersionResource: schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "pullsubscriptions"}, Namespace: ns1},
		},
		wantErr: true,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			legacyResources := make([]runtime.Object, 0)
			for _, o := range tc.objs {
				legacyResources = append(legacyResources, newUnstructured(o))
			}

			ctx, dc := fakedynamicclient.With(context.Background(), runtime.NewScheme(), legacyResources...)
			kc := fake.NewSimpleClientset(
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns1}},
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns2}},
			)
			ctx = context.WithValue(ctx, kubeclient.Key{}, kc)

			gotDeletes := make([]deletion, 0)
			dc.PrependReactor("delete-collection", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
				a := action.(clientgotesting.DeleteCollectionAction)
				gotDelete := deletion{
					GroupVersionResource: a.GetResource(),
					Namespace:            a.GetNamespace(),
				}
				gotDeletes = append(gotDeletes, gotDelete)
				if a.GetResource().Resource == tc.deleteErrLegacyResource {
					return true, nil, fmt.Errorf("inducing failure for %s %s", action.GetVerb(), action.GetResource().Resource)
				}
				return true, nil, nil
			})
			if tc.listNamespaceErr {
				kc.PrependReactor("list", "namespaces", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("inducing failure for %s %s", action.GetVerb(), action.GetResource().Resource)
				})
			}

			err := Upgrade(ctx)
			if tc.wantErr != (err != nil) {
				t.Errorf("Upgrade want error got=%v, want=%v", (err != nil), tc.wantErr)
			}
			if diff := cmp.Diff(tc.wantDeletes, gotDeletes); diff != "" {
				t.Errorf("Resources deleted (-want,+got): %v", diff)
			}
		})
	}
}

func newUnstructured(o obj) *unstructured.Unstructured {
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": o.apiVersion,
			"kind":       o.kind,
			"metadata": map[string]interface{}{
				"namespace": o.namespace,
				"name":      o.name,
			},
			"spec":   map[string]interface{}{},
			"status": map[string]interface{}{},
		},
	}
	return u
}
