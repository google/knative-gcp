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

package controller

import (
	"context"
	"sync"
	"time"

	. "github.com/google/knative-gcp/pkg/broker/readiness"
	brokercellinformer "github.com/google/knative-gcp/pkg/client/injection/informers/intevents/v1alpha1/brokercell"
	inteventsv1alpha1lister "github.com/google/knative-gcp/pkg/client/listers/intevents/v1alpha1"
	"github.com/google/knative-gcp/pkg/logging"
	"github.com/google/knative-gcp/pkg/utils"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/workqueue"
	podinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/pod"
)

var (
	defaultBackoff = wait.Backoff{
		Steps:    5,
		Duration: 500 * time.Millisecond,
		Factor:   2.0,
		// The sleep at each iteration is the duration plus an additional
		// amount chosen uniformly at random from the interval between 0 and jitter*duration.
		Jitter: 1.0,
	}
)

type getGenClientFn func(ctx context.Context, r *GetGenerationReq, opts ...grpc.CallOption) (*GetGenerationResp, error)

type ConfigQueryManager struct {
	objectsQueue  workqueue.Interface
	clients       *ConfigCheckClientsMap
	records       recordsMap
	podLister     corev1listers.PodLister
	bcLister      inteventsv1alpha1lister.BrokerCellLister
	kubeClientSet kubernetes.Interface
}

func NewConfigQueryManager(clients *ConfigCheckClientsMap) *ConfigQueryManager {
	return &ConfigQueryManager{
		clients: clients,
	}
}

func (m *ConfigQueryManager) Setup(ctx context.Context, objQueue workqueue.Interface, kubeClientSet kubernetes.Interface) {
	setupPodInformerOnce(m, podinformer.Get(ctx))
	m.objectsQueue = objQueue
	m.kubeClientSet = kubeClientSet
	m.podLister = podinformer.Get(ctx).Lister()
	m.bcLister = brokercellinformer.Get(ctx).Lister()
}

func (m *ConfigQueryManager) collectPodsFromCache(ctx context.Context, object ReadinessCheckable) ([]*corev1.Pod, error) {
	bc, err := getBrokerCell(object.getBrokerCellName(), m.bcLister)
	if err != nil {
		return nil, err
	}

	pods := []*corev1.Pod{}
	for _, role := range object.getDataplanePodsRoles() {
		list, err := m.podLister.List(labels.SelectorFromSet(map[string]string{"brokerCell": bc.Name, "role": role}))
		if err != nil {
			return nil, err
		}
		pods = append(pods, list...)
	}
	return pods, nil
}

func (m *ConfigQueryManager) collectPodsFromServer(ctx context.Context, object ReadinessCheckable) ([]corev1.Pod, error) {
	bc, err := getBrokerCell(object.getBrokerCellName(), m.bcLister)
	if err != nil {
		return nil, err
	}

	pods := []corev1.Pod{}
	for _, role := range object.getDataplanePodsRoles() {
		ls := labels.SelectorFromSet(map[string]string{"brokerCell": bc.Name, "role": role})
		list, err := utils.GetPodList(ctx, ls, m.kubeClientSet, bc.Namespace)
		if err != nil {
			return nil, err
		}
		pods = append(pods, list.Items...)
	}
	return pods, nil
}

func (m *ConfigQueryManager) DataplanePodsReceiptConfirmed(ctx context.Context, object ReadinessCheckable) bool {
	logging := logging.FromContext(ctx)
	pods, err := m.collectPodsFromCache(ctx, object)
	if err != nil {
		logging.Error("can't get dataplane pods", zap.Any("object", object), zap.Error(err))
		return false
	}

	genExpect := object.getGeneration()
	if err != nil {
		logging.Error("can't get dataplane pods", zap.Any("object", object), zap.Error(err))
		return false
	}

	r, ok := m.records.getRecord(object.getKey())
	if ok {
		if allPodsAreReady(pods, r, genExpect) && m.noNewPodsComesUp(ctx, r, object) {
			m.records.deleteRecord(object.getKey())
			close(r.doneChan)
			return true
		}
	} else {
		r = m.records.addRecord(object.getKey())
		defer func() {
			go m.queryDataplanePodsForObject(ctx, object, r)
		}()
	}
	r.genChan <- genExpect
	return false
}

func (m *ConfigQueryManager) noNewPodsComesUp(ctx context.Context, r *record, object ReadinessCheckable) bool {
	pods, err := m.collectPodsFromServer(ctx, object)
	if err != nil {
		logging.FromContext(ctx).Error("fail to get dataplane from API server", zap.Error(err))
		return false
	}
	for _, pod := range pods {
		if _, ok := r.podGens[getPodKey(&pod)]; !ok {
			return false
		}
	}
	return true
}

func (m *ConfigQueryManager) queryDataplanePodsForObject(ctx context.Context, object ReadinessCheckable, r *record) {
	var retry *wait.Backoff
	var genExpect int64 = 0
	for {
		select {
		case <-r.doneChan:
			return
		case genMax := <-r.genChan:
			genMax = getMaximumGenerationFromChan(r.genChan, genMax)
			if genMax > genExpect {
				retry = &wait.Backoff{}
				*retry = defaultBackoff
				genExpect = genMax
			} else {
				time.Sleep(retry.Step())
			}

			pods, err := m.collectPodsFromCache(ctx, object)
			if err != nil {
				logging.FromContext(ctx).Error("can't get dataplane pods", zap.Any("cellTenant", object), zap.Error(err))
				r.genChan <- genExpect
				continue
			}
			var wg sync.WaitGroup
			for _, pod := range pods {
				podKey := getPodKey(pod)
				genGot, ok := r.podGens[podKey]
				if !ok {
					genGot = genExpect - 1
					r.podGens[podKey] = genGot
				}

				if genGot < genExpect {
					wg.Add(1)
					go m.querySingleDateplanePodForObject(ctx, object, r, pod, podKey)
				}
			}
			wg.Wait()
			if allPodsAreReady(pods, r, genExpect) && m.noNewPodsComesUp(ctx, r, object) {
				metaObj := object.(metav1.Object)
				m.objectsQueue.Add(types.NamespacedName{Namespace: metaObj.GetNamespace(), Name: metaObj.GetName()})
			} else {
				r.genChan <- genExpect
			}
		}
	}
}

func (m *ConfigQueryManager) querySingleDateplanePodForObject(ctx context.Context, object ReadinessCheckable, r *record, pod *corev1.Pod, podKey podKeyType) {
	client := m.clients.getClient(pod)
	clientFn := object.getGenFn(client)
	if resp, err := clientFn(ctx, &GetGenerationReq{Key: object.getKey()}); err != nil {
		r.podGens[podKey] = resp.Generation
	}
}
