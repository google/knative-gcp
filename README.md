# Knative + Google Cloud Platform = 🚀

[![GoDoc](https://godoc.org/github.com/google/knative-gcp?status.svg)](https://godoc.org/github.com/google/knative-gcp)
[![Go Report Card](https://goreportcard.com/badge/google/knative-gcp)](https://goreportcard.com/report/google/knative-gcp)
[![LICENSE](https://img.shields.io/github/license/google/knative-gcp.svg)](https://github.com/google/knative-gcp/blob/master/LICENSE)

Knative-GCP builds on Kubernetes to enable easy configuration and consumption of
Google Cloud Platform events and services. It can be useful independently, but
is best mixed with [Knative](https://knative.dev).

If you are interested in contributing, see [DEVELOPMENT.md](./DEVELOPMENT.md).

## Installing Knative-GCP

Follow this guide to install Knative-GCP components on a platform of your
choice.

1. [Installing Knative-GCP](./docs/install/install-knative-gcp.md)
1. [Installing a Service Account for the Data Plane](./docs/install/dataplane-service-account.md)
1. [Installing GCP Broker](./docs/install/install-gcp-broker.md)
1. [Installing Broker with PubSub Channel](./docs/install/install-broker-with-pubsub-channel.md)
1. [Managing Multiple Projects](./docs/install/managing-multiple-projects.md)

## Operating Knative-GCP

The following guides pertain to operating an existing Knative-GCP installation.

1. [Accessing Event Traces in Cloud Trace](./docs/how-to/cloud-trace.md)

## Knative-GCP Sources

In order to consume events from different GCP services, Knative-GCP provides
different Sources. A Source is a Kubernetes object that generate or import
events into the cluster and sends them downstream in
[CloudEvents](https://cloudevents.io/) format. Use the examples below to learn
how to configure and consume events from different GCP services.

1. [CloudPubSubSource](./docs/examples/cloudpubsubsource/README.md)
1. [CloudStorageSource](./docs/examples/cloudstoragesource/README.md)
1. [CloudSchedulerSource](./docs/examples/cloudschedulersource/README.md)
1. [CloudAuditLogsSource](./docs/examples/cloudauditlogssource/README.md)
1. [CloudBuildSource](./docs/examples/cloudbuildsource/README.md)

All of the above Sources are Pull-based, i.e., they poll messages from Pub/Sub
subscriptions. Different mechanisms can be used to scale them out. Roughly
speaking, all such mechanisms need metrics to understand how "congested" the
Pub/Sub subscription is and inform their scaling decision subsystem. We
currently support the following scaling mechanisms:

1. [Keda-based Scaling](./docs/examples/keda/README.md)

## Pub/Sub Channel

A Channel is a Knative Eventing logical construct that provides an event
delivery mechanism which can fan-out received events to multiple destinations
via Subscriptions. A Channel has a single inbound HTTP-addressable interface,
which may accept events delivered directly or forwarded from multiple
Subscriptions. Use the example below if you want to use our Channel backed by
[Cloud Pub/Sub](https://cloud.google.com/pubsub/docs/overview), which offers
at-least-once message delivery and best-effort ordering to existing subscribers.

1. [Channel](./docs/examples/channel/README.md)

## Pub/Sub Core Resources

In [Cloud Pub/Sub](https://cloud.google.com/pubsub/docs/overview), a publisher
application creates and sends messages to a topic, while subscriber applications
create a subscription to a topic in order to receive messages from it. If you
want to interact directly with Cloud Pub/Sub topics and subscriptions within
your Kubernetes cluster, use our custom Kubernetes resources below.

1. [Topic](./docs/examples/topic/README.md)
1. [PullSubscription](./docs/examples/pullsubscription/README.md)

_Note:_ This repo is still in development, APIs and resource names are subject
to change in the future.
