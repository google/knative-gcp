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

package pubsubutil

import (
	"context"
	"io/ioutil"

	"cloud.google.com/go/pubsub"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	"golang.org/x/oauth2/google"
)

// GetCredentials gets GCP credentials from a file.
func GetCredentials(ctx context.Context, file string) (*google.Credentials, error) {
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	creds, err := google.CredentialsFromJSON(ctx, bytes, pubsub.ScopePubSub)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to create the GCP credential", zap.Error(err))
		return nil, err
	}
	return creds, nil
}
