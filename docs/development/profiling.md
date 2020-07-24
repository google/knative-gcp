# Profiling

## Which services can be profiled?

Commands that use the `mainhelper.Init` function support profiling as described
below. Currently this includes:

- broker-ingress
- broker-fanout
- broker-retry

## Enabling the pprof HTTP server

The pprof HTTP server is enabled automatically when `config-observability` in
the system namespace (normally `cloud-run-events`) includes the key
`profiling.enable` with any value. The profiling port is 8008. See
[`server.go`](https://github.com/knative/pkg/blob/master/profiling/server.go)
for more details.

## Enabling the GCP Profiler

The GCP Profiler may be enabled and configured using these environment
variables:

- `GCP_PROFILER=1` enables the GCP Profiler.
- `GCP_PROFILER_PROJECT=<project>` sets the GCP project to use. This is optional
  when running on GCP (unless the GCE metadata server has been disabled).
- `GCP_PROFILER_SERVICE_VERSION=0.1` sets the version string of the service
  being profiled. This may be used to compare profiler results of different
  versions of a service. Defaults to 0.1.
- `GCP_PROFILER_DEBUG_LOGGING=1` enables debug logging for the GCP profiler
  client package. Defaults to false.
