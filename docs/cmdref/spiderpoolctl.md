# spiderpoolctl

CLI for debug

## spiderpoolclt gc

trigger GC request to spiderpool-controller

```
    --address string         [optional] address for spider-controller (default to service address)
```

## spiderpoolclt ip show

show pod who is taking this ip

### Options

```
    --ip string     [required] ip
```

## spiderpoolclt ip release

try to release ip

### Options

```
    --ip string     [required] ip
    --force         [optional] force release ip
```

## spiderpoolclt ip set

set ip to be taken by a pod , this will udpate ippool and workloadendpoint resource

### Options

```
    --ip string     [required] ip
    --pod string                [required] pod name
    --namespace string          [required] pod namespace
    --containerid string        [required] pod container id
    --node string               [required] the node name who the pod locates
    --interface string          [required] pod interface who taking effect the ip
```
