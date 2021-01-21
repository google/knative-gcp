Compiling the proto file is done out of band, it is not done automatically. See
https://github.com/google/knative-gcp/issues/2017.

1. Download and install a
   [release](https://github.com/protocolbuffers/protobuf/releases).
   - The current version is
     [v3.14.0](https://github.com/protocolbuffers/protobuf/releases/tag/v3.14.0).
1. From the root of the repo, run:
   ```shell
   protoc pkg/broker/config/targets.proto --go_out=$GOPATH/src
   ```

Note that I had also initially run:

- `go get -u github.com/golang/protobuf/protoc-gen-go`
- `go install github.com/golang/protobuf/protoc-gen-go`

But as far as I am aware, they did not affect anything. Only noted here in case
it actually did make a difference and are required steps. If so, please update
these instructions to say so.
