# GCP Broker Examples

These examples are adapted from
https://knative.dev/docs/eventing/getting-started/.

## Prerequisites

[Install GCP Broker](../../install/install-gcp-broker.md)

## Apply the Example YAMLs

Apply example yamls in [this directory](./):

```shell
kubectl apply -f namespace.yaml
kubectl apply -f event-publisher.yaml
kubectl apply -f event-consumers.yaml
kubectl apply -f broker.yaml
kubectl apply -f triggers.yaml
```

The yamls create the following resources:

- `cloud-run-events-example` namespace
- A pod in which you can run `curl` as the event producer
- 2 Kubernetes services `hello-display` and `goodbye-display` as event consumers
- A GCP broker named `test-broker`
- 2 Triggers `hello-display` and `goodbye-display` that points to the 2
  consumers, respectively. `hello-display` filters events with `type: greeting`
  attribute and `goodbye-display` filters events with `source: sendoff`
  attribute.

Verify the broker is ready:

```shell
kubectl -n cloud-run-events-example get broker test-broker
```

```shell
NAME          READY   REASON   URL                                                                                                         AGE
test-broker   True             http://default-brokercell-ingress.cloud-run-events.svc.cluster.local/cloud-run-events-example/test-broker   9s
```

Verify the triggers are ready:

```shell
kubectl -n cloud-run-events-example get triggers
```

```shell
NAME              READY   REASON   BROKER        SUBSCRIBER_URI                                                       AGE
goodbye-display   True             test-broker   http://goodbye-display.cloud-run-events-example.svc.cluster.local/   4s
hello-display     True             test-broker   http://hello-display.cloud-run-events-example.svc.cluster.local/     4s
```

## Send Events to the Broker

SSH into the event publisher pod by running the following command:

```sh
  kubectl -n cloud-run-events-example attach curl -it
```

A prompt similar to the one below will appear:

```sh
    Defaulting container name to curl.
    Use 'kubectl describe pod/ -n cloud-run-events-example' to see all of the containers in this pod.
    If you don't see a command prompt, try pressing enter.
    [ root@curl:/ ]$
```

To show the various types of events you can send, you will make three requests:

1. To make the first request, which creates an event that has the
   `type:greeting`, run the following in the SSH terminal:

   ```sh
   curl -v "http://default-brokercell-ingress.cloud-run-events.svc.cluster.local/cloud-run-events-example/test-broker" \
     -X POST \
     -H "Ce-Id: say-hello" \
     -H "Ce-Specversion: 1.0" \
     -H "Ce-Type: greeting" \
     -H "Ce-Source: not-sendoff" \
     -H "Content-Type: application/json" \
     -d '{"msg":"Hello Cloud Run Events!"}'
   ```

   When the `Broker` receives your event, `hello-display` will activate and send
   it to the event consumer of the same name.

   If the event has been received, you will receive a `202 Accepted` response
   similar to the one below:

   ```sh
   < HTTP/1.1 202 Accepted
   < Date: Thu, 23 Apr 2020 22:14:21 GMT
   < Content-Length: 0
   ```

1. To make the second request, which creates an event that has the
   `source:sendoff`, run the following in the SSH terminal:

   ```sh
   curl -v "http://default-brokercell-ingress.cloud-run-events.svc.cluster.local/cloud-run-events-example/test-broker" \
     -X POST \
     -H "Ce-Id: say-goodbye" \
     -H "Ce-Specversion: 1.0" \
     -H "Ce-Type: not-greeting" \
     -H "Ce-Source: sendoff" \
     -H "Content-Type: application/json" \
     -d '{"msg":"Goodbye Cloud Run Events!"}'
   ```

   When the `Broker` receives your event, `goodbye-display` will activate and
   send the event to the event consumer of the same name.

   If the event has been received, you will receive a `202 Accepted` response
   similar to the one below:

   ```sh
     < HTTP/1.1 202 Accepted
   < Date: Thu, 23 Apr 2020 22:14:21 GMT
   < Content-Length: 0
   ```

1. To make the third request, which creates an event that has the
   `type:greeting` and the`source:sendoff`, run the following in the SSH
   terminal:

   ```sh
   curl -v "http://default-brokercell-ingress.cloud-run-events.svc.cluster.local/cloud-run-events-example/test-broker" \
     -X POST \
     -H "Ce-Id: say-hello-goodbye" \
     -H "Ce-Specversion: 1.0" \
     -H "Ce-Type: greeting" \
     -H "Ce-Source: sendoff" \
     -H "Content-Type: application/json" \
     -d '{"msg":"Hello Cloud Run Events! Goodbye Cloud Run Events!"}'
   ```

   When the `Broker` receives your event, `hello-display` and `goodbye-display`
   will activate and send the event to the event consumer of the same name.

   If the event has been received, you will receive a `202 Accepted` response
   similar to the one below:

   ```sh
   < HTTP/1.1 202 Accepted
   < Date: Thu, 23 Apr 2020 22:14:21 GMT
   < Content-Length: 0
   ```

