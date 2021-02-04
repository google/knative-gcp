/*
Copyright 2021 Google LLC

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

package readiness

import (
	"fmt"
	"sync"

	inteventsv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	inteventsv1alpha1lister "github.com/google/knative-gcp/pkg/client/listers/intevents/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler/broker/resources"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
)

var (
	once sync.Once
)

func getBrokerCellName(obj runtime.Object) string {
	return resources.DefaultBrokerCellName
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func getBrokerCell(bcName string, bcLister inteventsv1alpha1lister.BrokerCellLister) (*inteventsv1alpha1.BrokerCell, error) {
	bcs, err := bcLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, bc := range bcs {
		if bc.Name == bcName {
			return bc, nil
		}
	}
	return nil, fmt.Errorf("Can't find brokercell %s", bcName)
}

func getMaximumGenerationFromChan(genChan <-chan int64, maxGen int64) int64 {
	for {
		select {
		case gen := <-genChan:
			maxGen = max(gen, maxGen)
		default:
			return maxGen
		}
	}
}

func allPodsAreReady(pods []*corev1.Pod, r *record, genExpect int64) bool {
	for _, pod := range pods {
		genGot, found := r.podGens[getPodKey(pod)]
		if !found || genGot < genExpect {
			return false
		}
	}
	return true
}

func setupPodInformerOnce(m *ConfigQueryManager, podinformer corev1informer.PodInformer) {
	once.Do(func() {
		handler := cache.ResourceEventHandlerFuncs{
			DeleteFunc: func(obj interface{}) {
				if pod, ok := obj.(*corev1.Pod); ok {
					m.clients.deleteClient(pod)
				}
			},
		}
		podinformer.Informer().AddEventHandler(handler)
	})
}
