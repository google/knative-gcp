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

package context

import (
	"context"
	"testing"
)

func TestSubscriptionKey(t *testing.T) {
	_, err := GetSubscriptionKey(context.Background())
	if err != ErrSubscriptionKeyNotPresent {
		t.Errorf("error from GetSubscriptionKey got=%v, want=%v", err, ErrSubscriptionKeyNotPresent)
	}

	wantKey := "key"
	ctx := WithSubscriptionKey(context.Background(), wantKey)
	gotKey, err := GetSubscriptionKey(ctx)
	if err != nil {
		t.Errorf("unexpected error from GetSubscriptionKey: %v", err)
	}
	if gotKey != wantKey {
		t.Errorf("topic key from context got=%s, want=%s", gotKey, wantKey)
	}
}
