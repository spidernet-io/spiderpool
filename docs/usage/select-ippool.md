# Select ippool

>Spiderpool supports multiple ways to select ippool. Pod will select a specific ippool to allocate IP according to the corresponding rules that with different priorities. Meanwhile, ippool can use selector to filter its user.

## Priority

Spiderpool supports the following ways to specify ippool for Pod:

1. Pod annotations (high priority): Specifies which ippool the current Pod should use to allocate IP, which overrides any other selection rules.

   - `ipam.spidernet.io/ippool`: For single interface case. Ensure that the interface field specified in Pod annotations is consistent with that in the CNI request.

     ```yaml
     ipam.spidernet.io/ippool: |-
       {
         "interface": "eth0",
         "ipv4pools": ["v4pool1"],
         "ipv6pools": ["v6pool1", "v6pool2"]
       }
     ```

   - `ipam.spidernet.io/ippools`: For multiple interface case. Note that it does not means that CNI will return the IP allocation results of multiple interface in one request (this will break the [CNI Specification](https://www.cni.dev/docs/spec/)). Spiderpool will allocate multiple IP in one request, but return them in several times. It is mainly used with [Spiderflat](TODO).

     ```yaml
     ipam.spidernet.io/ippools: |-
       [{
           "interface": "eth0",
           "ipv4pools": ["v4pool1"],
           "ipv6pools": ["v6pool1"],
           "defaultRoute": true,
        },{
           "interface": "eth1",
           "ipv4pools": ["v4pool2"],
           "ipv6pools": ["v6pool2"],
           "defaultRoute": false
       }]
     ```

   BTW, `ipam.spidernet.io/ippools` has precedence over `ipam.spidernet.io/ippool`.

2. Namespace annotations (medium priority): Specifies default ippools for current Namespace. Pods that do not have Pod IPAM annotations under this Namespace will all be allocated IP by these ippools.

   - `ipam.spidernet.io/defaultv4ippool`: Default IPv4 ippools at Namespace level.

     ```yaml
     ipam.spidernet.io/defaultv4ippool: ["v4pool1","v4pool2"]
     ```

   - `ipam.spidernet.io/defaultv6ippool`: Default IPv6 ippools at Namespace level.

     ```yaml
     ipam.spidernet.io/defaultv6ippool: ["v6pool1","v6pool2"]
     ```

3. Configmap (low priority)：Default ippools at cluster level. Pods that do not append any special ippool selection rules will try to allocate IP from these ippools. They are valid in whole cluster.

   ```yaml
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: spiderpool-conf
     namespace: kube-system
   data:
     clusterDefaultIPv4IPPool: ["v4pool1"]
     clusterDefaultIPv6IPPool: ["v6pool1"]
     ...
   ```

More detailed description of Spiderpool [annotation](https://spidernet-io.github.io/spiderpool/usage/annotation/) and [configuration](https://spidernet-io.github.io/spiderpool/usage/config/).

## Backup

Each ippool selection rule supports 'backup'. We can specify multiple ippools in the array to achieve this effect. Spiderpool will successively try to allocate IP in the order of the elements in the ippool array until the first allocation succeeds or all fail.

The following is an example of Pod annotation `ipam.spidernet.io/ippool`, the same is true for other ippool selection rules. We can create such a Pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: sample-backup
  annotations:   
    ipam.spidernet.io/ippool: |-
      {
        "interface": "eth0",
        "ipv4pools": ["default-v4-ippool", "backup-v4-ippool"]
      }
spec:
  containers:
  - name: sample-backup
    image: alpine
    imagePullPolicy: IfNotPresent
    command: ["/bin/bash", "-c", "trap : TERM INT; sleep infinity & wait"]
```

As we can see from the above, Pod `sample-backup` will attempt to allocate IP from ippools defined in field `ipv4pools`.

Unfortunately, ippool `default-v4-ippool` has run out of IP.

```bash
$ kubectl get spl
NAME                VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
default-v4-ippool   IPv4      172.18.0.0/16             5                    5                false
backup-v4-ippool    IPv4      172.18.0.0/16             1                    5                false
```

We will see Pod `sample-backup` successfully allocated IP from ippool `backup-v4-ippool`.

```bash
$ kubectl get swe sample-backup -n default
NAME            INTERFACE   IPV4POOL           IPV4              IPV6POOL   IPV6   NODE            CREATETION TIME
sample-backup   eth0        backup-v4-ippool   172.18.40.40/16                     spider-worker   1m33s
```

## Selectors

Ippool can also use `Node Selector`, `Namespace Selector` and `Pod Selector` to filter its users. It should be regarded as a filtering mechanism rather than a ippool selection rule (refer to chapter [Priority](#Priority) for ippool selection rules).

All selectors follow the syntax of [Kubernetes label selector](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/):

```yaml
selector:
  matchLabels:
    component: redis
  matchExpressions:
    - {key: tier, operator: In, values: [cache]}
    - {key: environment, operator: NotIn, values: [dev]}
```

We have such an ippool:

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: IPPool
metadata:
  name: default-v4-ippool
spec:
  disable: false
  ipVersion: IPv4
  subnet: 172.18.0.0/16
  ips:
  - 172.18.40.40-172.18.40.45
  vlan: 0
  namesapceSelector:
    matchExpressions:
      - {key: foo, operator: In, values: [bar]}
```

It means that only Pod under the Namespace with `foo: bar` label can use this ippool.

At the same time, `default-v4-ippool` is also the default IPv4 ippool of the cluster.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: spiderpool-conf
  namespace: kube-system
data:
  enableIPv4: true
  enableIPv6: false
  clusterDefaultIPv4IPPool: ["default-v4-ippool"]
  clusterDefaultIPv6IPPool: []
  ...
```

Try to create a Deployment in `default` Namespace.

```bash
$ kubectl create deployment my-deploy --image=nginx
```

Unfortunately, this Deployment will eventually fail to function properly because it is not allocated any IP, and not in a Namespace that matches `Namespace Selector`.

Do not give too harsh selectors to cluster default ippools , which will not make life better :)
