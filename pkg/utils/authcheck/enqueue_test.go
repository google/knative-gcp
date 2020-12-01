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

package authcheck

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/controller"
	pkgtesting "knative.dev/pkg/reconciler/testing"

	serviceaccountinfomer "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/fake"

	"github.com/google/knative-gcp/pkg/logging"
	gcptesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	v1 "github.com/google/knative-gcp/pkg/reconciler/testing/v1"
)

const (
	pullsubscriptionName = "test-pullsubscription"
	topicName            = "test-topic"
	brokercellName       = "test-brokercell"
)

func TestEnqueuePullSubscriptions(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name           string
		objects        []runtime.Object
		serviceAccount *corev1.ServiceAccount
		wantKey        []string
	}{
		{
			name: "one pullsubcription in the same namespace as the k8s service account successfully enqueue.",
			objects: []runtime.Object{
				v1.NewPullSubscription(pullsubscriptionName, testNS, v1.WithPullSubscriptionServiceAccount(serviceAccountName)),
			},
			serviceAccount: gcptesting.NewServiceAccount(serviceAccountName, testNS),
			wantKey: []string{
				fmt.Sprintf("%s/%s", testNS, pullsubscriptionName),
			},
		},
		{
			name: "no pullsubcription in the same namespace as the k8s service account.",
			objects: []runtime.Object{
				v1.NewPullSubscription(pullsubscriptionName, testNS+"-diff", v1.WithPullSubscriptionServiceAccount(serviceAccountName)),
			},
			serviceAccount: gcptesting.NewServiceAccount(serviceAccountName, testNS),
			wantKey:        []string{},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			//  Sets up the the Context and the fake informers for the tests.
			ctx, cancel, _ := pkgtesting.SetupFakeContextWithCancel(t)
			defer cancel()
			// Get serviceaccount Informer from fake ctx.
			serviceaccountInformer := serviceaccountinfomer.Get(ctx)
			// Get pullsubscription lister from tc.objects.
			lister := gcptesting.NewListers(tc.objects)
			pullsubscriptionLister := lister.GetPullSubscriptionLister()
			// Create a fake controller implementation.
			logger := logging.FromContext(ctx).Sugar()
			r := newFakeReconciler()
			impl := controller.NewImplFull(r, controller.ControllerOptions{WorkQueueName: "test-pullsubscription-queue", Logger: logger})

			// Configure event handler for serviceaccount informer.
			serviceaccountInformer.Informer().AddEventHandler(EnqueuePullSubscription(impl, pullsubscriptionLister))

			// Put object into store to simulate the process of
			// reflector syncing a newly changed object into the local store.
			serviceaccountInformer.Informer().GetStore().Add(tc.serviceAccount)

			go func() {
				// Run the service account informer, it will stop when ctx is cancelled.
				serviceaccountInformer.Informer().Run(stopCh(ctx))
			}()

			time.Sleep(1 * time.Second)

			// Get keys enqueued to the workqueue from service account informer event handler.
			gotKey := []string{}
			for impl.WorkQueue().Len() != 0 {
				obj, quit := impl.WorkQueue().Get()
				if quit {
					continue
				}
				key := obj.(types.NamespacedName)
				gotKey = append(gotKey, key.String())
			}

			if diff := cmp.Diff(tc.wantKey, gotKey); diff != "" {
				t.Error("unexpected authType (-want, +got) = ", diff)
			}
		})
	}
}

