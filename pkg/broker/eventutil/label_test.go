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

package eventutil

import (
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/go-cmp/cmp"
)

func TestLabeledEevent(t *testing.T) {
	e := cloudevents.NewEvent()
	e.SetSource("example/uri")
	e.SetType("example.type")
	e.SetData(cloudevents.ApplicationJSON, map[string]string{"hello": "world"})
	e.SetExtension("custom", "foo")

	wantDelabeled := e.Clone()
	wantLabeled := e.Clone()
	wantLabeled.SetExtension("kgcplabel1", "val1")
	wantLabeled.SetExtension("kgcplabel2", "val2")

	le := NewLabeledEvent(&e).WithLabel("label1", "val1").WithLabel("label2", "val2")

	wantLabels := map[string]string{
		"label1": "val1",
		"label2": "val2",
	}
	gotLabels := le.GetLabels()
	if diff := cmp.Diff(wantLabels, gotLabels); diff != "" {
		t.Errorf("LabeledEvent.GetLabels() (-want,+got): %v", diff)
	}

	gotDelabeled := le.Delabeled()
	if diff := cmp.Diff(&wantDelabeled, gotDelabeled); diff != "" {
		t.Errorf("LabeledEvent.Delabeled() (-want,+got): %v", diff)
	}
	gotLabeled := le.Event()
	if diff := cmp.Diff(&wantLabeled, gotLabeled); diff != "" {
		t.Errorf("LabeledEvent.Event() (-want,+got): %v", diff)
	}

	le.WithLabel("label1", "").WithLabel("label2", "")
	if gotCnt := len(le.GetLabels()); gotCnt != 0 {
		t.Errorf("Labels count after removing labels got=%d want=0", gotCnt)
	}
}
