/*
 * Copyright 2019 The Knative Authors
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

package v1alpha1

import (
	"github.com/google/knative-gcp/pkg/apis/pubsub/internal"
)

// MarkDeprecated adds a warning condition that this object is deprecated and will be dropped in a
// future release. Note that this does not affect the Ready condition.
func (s *PullSubscriptionStatus) MarkDeprecated() {
	dc := internal.DeprecatedCondition
	for i, c := range s.Conditions {
		if c.Type == dc.Type {
			s.Conditions[i] = dc
			return
		}
	}
	s.Conditions = append(s.Conditions, dc)
}

// MarkDeprecated adds a warning condition that this object is deprecated and will be dropped in a
// future release. Note that this does not affect the Ready condition.
func (ts *TopicStatus) MarkDeprecated() {
	dc := internal.DeprecatedCondition
	for i, c := range ts.Conditions {
		if c.Type == dc.Type {
			ts.Conditions[i] = dc
			return
		}
	}
	ts.Conditions = append(ts.Conditions, dc)
}
