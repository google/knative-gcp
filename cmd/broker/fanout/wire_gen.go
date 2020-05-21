// Code generated by Wire. DO NOT EDIT.

//go:generate wire
//+build !wireinject

package main

import (
	"context"

	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/knative-gcp/pkg/broker/config/volume"
	"github.com/google/knative-gcp/pkg/broker/handler/pool"
)

// Injectors from wire.go:

func InitializeSyncPool(ctx context.Context, projectID pool.ProjectID, targetsVolumeOpts []volume.Option, opts ...pool.Option) (*pool.FanoutPool, error) {
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
	fanoutPool, err := pool.NewFanoutPool(readonlyTargets, client, deliverClient, retryClient, opts...)
	if err != nil {
		return nil, err
	}
	return fanoutPool, nil
}

var (
	_wireValue  = pool.DefaultHTTPOpts
	_wireValue2 = pool.DefaultCEClientOpts
)
