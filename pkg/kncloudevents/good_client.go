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

package kncloudevents

import (
	cev2 "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
)

func NewDefaultClient(target ...string) (cev2.Client, error) {
	var tOpts []http.Option
	if len(target) > 0 && target[0] != "" {
		tOpts = append(tOpts, cev2.WithTarget(target[0]))
	}

	// Make an http transport for the CloudEvents client.
	t, err := cev2.NewHTTP(tOpts...)
	if err != nil {
		return nil, err
	}

	// Use the transport to make a new CloudEvents client.
	c, err := cev2.NewClient(t,
		cev2.WithUUIDs(),
		cev2.WithTimeNow(),
	)

	if err != nil {
		return nil, err
	}
	return c, nil
}
