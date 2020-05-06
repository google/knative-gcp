// Code generated by Wire. DO NOT EDIT.

//go:generate wire
//+build !wireinject

package main

import (
	"context"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/knative-gcp/pkg/broker/config/volume"
	"github.com/google/knative-gcp/pkg/broker/handler/pool"
	"github.com/google/knative-gcp/pkg/broker/handler/pool/fanout"
)

// Injectors from wire.go:

func InitializeSyncPool(ctx context.Context, projectID pool.ProjectID, targetsVolumeOpts []volume.Option, opts ...pool.Option) (*fanout.SyncPool, error) {
	readonlyTargets, err := volume.NewTargetsFromFile(targetsVolumeOpts...)
	if err != nil {
		return nil, err
	}
	client, err := pool.NewPubsubClient(ctx, projectID)
	if err != nil {
		return nil, err
	}
	v := _wireValue
	protocol, err := http.New(v...)
	if err != nil {
		return nil, err
	}
	v2 := _wireValue2
	deliverClient, err := pool.NewDeliverClient(protocol, v2...)
	if err != nil {
		return nil, err
	}
	retryClient, err := pool.NewRetryClient(ctx, client, v2...)
	if err != nil {
		return nil, err
	}
	syncPool, err := fanout.NewSyncPool(readonlyTargets, client, deliverClient, retryClient, opts...)
	if err != nil {
		return nil, err
	}
	return syncPool, nil
}

var (
	_wireValue  = []http.Option(nil)
	_wireValue2 = pool.DefaultCEClientOpts
)
