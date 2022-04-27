# spiderpoolctl

CLI for debug

## spiderpoolctl gc

trigger GC request to spiderpool-controller

```
    --address string         [optional] address for spider-controller (default to service address)
```

## spiderpoolctl ip show

show pod who is taking this ip

### Options

```
    --ip string     [required] ip
```

## spiderpoolctl ip release

try to release ip

### Options

```
    --ip string     [optional] ip
    --force         [optional] force release ip
```

## spiderpoolctl ip set

set ip to be taken by a pod , this will update ippool and workloadendpoint resource

### Options

```
    --ip string     [required] ip
    --pod string                [required] pod name
    --namespace string          [required] pod namespace
    --containerid string        [required] pod container id
    --node string               [required] the node name who the pod locates
    --interface string          [required] pod interface who taking effect the ip
```
