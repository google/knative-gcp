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

package testing

import (
	"context"
	"time"

	intv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BrokerCellOption enables further configuration of a BrokerCell.
type BrokerCellOption func(*intv1alpha1.BrokerCell)

// NewBrokerCell creates a BrokerCell with BrokerCellOptions.
func NewBrokerCell(name, namespace string, o ...BrokerCellOption) *intv1alpha1.BrokerCell {
	bc := &intv1alpha1.BrokerCell{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	for _, opt := range o {
		opt(bc)
	}
	bc.SetDefaults(context.Background())
	return bc
}

// WithInitBrokerCellConditions initializes the BrokerCell's conditions.
func WithInitBrokerCellConditions(bc *intv1alpha1.BrokerCell) {
	bc.Status.InitializeConditions()
}

func WithBrokerCellFinalizers(finalizers ...string) BrokerCellOption {
	return func(bc *intv1alpha1.BrokerCell) {
		bc.Finalizers = finalizers
	}
}

func WithBrokerCellGeneration(gen int64) BrokerCellOption {
	return func(bc *intv1alpha1.BrokerCell) {
		bc.Generation = gen
	}
}

func WithBrokerCellStatusObservedGeneration(gen int64) BrokerCellOption {
	return func(bc *intv1alpha1.BrokerCell) {
		bc.Status.ObservedGeneration = gen
	}
}

func WithBrokerCellDeletionTimestamp(bc *intv1alpha1.BrokerCell) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	bc.ObjectMeta.SetDeletionTimestamp(&t)
}
