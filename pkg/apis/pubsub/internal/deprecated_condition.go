/*
 * Copyright 2020 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package internal

import (
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const deprecationMessage = "This object is deprecated and should be deleted. There is no public " +
	"replacement. If you want a public replacement, please comment on " +
	"https://github.com/google/knative-gcp/issues/905. This object should be deleted before " +
	"upgrading to 0.16. If it is not deleted before the 0.16 upgrade, then it will leak the " +
	"Topic/PullSubscription created in Google Cloud Platform."

// deprecatedCondition is the condition to add to types that will be removed in 0.16.
// See https://github.com/google/knative-gcp/issues/905 for more context.
var deprecatedCondition = apis.Condition{
	Type:               "Deprecated",
	Reason:             "willBeRemoved",
	Status:             corev1.ConditionTrue,
	Severity:           apis.ConditionSeverityWarning,
	Message:            deprecationMessage,
	LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now())},
}

// MarkDeprecated adds the DeprecatedCondition to the supplied conditions and returns the new
// conditions.
func MarkDeprecated(conditions duckv1.Conditions) duckv1.Conditions {
	dc := deprecatedCondition
	for i, c := range conditions {
		if c.Type == dc.Type {
			// If we'd only update the LastTransitionTime, then return.
			dc.LastTransitionTime = c.LastTransitionTime
			if !reflect.DeepEqual(&dc, &c) {
				dc.LastTransitionTime = apis.VolatileTime{Inner: metav1.NewTime(time.Now())}
				conditions[i] = dc
			}
			return conditions
		}
	}
	conditions = append(conditions, dc)
	return conditions
}
