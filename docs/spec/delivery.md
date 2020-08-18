# Delivery Implementation

This document outlines how GCP Brokers handle the translation from the
[Knative Eventing delivery specification](https://github.com/knative/eventing/tree/master/docs/delivery)
to the
[Cloud Pub/Sub subscription configuration](https://pkg.go.dev/cloud.google.com/go/pubsub?tab=doc#SubscriptionConfig).

The Knative Eventing delivery specification allows for the configuration of a
backoff retry policy and a dead letter policy.

## Dead Letter Policy

A Pub/Sub subscription has its dead letter policy configured through the
subscription configuration member `DeadLetterPolicy`.

The Knative dead letter policy is specified through the following parameters in
the Knative Eventing delivery spec:

- `DeadLetterSink`: We only allow special URLs as the dead letter sink, of the
  form `pubsub://[dead_letter_sink_topic]`. We assume that if a topic is
  specified, it already exists.
- `Retry`: This is the number of delivery attempts until the event is forwarded
  to the dead letter topic. Mapped to the Pub/Sub dead letter policy's
  `MaxDeliveryAttempts`.

## Retry Policy

A Pub/Sub subscription has its backoff retry policy configured through the
subscription configuration member `RetryPolicy`.

Pub/Sub supports an
[exponential backoff](https://en.wikipedia.org/wiki/Exponential_backoff) retry
policy based on the specified minimum backoff delay, and capped to some
specified maximum backoff delay.

- `BackoffDelay`: This is mapped to the retry policy's `MinimumBackoff`.
- `BackoffPolicy`: There are two options for the backoff policy:
  - `linear`: In this case, the retry policy's `MaximumBackoff` is set to be
    equal to the `MinimumBackoff`, which sets the delay between each retry to be
    equal.
  - `exponential`: In this case, the retry policy's `MaximumBackoff` is set to
    600 seconds, which is the largest value allowed by Pub/Sub.
