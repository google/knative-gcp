/*
Copyright 2019 Google LLC.

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

package logadmin

import (
	"context"

	"cloud.google.com/go/logging/logadmin"
	"google.golang.org/api/option"
)

// CreateFn is a factory function to create a logadmin client.
// Matches the signature of https://godoc.org/cloud.google.com/go/logging/logadmin#NewClient.
type CreateFn func(ctx context.Context, parent string, opts ...option.ClientOption) (Client, error)

func NewClient(ctx context.Context, parent string, opts ...option.ClientOption) (Client, error) {
	return logadmin.NewClient(ctx, parent, opts...)
}
