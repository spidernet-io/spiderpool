# IPv6 support

## Description

Spiderpool supports:

- **Dual stack**

    Each workload can get IPv4 and IPv6 addresses, and can communicate over IPv4 or IPv6.

- **IPv4 only**

    Each workload can acquire IPv4 addresses, and can communicate over IPv4.

- **IPv6 only**

    Each workload can acquire IPv6 addresses, and can communicate over IPv6.

## Get Started

### Set up Spiderpool

follow the guide [installation](./install/underlay/get-started-kind.md) to install Spiderpool.

### Create SpiderSubnet

Create a SpiderSubnet and allocate IP addresses from the IPPool.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/basic/custom-ipv4-subnet.yaml

kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/basic/custom-ipv6-subnet.yaml
```

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderSubnet
metadata:
  name: custom-ipv4-subnet
spec:
  subnet: 172.18.41.0/24
  ips:
    - 172.18.41.40-172.18.41.50
```

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderSubnet
metadata:
  name: custom-ipv6-subnet
spec:
  subnet: fd00:172:18::/64
  ips:
    - fd00:172:18::40-fd00:172:18::50

```

### Create Deployment By Subnet

create a Deployment whose Pods are setting the Pod annotation `ipam.spidernet.io/subnet` to  explicitly specify the subnet.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/basic/custom-dual-subnet-deploy.yaml
```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: custom-dual-subnet-deploy
spec:
  replicas: 3
  selector:
    matchLabels:
      app: custom-dual-subnet-deploy
  template:
    metadata:
      annotations:
        ipam.spidernet.io/subnet: |-
          {
            "ipv4": ["custom-ipv4-subnet"],"ipv6": ["custom-ipv6-subnet"]
          }
      labels:
        app: custom-dual-subnet-deploy
    spec:
      containers:
        - name: custom-dual-subnet-deploy
          image: busybox
          imagePullPolicy: IfNotPresent
          command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]

```

The Pods are running.

```bash
kubectl get pod -l app=custom-dual-subnet-deploy -owide
NAME                                         READY   STATUS    RESTARTS   AGE   IP             NODE                NOMINATED NODE   READINESS GATES
custom-dual-subnet-deploy-7fdbccfbb8-h5l4d   1/1     Running   0          33s   172.18.41.41   controller-node-1   <none>           <none>
custom-dual-subnet-deploy-7fdbccfbb8-rhdbd   1/1     Running   0          33s   172.18.41.42   controller-node-1   <none>           <none>
custom-dual-subnet-deploy-7fdbccfbb8-t6m5c   1/1     Running   0          33s   172.18.41.40   controller-node-1   <none>           <none>
```

View all IPs of Pods

```bash
kubectl get pod -l app=custom-dual-subnet-deploy  -o go-template='{{range .items}}{{.metadata.name}}: {{range .status.podIPs}}{{.}} {{end}}{{"\n"}}{{end}}'
custom-dual-subnet-deploy-7fdbccfbb8-h5l4d: map[ip:172.18.41.41] map[ip:fd00:172:18::42]
custom-dual-subnet-deploy-7fdbccfbb8-rhdbd: map[ip:172.18.41.42] map[ip:fd00:172:18::41]
custom-dual-subnet-deploy-7fdbccfbb8-t6m5c: map[ip:172.18.41.40] map[ip:fd00:172:18::40]
```

### Create Deployment By IPPool

1. Create IPPool

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/basic/custom-ipv4-ippool.yaml
    
    kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/basic/custom-ipv6-ippool.yaml
    ```

    ```yaml
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata:
      name: custom-ipv4-ippool
    spec:
      subnet: 172.18.41.0/24
      ips:
        - 172.18.41.40-172.18.41.50
    ```

    ```yaml
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata:
      name: custom-ipv6-ippool
    spec:
      subnet: fd00:172:18::/64
      ips:
        - fd00:172:18::40-fd00:172:18::50
    ```

2. Create Deployment
  
    create a Deployment whose Pods are setting the Pod annotation `ipam.spidernet.io/ippool` to  explicitly specify the pool.

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/basic/custom-dual-ippool-deploy.yaml
    ```

    ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: custom-dual-ippool-deploy
    spec:
      replicas: 3
      selector:
        matchLabels:
          app: custom-dual-ippool-deploy
      template:
        metadata:
          annotations:
            ipam.spidernet.io/ippool: |-
              {
                "ipv4": ["custom-ipv4-ippool"],"ipv6": ["custom-ipv6-ippool"]
              }
          labels:
            app: custom-dual-ippool-deploy
        spec:
          containers:
            - name: custom-dual-ippool-deploy
              image: busybox
              imagePullPolicy: IfNotPresent
              command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
    ```

    The Pods are running.

    ```bash
    kubectl get pod -owide -l app=custom-dual-ippool-deploy
    NAME                                        READY   STATUS    RESTARTS   AGE   IP             NODE                NOMINATED NODE   READINESS GATES
    custom-dual-ippool-deploy-9bb6696c4-6wjnl   1/1     Running   0          76s   172.18.41.42   controller-node-1   <none>           <none>
    custom-dual-ippool-deploy-9bb6696c4-8vtpf   1/1     Running   0          76s   172.18.41.45   controller-node-1   <none>           <none>
    custom-dual-ippool-deploy-9bb6696c4-zbknv   1/1     Running   0          76s   172.18.41.43   controller-node-1   <none>           <none>
    ```

    View all IPs of Pods

    ```bash
    kubectl get pod -l app=custom-dual-ippool-deploy  -o go-template='{{range .items}}{{.metadata.name}}: {{range .status.podIPs}}{{.}} {{end}}{{"\n"}}{{end}}'
    custom-dual-ippool-deploy-9bb6696c4-6wjnl: map[ip:172.18.41.42] map[ip:fd00:172:18::4d]
    custom-dual-ippool-deploy-9bb6696c4-8vtpf: map[ip:172.18.41.45] map[ip:fd00:172:18::4e]
    custom-dual-ippool-deploy-9bb6696c4-zbknv: map[ip:172.18.41.43] map[ip:fd00:172:18::46]
    ```

### Clean up

Clean the relevant resources so that you can run this tutorial again

   ```bash
   kubectl delete \
   -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/basic/custom-ipv4-subnet.yaml \
   -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/basic/custom-ipv6-subnet.yaml \
   -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/basic/custom-ipv4-ippool.yaml \
   -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/basic/custom-ipv6-ippool.yaml \
   -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/basic/custom-dual-ippool-deploy.yaml \
   --ignore-not-found=true
   ```
