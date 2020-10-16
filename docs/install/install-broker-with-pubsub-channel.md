# Installing Broker with Pub/Sub Channel

In Knative Eventing, a Broker represents an "event mesh". Events are sent to the
Broker's ingress and are then sent to any subscribers that are interested in
that event. Brokers are currently backed by a configurable Channel. In this
guide, we show an example of how to configure a Broker backed by the Pub/Sub
Channel.

## Prerequisites

1. [Install Knative-GCP](./install-knative-gcp.md). Remember to install
   [Knative Eventing](https://knative.dev/docs/eventing/).

1. [Create a Service Account for the Data Plane](./dataplane-service-account.md).

## Deployment

1.  Verify the configmap `config-br-default-channel` is in the
    `cloud-run-events` namespace.

    ```shell
    kubectl get configmap config-br-default-channel -n cloud-run-events
    ```

    By default, it is assumed to use a default secret. Modify
    [`config-br-default-channel`](config-br-default-channel.yaml) to use a
    different method and apply it. The two options are:

    1.  If you are in GKE and using
        [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity),
        update `serviceAccountName` with the Pub/Sub enabled service account you
        created in
        [Create a Service Account for the Data Plane](./dataplane-service-account.md).

    1.  If you are using standard Kubernetes secrets, but want to use a
        non-default one, update `secret` with your own secret.

    ```shell
    kubectl apply -f config-br-default-channel.yaml
    ```

1.  Patch the configmap in the `knative-eventing` namespace to use the Pub/Sub
    `Channel` as the
    [default channel](https://knative.dev/docs/eventing/channels/default-channels/)
    for Brokers with
    [patch-config-br-defaults-with-pubsub.yaml](./patch-config-br-defaults-with-pubsub.yaml).

    ```shell
    kubectl patch configmap config-br-defaults -n knative-eventing --patch "$(cat patch-config-br-defaults-with-pubsub.yaml)"
    ```

1.  Add the `knative-eventing-injection` label to your namespace with the
    following command.

    ```shell
    kubectl label namespace default knative-eventing-injection=enabled
    ```

    This triggers a reconciliation process that creates the `default Broker` in
    that namespace.

1.  Verify that the `Broker` is running

    ```shell
    kubectl --namespace default get broker default
    ```

    This shows the `Broker` that you created:

    ```shell
    NAME      READY   REASON   URL                                                        AGE
    default   True             http://default-broker.default.svc.cluster.local      1m
    ```

    When the `Broker` has the `READY=True` state, it can start processing any
    events it receives.
