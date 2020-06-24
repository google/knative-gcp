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
)

// The key used to store/retrieve the topic in the context.
type topicKey struct{}

// WithTopicKey sets a topic key in the context.
func WithTopicKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, topicKey{}, key)
}

// GetTopicKey gets the topic key from the context.
func GetTopicKey(ctx context.Context) (string, error) {
	untyped := ctx.Value(topicKey{})
	if untyped == nil {
		return "", ErrTopicKeyNotPresent
	}
	return untyped.(string), nil
}
