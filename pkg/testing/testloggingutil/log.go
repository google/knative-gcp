/*
Copyright 2020 Google LLC.

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

package testloggingutil

import (
	"os"

	v1 "k8s.io/api/core/v1"

	"go.uber.org/zap"
)

// LogBasedOnEnv is used by the TestLogging* E2E tests. It logs values, using the provided logger,
// based on the environment variables.
func LogBasedOnEnv(logger *zap.Logger) {
	if v := os.Getenv(LoggingE2ETestEnvVarName); v != "" {
		// This is added purely for the TestCloudLogging E2E tests, which verify that the log line
		// is written if this annotation is present.
		logger.Error("Adding log line for the TestCloudLogging E2E tests", zap.String(LoggingE2EFieldName, v))
	}
}

// LogBasedOnAnnotations is used by the TestLogging* E2E tests. It logs values, using the provided
// logger, based on the annotations.
func LogBasedOnAnnotations(logger *zap.Logger, annotations map[string]string) {
	if v, present := annotations[LoggingE2ETestAnnotation]; present {
		// This is added purely for the TestCloudLogging E2E tests, which verify that the log line
		// is written if this annotation is present.
		logger.Error("Adding log line for the TestCloudLogging E2E test", zap.String(LoggingE2EFieldName, v))
	}
}

// PropagateLoggingE2ETestAnnotation propagates the annotation used by the TestLogging* E2E tests.
// It returns the updated environment variables that should be used in that container.
func PropagateLoggingE2ETestAnnotation(inputAnnotations map[string]string, outputEnvVars []v1.EnvVar) []v1.EnvVar {
	if v, present := inputAnnotations[LoggingE2ETestAnnotation]; present {
		outputEnvVars = append(outputEnvVars, v1.EnvVar{
			Name:  LoggingE2ETestEnvVarName,
			Value: v,
		})
	}
	return outputEnvVars
}
