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

package resources

import (
	"k8s.io/apimachinery/pkg/labels"
)

func GetLabelSelector(controller, channel string) labels.Selector {
	return labels.SelectorFromSet(GetLabels(controller, channel))
}

func GetLabels(controller, channel string) map[string]string {
	return map[string]string{
		"cloud-run-events-channel":      controller,
		"cloud-run-events-channel-name": channel,
	}
}

func GetPullSubscriptionLabelSelector(controller, source, subscriber string) labels.Selector {
	return labels.SelectorFromSet(GetPullSubscriptionLabels(controller, source, subscriber))
}

func GetPullSubscriptionLabels(controller, channel, subscriber string) map[string]string {
	l := GetLabels(controller, channel)
	l["cloud-run-events-channel-subscriber"] = subscriber
	return l
}
