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

package broker

import (
	"context"

	"github.com/google/knative-gcp/pkg/broker/ingress"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler/names"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/system"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	inteventsv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler/broker/resources"
	brokercellresources "github.com/google/knative-gcp/pkg/reconciler/brokercell/resources"
)

// ensureBrokerCellExists creates a BrokerCell if it doesn't exist, and update broker status based on brokercell status.
func (r *Reconciler) ensureBrokerCellExists(ctx context.Context, b *brokerv1beta1.Broker) error {
	var bc *inteventsv1alpha1.BrokerCell
	var err error
	// TODO(#866) Get brokercell based on the label (or annotation) on the broker.
	bc, err = r.brokerCellLister.BrokerCells(system.Namespace()).Get(resources.DefaultBrokerCellName)

	if err != nil && !apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Error("Error reconciling brokercell", zap.String("namespace", b.Namespace), zap.String("broker", b.Name), zap.Error(err))
		b.Status.MarkBrokerCellUnknown("BrokerCellUnknown", "Failed to get brokercell %s/%s", bc.Namespace, bc.Name)
		return err
	}

	if apierrs.IsNotFound(err) {
		want := resources.CreateBrokerCell(b)
		bc, err = r.RunClientSet.InternalV1alpha1().BrokerCells(want.Namespace).Create(want)
		if err != nil && !apierrs.IsAlreadyExists(err) {
			logging.FromContext(ctx).Error("Error creating brokercell", zap.String("namespace", b.Namespace), zap.String("broker", b.Name), zap.Error(err))
			b.Status.MarkBrokerCellFailed("BrokerCellCreationFailed", "Failed to create %s/%s", want.Namespace, want.Name)
			return err
		}
		if apierrs.IsAlreadyExists(err) {
			logging.FromContext(ctx).Info("Brokercell already exists", zap.String("namespace", b.Namespace), zap.String("broker", b.Name))
			// There can be a race condition where the informer is not updated. In this case we directly
			// read from the API server.
			bc, err = r.RunClientSet.InternalV1alpha1().BrokerCells(want.Namespace).Get(want.Name, metav1.GetOptions{})
			if err != nil {
				logging.FromContext(ctx).Error("Failed to get the brokercell from the API server", zap.String("namespace", b.Namespace), zap.String("broker", b.Name), zap.Error(err))
				b.Status.MarkBrokerCellUnknown("BrokerCellUnknown", "Failed to get the brokercell from the API server %s/%s", want.Namespace, want.Name)
				return err
			}
		}
		if err == nil {
			r.Recorder.Eventf(b, corev1.EventTypeNormal, brokerCellCreated, "Created brokercell %s/%s", bc.Namespace, bc.Name)
		}
	}

	if bc.Status.IsReady() {
		b.Status.MarkBrokerCellReady()
	} else {
		b.Status.MarkBrokerCellUnknown("BrokerCellNotReady", "Brokercell %s/%s is not ready", bc.Namespace, bc.Name)
	}

	//TODO(#1019) Use the IngressTemplate of brokercell.
	ingressServiceName := brokercellresources.Name(bc.Name, brokercellresources.IngressName)
	b.Status.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   names.ServiceHostName(ingressServiceName, bc.Namespace),
		Path:   ingress.BrokerPath(b.Namespace, b.Name),
	})

	return nil
}
