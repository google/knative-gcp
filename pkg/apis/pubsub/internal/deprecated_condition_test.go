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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	conditions duckv1.Conditions = []apis.Condition{
		{
			Type:               "PublisherReady",
			Status:             "True",
			Severity:           "Warning",
			LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now())},
			Reason:             "publisherReady",
			Message:            "The Publisher is ready",
		},
	}

	conditionsWithDeprecated = append(conditions.DeepCopy(), deprecatedCondition)
)

func TestMarkDeprecated_NotPresent(t *testing.T) {
	want := conditionsWithDeprecated.DeepCopy()
	got := MarkDeprecated(conditions.DeepCopy())
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected status (-want +got): %v", diff)
	}
}

func TestMarkDeprecated_OnlyTimeChanges(t *testing.T) {
	// If only time changes, then we don't want to make the change. So in this case the initial
	// conditions are the same as the wanted conditions.
	_, want := addDeprecatedCondition(conditions, func(c *apis.Condition) {
		c.LastTransitionTime = apis.VolatileTime{
			Inner: metav1.NewTime(c.LastTransitionTime.Inner.Add(-1 * time.Minute)),
		}
	})
	got := MarkDeprecated(want)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected status (-want +got): %v", diff)
	}
}

func TestMarkDeprecated_ReasonChanges(t *testing.T) {
	init, want := addDeprecatedCondition(conditions, func(c *apis.Condition) {
		c.Reason = "someOtherReason"
	})
	got := MarkDeprecated(init)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected status (-want +got): %v", diff)
	}
}

func TestMarkDeprecated_NoChange(t *testing.T) {
	init := conditionsWithDeprecated.DeepCopy()
	want := conditionsWithDeprecated.DeepCopy()
	got := MarkDeprecated(init)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected status (-want +got): %v", diff)
	}
}

// addDeprecatedCondition adds the deprecated condition to the passed in initial conditions. It also
// applies the function f to the the 'altered' conditions. This is intended to be the input to
// MarkDeprecated, whereas 'unaltered' conditions are meant to the output.
func addDeprecatedCondition(init duckv1.Conditions, f func(*apis.Condition)) (altered, unaltered duckv1.Conditions) {
	dc := deprecatedCondition.DeepCopy()
	f(dc)
	altered = append(init, *dc)
	unaltered = append(init, *deprecatedCondition.DeepCopy())
	return altered, unaltered
}
