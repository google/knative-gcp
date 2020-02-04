# Installing Broker with PubSub Channel

A channel provides an event delivery mechanism that can fan-out received events to multiple destinations. 
The default channel configuration allows channels to be created without specifying an underlying implementation.
This is useful for users that do not care about the properties a particular channel provides (e.g.,
ordering, persistence, etc.), but are rather fine with using the  
implementation selected by the the operator. The operator controls the default
settings via a `ConfigMap`. For example, when `Brokers` are
created, an operator can configure a default channel to use for their underlying
channel-based implementations.
For [Knative Eventing](https://github.com/knative/eventing), we use `InMemoryChannel` as the default channel for Broker.
We need to modify the `ConfigMap` to install broker with pubsub channel.

   
1.  Patch the configmap in namespace knative-eventing to use pubsub channel as the default channel with [patch-default-ch-config-with-pubsub.yaml](./patch-default-ch-config-with-pubsub.yaml)

    ```shell
    kubectl patch configmap default-ch-webhook -n knative-eventing --patch "$(cat patch-default-ch-config-with-pubsub.yaml)"
    ```

1. Add a label to your namespace with the following command:

    ```shell
    kubectl label namespace default knative-eventing-injection=enabled
    ```

   This gives the `default`  namespace the `knative-eventing-injection` label, which installs broker.

1. Verify that the `Broker` is running

    ```shell
    kubectl --namespace default get broker default
    ```

   This shows the `Broker` that you created:

    ```shell
    NAME      READY   REASON   URL                                                        AGE
    default   True             http://default-broker.event-example.svc.cluster.local      1m
    ```

    When the `Broker` has the `READY=True` state, it can begin to manage any events it receives.
