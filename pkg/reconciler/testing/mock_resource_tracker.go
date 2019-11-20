/*
Copyright 2019 The Knative Authors

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

package testing

import (
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/tracker"
)

// MockTracker is a mock AddressableTracker.
type MockTracker struct {
	err error
}

var _ tracker.Interface = (*MockTracker)(nil)

// Track implements the tracker.Interface interface.
func (at *MockTracker) Track(ref corev1.ObjectReference, obj interface{}) error {
	return nil
}

// TrackReference implements the tracker.Interface interface.
func (at *MockTracker) TrackReference(ref tracker.Reference, obj interface{}) error {
	return nil
}

// OnChanged implements the tracker.Interface interface.
func (at *MockTracker) OnChanged(obj interface{}) {
}
