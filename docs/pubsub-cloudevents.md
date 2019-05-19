# PubSub Bindings

TODO: Make this like
https://github.com/cloudevents/spec/blob/master/http-transport-binding.md

Pubsub has the following:

```go
type Message struct {
    // ID identifies this message.
    // This ID is assigned by the server and is populated for Messages obtained from a subscription.
    // This field is read-only.
    ID  string

    // Data is the actual data in the message.
    Data []byte

    // Attributes represents the key-value pairs the current message
    // is labelled with.
    Attributes map[string]string

    // The time at which the message was published.
    // This is populated by the server for Messages obtained from a subscription.
    // This field is read-only.
    PublishTime time.Time
    // contains filtered or unexported fields
}
```

The ID provided by PubSub is not the CloudEvents.ID. The PublishTime provided by
PubSub is not the CloudEvents.Time

## Binary

The naming convention for the Attributes mapping of well-known CloudEvents
attributes is that each attribute name MUST be prefixed with "ce-".

Examples:

- `time` maps to `ce-time`
- `id` maps to `ce-id`
- `specversion` maps to `ce-specversion`

Data goes in Data.

## Structured

The structured content mode keeps event metadata and data together in the data
payload, allowing simple forwarding of the same event across multiple routing
hops, and across multiple transports.