func TestEnqueueTopics(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name           string
		objects        []runtime.Object
		serviceAccount *corev1.ServiceAccount
		wantKey        []string
	}{
		{
			name: "one topic in the same namespace as the k8s service account successfully enqueue.",
			objects: []runtime.Object{
				v1.NewTopic(topicName, testNS, v1.WithTopicServiceAccountName(serviceAccountName)),
			},
			serviceAccount: gcptesting.NewServiceAccount(serviceAccountName, testNS),
			wantKey: []string{
				fmt.Sprintf("%s/%s", testNS, topicName),
			},
		},
		{
			name: "no topic in the same namespace as the k8s service account.",
			objects: []runtime.Object{
				v1.NewTopic(topicName, testNS+"-diff", v1.WithTopicServiceAccountName(serviceAccountName)),
			},
			serviceAccount: gcptesting.NewServiceAccount(serviceAccountName, testNS),
			wantKey:        []string{},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			//  Sets up the the Context and the fake informers for the tests.
			ctx, cancel, _ := pkgtesting.SetupFakeContextWithCancel(t)
			defer cancel()
			// Get serviceaccount Informer from fake ctx.
			serviceaccountInformer := serviceaccountinfomer.Get(ctx)
			// Get topic lister from tc.objects.
			lister := gcptesting.NewListers(tc.objects)
			topicLister := lister.GetTopicLister()
			// Create a fake controller implementation.
			logger := logging.FromContext(ctx).Sugar()
			r := newFakeReconciler()
			impl := controller.NewImplFull(r, controller.ControllerOptions{WorkQueueName: "test-topic-queue", Logger: logger})

			// Configure event handler for serviceaccount informer.
			serviceaccountInformer.Informer().AddEventHandler(EnqueueTopic(impl, topicLister))

			// Put object into store to simulate the process of
			// reflector syncing a newly changed object into the local Store.
			serviceaccountInformer.Informer().GetStore().Add(tc.serviceAccount)

			go func() {
				// Run the service account informer, it will stop when ctx is cancelled.
				serviceaccountInformer.Informer().Run(stopCh(ctx))
			}()

			time.Sleep(1 * time.Second)

			// Get keys enqueued to the workqueue from service account informer event handler.
			gotKey := []string{}
			for impl.WorkQueue().Len() != 0 {
				obj, quit := impl.WorkQueue().Get()
				if quit {
					continue
				}
				key := obj.(types.NamespacedName)
				gotKey = append(gotKey, key.String())
			}

			if diff := cmp.Diff(tc.wantKey, gotKey); diff != "" {
				t.Error("unexpected authType (-want, +got) = ", diff)
			}
		})
	}
}
func TestEnqueueBrokerCells(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name           string
		objects        []runtime.Object
		serviceAccount *corev1.ServiceAccount
		wantKey        []string
	}{
		{
			name: "one brokercell in the same namespace as the k8s service account successfully enqueue.",
			objects: []runtime.Object{
				gcptesting.NewBrokerCell(brokercellName, testNS),
			},
			serviceAccount: gcptesting.NewServiceAccount(serviceAccountName, testNS),
			wantKey: []string{
				fmt.Sprintf("%s/%s", testNS, brokercellName),
			},
		},
		{
			name: "no brokercells in the same namespace as the k8s service account.",
			objects: []runtime.Object{
				gcptesting.NewBrokerCell(brokercellName, testNS+"diff"),
			},
			serviceAccount: gcptesting.NewServiceAccount(serviceAccountName, testNS),
			wantKey:        []string{},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			//  Sets up the the Context and the fake informers for the tests.
			ctx, cancel, _ := pkgtesting.SetupFakeContextWithCancel(t)
			defer cancel()
			// Get serviceaccount Informer from fake ctx.
			serviceaccountInformer := serviceaccountinfomer.Get(ctx)
			// Get brokercell lister from tc.objects.
			lister := gcptesting.NewListers(tc.objects)
			brokerCellLister := lister.GetBrokerCellLister()
			// Create a fake controller implementation.
			logger := logging.FromContext(ctx).Sugar()
			r := newFakeReconciler()
			impl := controller.NewImplFull(r, controller.ControllerOptions{WorkQueueName: "test-brokercell-queue", Logger: logger})

			// Configure event handler for serviceaccount informer.
			serviceaccountInformer.Informer().AddEventHandler(EnqueueBrokerCell(impl, brokerCellLister))

			// Put object into store to simulate the process of
			// reflector syncing a newly changed object into the local Store.
			serviceaccountInformer.Informer().GetStore().Add(tc.serviceAccount)

			go func() {
				// Run the service account informer, it will stop when ctx is cancelled.
				serviceaccountInformer.Informer().Run(stopCh(ctx))
			}()

			time.Sleep(1 * time.Second)

			// Get keys enqueued to the workqueue from service account informer event handler.
			gotKey := []string{}
			for impl.WorkQueue().Len() != 0 {
				obj, quit := impl.WorkQueue().Get()
				if quit {
					continue
				}
				key := obj.(types.NamespacedName)
				gotKey = append(gotKey, key.String())
			}

			if diff := cmp.Diff(tc.wantKey, gotKey); diff != "" {
				t.Error("unexpected authType (-want, +got) = ", diff)
			}
		})
	}
}

// Fake reconciler for testing.
type fakeReconciler struct {
	Reconciler controller.Reconciler
}

func (fr *fakeReconciler) Reconcile(ctx context.Context, key string) error {
	return nil
}

func newFakeReconciler() controller.Reconciler {
	return fakeReconciler{}.Reconciler
}

// stopCh will receive termination signal when ctx is cancelled.
func stopCh(ctx context.Context) chan struct{} {
	ch := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}
