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

type targetKey struct{}

// WithTargetKey sets a target key in the context.
func WithTargetKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, targetKey{}, key)
}

// GetTargetKey gets a target key from the context.
func GetTargetKey(ctx context.Context) (string, error) {
	untyped := ctx.Value(targetKey{})
	if untyped == nil {
		return "", ErrTargetKeyNotPresent
	}
	return untyped.(string), nil
}
