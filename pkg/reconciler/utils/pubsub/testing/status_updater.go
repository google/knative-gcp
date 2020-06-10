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

package testing

type StatusUpdater struct {}

func(su *StatusUpdater) MarkTopicFailed(reason, format string, args ...interface{}) {    }
func(su *StatusUpdater) MarkTopicUnknown(reason, format string, args ...interface{}) {    }
func(su *StatusUpdater) MarkTopicReady() {    }
func(su *StatusUpdater) MarkSubscriptionFailed(reason, format string, args ...interface{}) {    }
func(su *StatusUpdater) MarkSubscriptionUnknown(reason, format string, args ...interface{}) {    }
func(su *StatusUpdater) MarkSubscriptionReady() {    }