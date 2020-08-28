/*
Copyright 2020 Google LLC.

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

package eventutil

import (
	"context"
	"testing"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/transformer"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/google/go-cmp/cmp"
)

func TestImmutableEventMessage(t *testing.T) {
	e := event.New()
	e.SetID("id")
	e.SetType("type")
	e.SetSource("source")

	e.SetExtension("ext", "val")
	e.SetExtension("ext2", "val2")

	e2, err := binding.ToEvent(context.TODO(), NewImmutableEventMessage(&e), transformer.DeleteExtension("ext"))
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(map[string]interface{}{"ext2": "val2"}, e2.Extensions()); diff != "" {
		t.Errorf("unexpected extensions in transformed event (-want, +got) = %v", diff)
	}

	if diff := cmp.Diff(map[string]interface{}{"ext": "val", "ext2": "val2"}, e.Extensions()); diff != "" {
		t.Errorf("unexpected mutation of event extensions (-want, +got) = %v", diff)
	}
}
