/*
Copyright 2019 Google LLC

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

package lib

type outputSuccess struct {
	Success bool `json:"success"`
}

func (o *outputSuccess) Successful() bool {
	return o.Success
}

type TargetOutput struct {
	outputSuccess
}

type SenderOutput struct {
	outputSuccess
	TraceID string `json:"traceid"`
}

type Output interface {
	Successful() bool
}

type AuthConfig struct {
	WorkloadIdentity   bool
	ServiceAccountName string
}

type PropPair struct {
	Expected string
	Received string
}
