# e2e case for performance

| case id | category    | title                             | check point |priority | status | other |
|---------|-------------|-----------------------------------------|-------------|----------|--------|-------|
| P00001  | performance | time cost for assigning ip to 1000 pods | |p3       | NA     |       |
| P00002  | performance | through controller deployment CRUD，check time cost for assigning ipv4、ipv6 to 500 pods| |p3|NA||
| P00003  | performance | through controller statefulSet CRUD，check time cost for assigning ipv4、ipv6 to 500 pods| |p3|NA| considering its performance and other controllers have a certain difference, So separate use case coverage|
