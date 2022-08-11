# spiderpoolctl

This page describes CLI usage of spiderpoolctl for debug.

## spiderpoolctl gc

Trigger the GC request to spiderpool-controller.

```
    --address string         [optional] address for spider-controller (default to service address)
```

## spiderpoolctl ip show

Show a pod that is taking this IP.

### Options

```
    --ip string     [required] ip
```

## spiderpoolctl ip release

Try to release an IP.

### Options

```
    --ip string     [optional] ip
    --force         [optional] force release ip
```

## spiderpoolctl ip set

Set IP to be taken by a pod. This will update ippool and workload endpoint resource.

### Options

```
    --ip string     [required] ip
    --pod string                [required] pod name
    --namespace string          [required] pod namespace
    --containerid string        [required] pod container id
    --node string               [required] the node name who the pod locates
    --interface string          [required] pod interface who taking effect the ip
```
