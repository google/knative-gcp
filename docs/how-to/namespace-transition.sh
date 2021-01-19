#!/usr/bin/env bash

# create the new namespace by copying the information from the old one
kubectl get namespace cloud-run-events -o yaml | \
  sed 's/  name: cloud-run-events/  name: events-system/g' | \
  kubectl create -f -
# setup identical authentication by copying the existing secrets from the old namespace
kubectl get secrets -n cloud-run-events -o yaml | \
  sed 's/  namespace: cloud-run-events/  namespace: events-system/g' | \
  kubectl create -f -
# copy the existing config maps to maintain existing configurations
kubectl get configmaps -n cloud-run-events --field-selector metadata.name!=kube-root-ca.crt -o yaml | \
  sed 's/  namespace: cloud-run-events/  namespace: events-system/g' | \
  kubectl create -f -

# remove the old webhook and controller to prevent them from interfering with the new ones
kubectl delete deployment webhook -n cloud-run-events
kubectl delete service webhook -n cloud-run-events
kubectl delete deployment controller -n cloud-run-events
kubectl delete service controller -n cloud-run-events
