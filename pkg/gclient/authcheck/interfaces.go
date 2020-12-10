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

// Package authcheck provides interfaces and wrappers around the http client.
package authcheck

import (
	"net/http"
)

type Client interface {
	// Do sends an HTTP request and returns an HTTP response,
	// following policy (such as redirects, cookies, auth) as configured on the client.
	// See https://golang.org/pkg/net/http/#Client.Do
	Do(req *http.Request) (*http.Response, error)
}