1. Exit SSH by typing `exit` into the command prompt.

You have sent two events to the `hello-display` event consumer and two events to
the `goodbye-display` event consumer (note that `say-hello-goodbye` activates
the trigger conditions for _both_ `hello-display` and `goodbye-display`). You
will verify that these events were received correctly in the next section.

## Verify Event Delivery

After sending events, verify that your events were received by the appropriate
`Subscribers`.

1. Look at the logs for the `hello-display` event consumer by running the
   following command:

   ```sh
   kubectl -n cloud-run-events-example logs -l app=hello-display --tail=100
   ```

   This returns the `Attributes` and `Data` of the events you sent to
   `hello-display`:

   ```sh
   ☁️  cloudevents.Event
   Validation: valid
   Context Attributes,
     specversion: 1.0
     type: greeting
     source: not-sendoff
     id: say-hello
     time: 2019-05-20T17:59:43.81718488Z
     contenttype: application/json
   Extensions,
     knativehistory: default-broker-srk54-channel-24gls.cloud-run-events-example.svc.cluster.local
   Data,
     {
       "msg": "Hello Cloud Run Events!"
     }
   ☁️  cloudevents.Event
   Validation: valid
   Context Attributes,
     specversion: 1.0
     type: greeting
     source: sendoff
     id: say-hello-goodbye
     time: 2019-05-20T17:59:54.211866425Z
     contenttype: application/json
   Extensions,
     knativehistory: default-broker-srk54-channel-24gls.cloud-run-events-example.svc.cluster.local
   Data,
    {
      "msg": "Hello Cloud Run Events! Goodbye Cloud Run Events!"
    }
   ```

1. Look at the logs for the `goodbye-display` event consumer by running the
   following command:

   ```sh
   kubectl -n cloud-run-events-example logs -l app=goodbye-display --tail=100
   ```

   This returns the `Attributes` and `Data` of the events you sent to
   `goodbye-display`:

   ```sh
   ☁️  cloudevents.Event
   Validation: valid
   Context Attributes,
      specversion: 1.0
      type: not-greeting
      source: sendoff
      id: say-goodbye
      time: 2019-05-20T17:59:49.044926148Z
      contenttype: application/json
    Extensions,
      knativehistory: default-broker-srk54-channel-24gls.cloud-run-events-example.svc.cluster.local
   Data,
      {
        "msg": "Goodbye Cloud Run Events!"
      }
    ☁️  cloudevents.Event
    Validation: valid
    Context Attributes,
      specversion: 1.0
      type: greeting
      source: sendoff
      id: say-hello-goodbye
      time: 2019-05-20T17:59:54.211866425Z
      contenttype: application/json
    Extensions,
      knativehistory: default-broker-srk54-channel-24gls.cloud-run-events-example.svc.cluster.local
    Data,
     {
       "msg": "Hello Cloud Run Events! Goodbye Cloud Run Events!"
     }
   ```

## Reply Events

TODO

## Retry Event Delivery

To demonstrate that GCP broker will guarantee at least once delivery, we will
temporarily delete the consumers and verify events will be delivered when the
consumers are back.

1. Delete the event consumers:

   ```shell
   kubectl delete -f event-consumers.yaml
   ```

1. SSH into the event publisher pod by running the following command:

   ```sh
   kubectl -n cloud-run-events-example attach curl -it
   ```

1. Send an event that has the `type:greeting` and the`source:sendoff`:

   ```sh
   curl -v "http://default-brokercell-ingress.cloud-run-events.svc.cluster.local/cloud-run-events-example/test-broker" \
     -X POST \
     -H "Ce-Id: say-hello-goodbye" \
     -H "Ce-Specversion: 1.0" \
     -H "Ce-Type: greeting" \
     -H "Ce-Source: sendoff" \
     -H "Content-Type: application/json" \
     -d '{"msg":"Test GCP Broker Retry"}'
   ```

1. Exit SSH by typing `exit` into the command prompt.

1. Now let's bring the event consumers back:

   ```shell
   kubectl apply -f event-consumers.yaml
   ```

   Wait a couple of seconds, and you should see the event delivered to the event
   consumers.

## Clean Up

```shell
kubectl delete -f event-publisher.yaml
kubectl delete -f event-consumers.yaml
kubectl delete -f triggers.yaml
kubectl delete -f broker.yaml
kubectl delete -f namespace.yaml
```
