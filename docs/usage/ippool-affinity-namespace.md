# SpiderIPPool namespace affinity

*Spiderpool supports multiple ways to select ippool. Pod will select a specific ippool to allocate IP according to the corresponding rules that with different priorities. Meanwhile, ippool can use selector to filter its user.*

## Setup Spiderpool

If you have not set up Spiderpool yet, follow the guide [Quick Installation](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/install.md) for instructions on how to install and simply configure Spiderpool.

## Get started

## Clean up

Let's clean the relevant resources so that we can run this tutorial again.

```bash
kubectl delete -f https://github.com/spidernet-io/spiderpool/tree/main/docs/example/ippool-affinity-namespace --ignore-not-found=true
```
