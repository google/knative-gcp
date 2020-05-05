# Overview
This pod is used to measure unreceived (but accepted) message sends.

# Details
This pod is designed to be both an event sender and an event receiver.  It
sends a series of events through a single threaded sender, keeping track of the
response code for each send.  The receiver maintains a list
of received events.  When stopped, the receiver than waits several minutes
for all events to be delivered.

The ranges of events successfully sent and received are then compared, with
any mismatches seen.  It is expected that no events will have been successfully
sent, but not received.  These are returned as failures.  It is possible (on
the other hand) that an event may not have been successfully sent, but will be
received.  For instance, if the ingress of a broker is killed after it enqueues
onto pubsub, but before the handler returns).  These are reported as warnings.
The number of duplicate receive events seen is also reported (no duplicate sends
are made, but reliable forwarders like the broker may forward some events more than
once if they are unsure that the previous forward worked)

The yaml's in this directory hard code it to send to the knative GCP
broker for the default namespace and to receive events via a trigger.
It must be run as a deployment with a single pod.  They can be deployed with
  ko apply -f .

This pod then provides a very basic 5 call http "api" to control data taking.
This API can be reached on the pods port 8070 either via kubectl port forwarding
or via a curl pod run inside the cluster.

# Control
- curl sendtracker:8070/ready
  returns "true" once the sender has successfully sent a startup message to the receiver.

- curl sendtracker:8070/start
  starts sending events to the receiver.  After this is run, the user should then do whatever
  perturbations are are desired to test (upgrades, failure injection, etc)

- curl sendtracker:8070/stop
  stops sending events to the receiver, but waits a configurable time for events to be delivered

- curl sendtracker:8070/resultsready
  returns "true" once the receiver has finished listening, and results are ready.

- curl sendtracker:8070/ready
  returns a human readable summary of the run.  The first line is "success" if no errors were seen
  and "failure" if errors were seen.  The following lines are human readable info about errors, warnings,
  all send results, all events received, and the number of duplicate events seen.

To run again, delete the pod and let the deployment restart it.

# Tuning
There are 3 parameters that can be tuned for the deployment.

* K\_SINK - The target for the events.  This target should eventually feed these events back to the pod.
* DELAY\_MS - The time to wait (in milliseconds) between event sends
* POST\_STOP\_SECS - The time to wait (in seconds) between the user request to stop and the receiver stopping to record received events.
