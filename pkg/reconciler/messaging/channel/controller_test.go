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

package channel

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	"github.com/google/knative-gcp/pkg/apis/messaging/v1beta1"
	reconcilertesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	"go.uber.org/zap"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/configmap"
	. "knative.dev/pkg/reconciler/testing"

	// Fake injection informers

	_ "github.com/google/knative-gcp/pkg/client/injection/informers/intevents/v1alpha1/brokercell/fake"
	_ "github.com/google/knative-gcp/pkg/client/injection/informers/intevents/v1beta1/pullsubscription/fake"
	_ "github.com/google/knative-gcp/pkg/client/injection/informers/intevents/v1beta1/topic/fake"
	_ "github.com/google/knative-gcp/pkg/client/injection/informers/messaging/v1beta1/channel/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/fake"
)

func TestNew(t *testing.T) {
	ctx, _ := SetupFakeContext(t)
	cmw := configmap.NewStaticWatcher()
	c := newController(ctx, cmw, reconcilertesting.NewDataresidencyTestStore(t, nil))

	if c == nil {
		t.Fatal("Expected newControllerWithIAMPolicyManager to return a non-nil value")
	}
}

func TestFilterChannelsForBrokerCell(t *testing.T) {
	testCases := map[string]struct {
		objectChanged interface{}
		channels      []runtime.Object
		wantEnqueued  []interface{}
	}{
		"not a BrokerCell": {
			objectChanged: 5,
			channels: []runtime.Object{
				channel("foo"),
			},
		},
		"all channels are enqueued": {
			objectChanged: &v1alpha1.BrokerCell{},
			channels: []runtime.Object{
				channel("foo"),
				channel("bar"),
			},
			wantEnqueued: []interface{}{
				channel("foo"),
				channel("bar"),
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			ls := reconcilertesting.NewListers(tc.channels)
			fi := &fakeImpl{}
			filter := filterChannelsForBrokerCell(zap.NewNop(), ls.GetChannelLister(), fi.enqueue)
			filter(tc.objectChanged)

			// In order to remove any chance of ordering differences causing flakiness, sort by
			// name.
			sortChannelByName(tc.wantEnqueued)
			sortChannelByName(fi.enqueued)

			if diff := cmp.Diff(tc.wantEnqueued, fi.enqueued); diff != "" {
				t.Errorf("Incorrect enqueued objects (-want +got) %s", diff)
			}
		})
	}
}

func channel(name string) *v1beta1.Channel {
	return &v1beta1.Channel{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
	}
}

type fakeImpl struct {
	enqueued []interface{}
}

func (f *fakeImpl) enqueue(obj interface{}) {
	f.enqueued = append(f.enqueued, obj)
}

func sortChannelByName(unordered []interface{}) {
	sort.Slice(unordered, func(i, j int) bool {
		iName := unordered[i].(*v1beta1.Channel).Name
		jName := unordered[j].(*v1beta1.Channel).Name
		return iName < jName
	})
}
