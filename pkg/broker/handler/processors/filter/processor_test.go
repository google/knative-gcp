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

package filter

import (
	"context"
	"testing"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/google/go-cmp/cmp"

	"github.com/google/knative-gcp/pkg/broker/config"
	handlerctx "github.com/google/knative-gcp/pkg/broker/handler/context"
	"github.com/google/knative-gcp/pkg/broker/handler/processors"
)

func TestFilterProcessor(t *testing.T) {
	next := &processors.FakeProcessor{}
	p := &Processor{}
	p.WithNext(next)

	t.Run("no filter", func(t *testing.T) {
		ch := make(chan *event.Event)
		next.PrevEventsCh = ch
		ctx := handlerctx.WithTarget(
			context.Background(),
			&config.Target{
				Name:      "name",
				Namespace: "namespace",
			},
		)
		e := event.New()
		e.SetID("id")
		e.SetSource("source")

		go func() {
			gotEvent := <-ch
			if diff := cmp.Diff(&e, gotEvent); diff != "" {
				t.Errorf("processed event (-want,+got): %v", diff)
			}
		}()

		if err := p.Process(ctx, &e); err != nil {
			t.Errorf("unexpected error from processing: %v", err)
		}
		close(ch)
	})

	t.Run("with filter", func(t *testing.T) {
		ch := make(chan *event.Event)
		next.PrevEventsCh = ch
		ctx := handlerctx.WithTarget(
			context.Background(),
			&config.Target{
				Name:      "name",
				Namespace: "namespace",
				FilterAttributes: map[string]string{
					"subject": "foo",
					"type":    "bar",
				},
			},
		)
		pass := event.New()
		pass.SetID("id")
		pass.SetSubject("foo")
		pass.SetType("bar")
		notpass := event.New()

		go func() {
			// Range channel and we only expect "pass" event to be present.
			for gotEvent := range ch {
				if diff := cmp.Diff(&pass, gotEvent); diff != "" {
					t.Errorf("processed event (-want,+got): %v", diff)
				}
			}
		}()

		if err := p.Process(ctx, &pass); err != nil {
			t.Errorf("unexpected error from processing: %v", err)
		}
		if err := p.Process(ctx, &notpass); err != nil {
			t.Errorf("unexpected error from processing: %v", err)
		}
		close(ch)
	})
}
