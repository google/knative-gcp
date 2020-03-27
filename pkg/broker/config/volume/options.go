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

package volume

// Option is the option to load targets.
type Option func(*Targets)

// WithPath is the option to load targets from the given path.
func WithPath(path string) Option {
	return func(t *Targets) {
		t.path = path
	}
}

// WithNotifyChan is the option to notify the given channel
// when the config cache was updated.
func WithNotifyChan(ch chan<- struct{}) Option {
	return func(t *Targets) {
		t.notifyChan = ch
	}
}
