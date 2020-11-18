# CE Test Actor

## What It Does?

It's a set of tools to help test eventing intermediary. It consists of the
following components:

- [Seeder](./seeder) generates events and sends them to a given address.
- [Actor](./actor) listens for events and could be configured to delay responses
  or return errors.
- (Optional) [Aggregator](./aggregator) listens for metrics from Seeder(s) and
  Actor(s) and output the results in the termination log.
- (Optional) [Namespace generator](./ns-gen) generates a list of yamls that
  could be applied in sequence to set up a namespace to carry out one elapsed
  test.

For detailed configuration for each component, please look at the corresponding
`main.go`.

## How it could be used?

The [original version](https://github.com/yolocs/ce-test-actor) of the tool was
mostly used for performance and load testing. In this example, we will go
through such a case.

Start with building the `ns-gen` and prepare a dir for generated yamls.

```
go build ./ns-gen && mkdir actor-test
```

Generate the yamls for the test.

```
./ns-gen -actors=2 -elapse=2m -ns=new-actor-1 -output=$PWD/actor-new -seeders=2 -triggers=5
```

The flags:

- `-actors=2` means we want 2 replicas for each actor deployment.
- `-elapse=2m` means we want all the `seeders` to send events for 2 minutes.
- `-ns=new-actor-1` means we want to run the test in namespace `new-actor-1`
- `-output=$PWD/actor-new` is the absolute path of the output dir for the yamls
- `-seeders=2` means we want 2 replicas for the seeder.
- `-triggers=5` means we want 5 triggers each with its own actor deployment.

Now `tree ./actor-test` we should see:

```
./actor-test
├── 00-namespace.yaml
├── 01-broker.yaml
├── 02-aggregator.yaml
├── 03-triggers.yaml
└── 04-seeder.yaml
```

Feel free to look into the yamls for what each does. In short:

- The seeder will send events to the broker.
- The triggers will subscribe actors to the Broker.
- The seeder and the actors will report metrics to the aggreator in the end.

Apply all of the yamls `ko apply -f ./actor-test`. Wait about 3 minutes and we
should see the aggregaor job gets terminated. By looking at the terminated pod
termination message, we should see something like:

```
...
    state:
      terminated:
        containerID: docker://b31da82fa4b2868d5da41995693ec34e9d18495b18d17063220b75981664c7c3
        exitCode: 0
        finishedAt: "2020-11-18T00:14:09Z"
        # Here is the output of the aggregator.
        # Successfully sent 240 from the seeder in total.
        # Received 1200 in total in the actors.
        message: '{"successSent": 240, "failureSent": 0, "received": 1200}'
        reason: Completed
        startedAt: "2020-11-18T00:11:44Z"

...
```
