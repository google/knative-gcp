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

package utils

import (
	"sync"
	"time"
)

// SyncTime is a synchronized wrapper around a time.
type SyncTime struct {
	sync.RWMutex
	time time.Time
}

// SetNow sets the desired timestamp to the current time.
func (t *SyncTime) SetNow() {
	t.Lock()
	defer t.Unlock()
	t.time = time.Now()
}

// Get gets the timestamp's time.
func (t *SyncTime) Get() time.Time {
	t.RLock()
	defer t.RUnlock()
	return t.time
}

// SyncTimesMap is a synchronized wrapped around a map of times.
type SyncTimesMap struct {
	sync.RWMutex
	Times map[string]time.Time
}
