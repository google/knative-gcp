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

package main

import (
	"flag"
	"log"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/context"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/pubsub/transformer"
)

func main() {
	flag.Parse()

	ctx := context.Background()
	logCfg := zap.NewProductionConfig()
	logCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := logCfg.Build()
	if err != nil {
		log.Fatalf("Unable to create logger: %v", err)
	}

	startable := &transformer.Transformer{}

	logger.Info("Starting Pub/Sub Transformer.", zap.Reflect("transformer", startable))
	if err := startable.Start(ctx); err != nil {
		logger.Fatal("failed to start transformer: ", zap.Error(err))
	}
}
