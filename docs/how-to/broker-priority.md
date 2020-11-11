# Configuring Pod Priority for GCP-Broker

## Background

The data plane components of `GCP-broker` are running on Kubernetes clusters in
`cloud-run-events` namespace. Their resource consumption depends on the number
of cluster nodes and number of pods running in total.

As existing components use more resources and as we add more new components,
more cluster and node resources are required to run Kubernetes. This
[summary](https://github.com/google/knative-gcp/issues/1502#issuecomment-664793074)  
illustrates the broker components may at risk to be OOMKilled/evicted/preempted
when node is over-committed.

To protect broker components from eviction or preemption, we would like to give
them higher scheduling/eviction priority. However, this would put other
applications at risk in the case of insufficient resource. To balance the needs,
we have a scaling mechanism for broker components, and an on-going work to
provide configurable resource request and limit.

Beside scaling and config, users are always able to set up pod priority for the
broker components if they regard the broker components as high priority
components and would like to further reduce its possibility to be preempted.
Follow this doc to set up a Pod Priority for your broker components.

## Add PriorityClass for broker

Apply the following yaml to add a
[PriorityClass](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#priorityclass)
for the broker:

```shell
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: broker-priority
value: 1000
preemptionPolicy: Never
globalDefault: false
description: "Priority class for broker. This priority class will not cause other pods to be preempted."
```

Here, we set `PreemptionPolicy: Never` to avoid broker components
[preempting other pods](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#non-preempting-priority-class).

There is no certain criteria to choose the priority value. You can choose a
value which you think is correctly reflecting the importance of the broker
components.

Pods without PriorityClass specified have the default Priority value: `0`. Pods
running critically for the cluster or for the node are generally with a priority
value bigger than `1000000000`(one billion).

## Update broker related deployment with PriorityClass

1. Make sure you have at least one broker in your cluster.
2. Apply the following command to check the broker data plane:
   ```
   kubectl get deployment -n cloud-run-events
   ```
   You are going to update `deployment`s:
   ```
   NAME                         READY   UP-TO-DATE   AVAILABLE   AGE
   default-brokercell-fanout    1/1     1            1           4h59m
   default-brokercell-ingress   1/1     1            1           4h59m
   default-brokercell-retry     1/1     1            1           4h59m
   ```
3. Add `PriorityClass` to each deployment. This field is located at
   `spec.template.spec` of the `deployment`, looks like:
   ```
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: default-brokercell-fanout
   spec:
     replicas: 1
     selector:
        ...
     template:
       ...
       spec:
         containers:
         ...
         priorityClassName: broker-priority
   ```
4. Use the following command to verify the Pod Priority.

   ```
   kubectl get pod -n cloud-run-events -l 'role in (ingress,fanout,retry)' -o yaml
   ```

   You will find a new field `Priority` under `spec.template.spec` with the
   value you defined in the `PriorityClass`

   **_Note:_**

   1. This broker update will cause new broker `Pod`s to be created, and the
      existing broker `Pod`s to be deleted after the new broker `Pod`s are
      available.
   2. After this broker update, `Pod Priority` will stay the same if you
      re-start any of the broker `Pod`s. However, if you re-start (delete and
      let the controller re-create) the `Deployment`, you'll need to update
      corresponding `Deployment` with `PriorityClass` again.
