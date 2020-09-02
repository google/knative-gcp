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

package helpers

import (
	"context"
	"fmt"
	"testing"
	"time"

	trace "cloud.google.com/go/trace/apiv1"
	"google.golang.org/genproto/googleapis/devtools/cloudtrace/v1"
)

func VerifyTrace(t *testing.T, testTree TestSpanTree, projectID string, traceID string) {
	ctx := context.Background()
	client, err := trace.NewClient(ctx)
	if err != nil {
		t.Error(err)
		return
	}
	timeout := time.After(time.Minute)
	for {
		err = tryVerifyBrokerTrace(ctx, nil, testTree, client, projectID, traceID)
		if err == nil {
			return
		}
		select {
		case <-timeout:
			if err := tryVerifyBrokerTrace(ctx, t, testTree, client, projectID, traceID); err != nil {
				t.Error(err)
			}
			return
		case <-time.After(time.Second):
		}
	}
}

func tryVerifyBrokerTrace(ctx context.Context, t *testing.T, testTree TestSpanTree, client *trace.Client, projectID string, traceID string) error {
	trace, err := client.GetTrace(ctx, &cloudtrace.GetTraceRequest{
		ProjectId: projectID,
		TraceId:   traceID,
	})
	if err != nil {
		return err
	}
	tree, err := GetTraceTree(trace)
	if err != nil {
		return err
	}
	if len(testTree.MatchesSubtree(t, tree)) == 0 {
		return fmt.Errorf("expected subtree %v, got %v", testTree, tree)
	}
	return nil
}

// SpanTree is the tree of Spans representation of a Trace.
type SpanTree struct {
	Span     *cloudtrace.TraceSpan
	Children []SpanTree
}

type SpanMatcher struct {
	Name   string
	Kind   *cloudtrace.TraceSpan_SpanKind
	Labels map[string]string
}

func (m *SpanMatcher) MatchesSpan(span *cloudtrace.TraceSpan) error {
	if m == nil {
		return nil
	}
	if m.Kind != nil {
		if got := span.GetKind(); got != *m.Kind {
			return fmt.Errorf("mismatched span kind: got %q, want %q", got, *m.Kind)
		}
	}
	if got := span.GetName(); m.Name != "" && m.Name != got {
		return fmt.Errorf("mismatched span name: got %q, want %q", got, m.Name)
	}
	for k, v := range m.Labels {
		if got := span.GetLabels()[k]; got != v {
			return fmt.Errorf("unexpected label %s: got %q, want %q", k, got, v)
		}
	}
	return nil
}

// TestSpanTree is the expected version of SpanTree used for assertions in testing.
type TestSpanTree struct {
	Name     string
	Span     *SpanMatcher
	Children []TestSpanTree
}

// GetTraceTree converts a set slice of spans into a SpanTree.
func GetTraceTree(trace *cloudtrace.Trace) (*SpanTree, error) {
	children := map[uint64][]*cloudtrace.TraceSpan{}
	spans := map[uint64]*cloudtrace.TraceSpan{}
	for _, span := range trace.GetSpans() {
		children[span.GetParentSpanId()] = append(children[span.GetParentSpanId()], span)
		if spans[span.GetSpanId()] != nil {
			return nil, fmt.Errorf("spans with duplicate ID: %v, %v", span, spans[span.GetSpanId()])
		}
		spans[span.GetSpanId()] = span
	}
	var root *cloudtrace.TraceSpan
	for _, span := range trace.GetSpans() {
		if spans[span.GetParentSpanId()] != nil {
			continue
		}
		if root != nil {
			return nil, fmt.Errorf("multiple root spans: %v, %v", root, span)
		}
		root = span
		delete(children, root.GetParentSpanId())
	}
	if root == nil {
		return nil, fmt.Errorf("no root span found in %v", trace)
	}
	tree := mkTree(children, root)
	if len(children) != 0 {
		return nil, fmt.Errorf("left over spans after generating the SpanTree: %v. Original: %v", children, trace)
	}
	return &tree, nil
}

