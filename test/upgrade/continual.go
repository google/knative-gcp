/*
Copyright 2020 The Knative Authors
Modified work Copyright 2020 Google LLC

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

package upgrade

import (
	"time"

	brokerv1 "github.com/google/knative-gcp/pkg/apis/broker/v1"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/eventing/test/upgrade/prober"
	"knative.dev/eventing/test/upgrade/prober/sut"
	pkgupgrade "knative.dev/pkg/test/upgrade"
)

func ContinualTest() pkgupgrade.BackgroundOperation {
	configurator := func(config *prober.Config) error {
		config.FailOnErrors = true
		config.Interval = 10 * time.Millisecond
		config.FinishedSleep = 40 * time.Second
		config.ConfigTemplate = "../../../../../../test/upgrade/config.toml"
		bt := config.SystemUnderTest.(*sut.BrokerAndTriggers)
		bt.Opts = []resources.BrokerOption{
			resources.WithBrokerClassForBroker(brokerv1.BrokerClass),
		}
		return nil
	}
	opts := prober.ContinualVerificationOptions{
		Configurators: []prober.Configurator{configurator},
	}
	return prober.NewContinualVerification("EventingContinualTest", opts)
}
