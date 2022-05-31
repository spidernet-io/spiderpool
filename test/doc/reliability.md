# e2e case for reliability

| case id  | title                             |priority |smoke| status | other |
|---------|-----------------------------------------|----------|----------|--------|-------|
| R00001  | create a pod while deleting and rebooting the spiderpool controller manager service.  After the service returns to be stable, pod will run in normal |p3   |    | NA     |       |
| R00002  | create a pod while deleting and rebooting the ETCD. After the ETCD returns to be stable, pod will run in normal |p3||done||
| R00003  | create a pod while deleting and rebooting the api-server.  After the api-server returns to be stable, pod will run in normal|p3||done||
| R00004  | create a pod while deleting and rebooting the spiderpool agent.  After the spiderpool agent returns to be stable, pod will run in normal|p4||NA||
| R00005  | create a pod while deleting and rebooting the COREDNS.  After the COREDNS returns to be stable, pod will run in normal|p4||NA||
| R00006  | power off one node for more than five minutes and check if pod could run in another node|p2||NA||