func mkTree(children map[uint64][]*cloudtrace.TraceSpan, root *cloudtrace.TraceSpan) SpanTree {
	var trees []SpanTree
	for _, span := range children[root.GetSpanId()] {
		trees = append(trees, mkTree(children, span))
	}
	delete(children, root.GetSpanId())
	return SpanTree{
		Span:     root,
		Children: trees,
	}
}

// MatchesSubtree checks to see if this TestSpanTree matches a subtree
// of the actual SpanTree. It is intended to be used for assertions
// while testing. Returns the set of possible subtree matches with the
// corresponding set of unmatched siblings.
func (tt TestSpanTree) MatchesSubtree(t *testing.T, actual *SpanTree) (matches [][]SpanTree) {
	if t != nil {
		t.Helper()
		t.Logf("attempting to match test tree %v against %v", tt, actual)
	}
	if err := tt.Span.MatchesSpan(actual.Span); err == nil {
		if t != nil {
			t.Logf("%v matches span %v, matching children", tt.Span, actual.Span)
		}
		// Tree roots match; check children.
		if err := matchesSubtrees(t, tt.Children, actual.Children); err == nil {
			// A matching root leaves no unmatched siblings.
			matches = append(matches, nil)
		}
	} else if t != nil {
		t.Logf("%v does not match span %v: %v", tt.Span, actual.Span, err)
	}
	// Recursively match children.
	for i, child := range actual.Children {
		for _, childMatch := range tt.MatchesSubtree(t, &child) {
			// Append unmatched children to child results.
			childMatch = append(childMatch, actual.Children[:i]...)
			childMatch = append(childMatch, actual.Children[i+1:]...)
			matches = append(matches, childMatch)
		}
	}
	return
}

// matchesSubtrees checks for a match of each TestSpanTree with a
// subtree of a distrinct actual SpanTree.
func matchesSubtrees(t *testing.T, ts []TestSpanTree, as []SpanTree) error {
	if t != nil {
		t.Helper()
		t.Logf("attempting to match test trees %v against %v", ts, as)
	}
	if len(ts) == 0 {
		return nil
	}
	tt := ts[0]
	for j, a := range as {
		// If there is no error, then it matched successfully.
		for _, match := range tt.MatchesSubtree(t, &a) {
			asNew := make([]SpanTree, 0, len(as)-1+len(match))
			asNew = append(asNew, as[:j]...)
			asNew = append(asNew, as[j+1:]...)
			asNew = append(asNew, match...)
			if err := matchesSubtrees(t, ts[1:], asNew); err == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("unmatched span trees. want: %v got %s", ts, as)
}

func BrokerTestTree(namespace string, brokerName string, trigger string, respTrigger string) TestSpanTree {
	response := TestSpanTree{
		Span: ingressSpan(namespace, brokerName),
		Children: []TestSpanTree{
			{
				Span: triggerSpan(namespace, trigger),
			},
			{
				Span: triggerSpan(namespace, respTrigger),
			},
		},
	}
	return TestSpanTree{
		// Initial event ingress
		Span: ingressSpan(namespace, brokerName),
		Children: []TestSpanTree{
			{
				// Transform trigger and response
				Span: triggerSpan(namespace, trigger),
				Children: []TestSpanTree{
					response,
				},
			},
			{
				Span: triggerSpan(namespace, respTrigger),
			},
		},
	}
}

func ingressSpan(namespace string, broker string) *SpanMatcher {
	brokerName := fmt.Sprintf("broker:%s.%s", broker, namespace)
	spanName := fmt.Sprintf("Recv.%s", brokerName)
	return &SpanMatcher{
		Name: spanName,
		Labels: map[string]string{
			"messaging.system":      "knative",
			"messaging.destination": brokerName,
		},
	}
}

func triggerSpan(namespace string, trigger string) *SpanMatcher {
	triggerName := fmt.Sprintf("trigger:%s.%s", trigger, namespace)
	return &SpanMatcher{
		Name: triggerName,
		Labels: map[string]string{
			"messaging.system":      "knative",
			"messaging.destination": triggerName,
		},
	}
}
