# e2e case for reliability

| case id | category    | title                             | check point |priority | status | other |
|---------|-------------|-----------------------------------------|-------------|----------|--------|-------|
| R00001  | chaos | IP address allocation, spider controller manager service failure | |p3       | NA     |       |
| R00002  | chaos | IP address allocation ETCD failure ||p3|NA||
| R00003  | chaos | IP address allocation, spiderpool api-server service failure ||p3|NA||
| R00004  | chaos | IP address allocation, spiderpool ipam plugin service failure||p4|NA||
| R00005  | chaos | IP address allocation, spiderpool agent failure ||p4|NA||
| R00006  | chaos | IP address allocation, COREDNS failure  ||p4|NA||
