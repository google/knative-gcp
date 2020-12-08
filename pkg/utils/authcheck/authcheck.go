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

// Package authcheck provides utilities to check authentication configuration for data plane resources.
// File authcheck contains functions to run customized checks inside of a Pod.
package authcheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	nethttp "net/http"

	"go.uber.org/zap"
	"golang.org/x/oauth2/google"
)

const (
	// Resource is used as the path to get the default token from metadata server.
	// In workload-identity-gsa mode, this path will return a token if
	// corresponding k8s service account and google service account establish a correct relationship.
	resource = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"
	// Scope is used as the scope to get token from default credential.
	scope = "https://www.googleapis.com/auth/cloud-platform"
)

// AuthenticationCheck performs the authentication check running in the Pod.
func AuthenticationCheck(ctx context.Context, logger *zap.Logger, authType AuthType, response nethttp.ResponseWriter) {
	var err error
	if authType == Secret {
		err = AuthenticationCheckForSecret(ctx)
	} else if authType == WorkloadIdentityGSA {
		err = AuthenticationCheckForWorkloadIdentityGSA(resource)
	} else {
		logger.Error(fmt.Sprint("unknown auth type: ", authType))
		response.WriteHeader(nethttp.StatusUnauthorized)
		return
	}

	if err != nil {
		// Transfer the error into a string message, otherwise, marshalling error may return nil unexpectedly.
		message := fmt.Sprintf("using %s mode, when checking authentication, get error: %s", authType, err.Error())
		b, err := json.Marshal(map[string]interface{}{
			"error": message,
		})
		if err != nil {
			logger.Error(fmt.Sprint("error marshalling the message: ", message), zap.Error(err))
			response.WriteHeader(nethttp.StatusUnauthorized)
			return
		}
		errs := ioutil.WriteFile("/dev/termination-log", b, 0644)
		if errs != nil {
			logger.Error(fmt.Sprintf("error writing the message: %s into termination log", message), zap.Error(err))
			response.WriteHeader(nethttp.StatusUnauthorized)
			return
		}
		logger.Info(message)
		response.WriteHeader(nethttp.StatusUnauthorized)
		return
	}
	response.WriteHeader(nethttp.StatusOK)
}

// AuthenticationCheckForSecret performs the authentication check for Pod in secret mode.
func AuthenticationCheckForSecret(ctx context.Context) error {
	cred, err := google.FindDefaultCredentials(ctx, scope)
	if err != nil {
		return fmt.Errorf("error finding the default credential: %w", err)
	}
	s, err := cred.TokenSource.Token()
	if err != nil {
		return fmt.Errorf("error getting the token, probably due to the key stored in the Kubernetes Secret is expired or revoked: %w", err)
	}
	if !s.Valid() {
		return errors.New("token is not valid")
	}
	return nil
}

// AuthenticationCheckForWorkloadIdentityGSA performs the authentication check for Pod in workload-identity-gsa mode.
func AuthenticationCheckForWorkloadIdentityGSA(resource string) error {
	req, err := http.NewRequest(http.MethodGet, resource, nil)
	if err != nil {
		return fmt.Errorf("error setting up the http request: %w", err)
	}
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error getting the http response: %w", err)
	}
	defer resp.Body.Close()
	// Check if we can successfully get the token.
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return errors.New("the Pod is not fully authenticated, " +
			"probably due to corresponding k8s service account and google service account do not establish a correct relationship")
	}
	return nil
}
