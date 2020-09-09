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

package resources

import (
	"fmt"

	"knative.dev/pkg/kmeta"

	intv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
)

const (
	// IngressName is the name used for the ingress container.
	IngressName = "ingress"
	// FanoutName is the name used for the fanout container.
	FanoutName = "fanout"
	// RetryName is the name used for the retry container.
	RetryName          = "retry"
	BrokerCellLabelKey = "brokerCell"
)

var (
	optionalSecretVolume = true
)

// Args are the common arguments to create a Broker's data plane Deployment.
type Args struct {
	ComponentName      string
	BrokerCell         *intv1alpha1.BrokerCell
	Image              string
	ServiceAccountName string
	MetricsPort        int
	AllowIstioSidecar  bool
	CPURequest         string
	CPULimit           string
	MemoryRequest      string
	MemoryLimit        string
}

// IngressArgs are the arguments to create a Broker's ingress Deployment.
type IngressArgs struct {
	Args
	Port int
}

// FanoutArgs are the arguments to create a Broker's fanout Deployment.
type FanoutArgs struct {
	Args
}

// RetryArgs are the arguments to create a Broker's retry Deployment.
type RetryArgs struct {
	Args
}

// AutoscalingArgs are the arguments to create HPA for deployments.
type AutoscalingArgs struct {
	ComponentName     string
	BrokerCell        *intv1alpha1.BrokerCell
	AvgCPUUtilization *int32
	AvgMemoryUsage    *string
	MaxReplicas       int32
	MinReplicas       int32
}

// Labels generates the labels present on all resources representing the
// component of the given BrokerCell.
func Labels(brokerCellName, componentName string) map[string]string {
	cl := CommonLabels(brokerCellName)
	cl["role"] = componentName
	return cl
}

func CommonLabels(brokerCellName string) map[string]string {
	return map[string]string{
		"app":              "cloud-run-events",
		BrokerCellLabelKey: brokerCellName,
	}
}

// Name creates a name for the component (ingress/fanout/retry).
func Name(brokerCellName, componentName string) string {
	return kmeta.ChildName(fmt.Sprintf("%s-brokercell-", brokerCellName), componentName)
}
