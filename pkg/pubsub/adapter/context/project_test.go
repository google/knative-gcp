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

func TestProjectKey(t *testing.T) {
	_, err := GetProjectKey(context.Background())
	if err != ErrProjectKeyNotPresent {
		t.Errorf("error from GetProjectKey got=%v, want=%v", err, ErrProjectKeyNotPresent)
	}

	wantKey := "key"
	ctx := WithProjectKey(context.Background(), wantKey)
	gotKey, err := GetProjectKey(ctx)
	if err != nil {
		t.Errorf("unexpected error from GetProjectKey: %v", err)
	}
	if gotKey != wantKey {
		t.Errorf("topic key from context got=%s, want=%s", gotKey, wantKey)
	}
}
