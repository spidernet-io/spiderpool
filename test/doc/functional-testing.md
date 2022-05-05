# e2e case for ippool

| case id | category  | description                                             | priority | status | other |
|---------|-----------|---------------------------------------------------------|----------|--------|-------|
| F00001  | ippool | assign ip to a pod for case ipv4, ipv6, dual-stack ip   | p2       | done   |       |
| F00002  | ippool | assign ip to 100 pod for case ipv4, ipv6, dual-stack ip   | p2       |       |       |
| F00003  | api | post /ipam/ip   | p2       |       |       |
| F00004  | api | get /ipam/ip   | p2       |       |       |
| F00005  | api | patch /ipam/ip   | p2       |       |       |
| F00006  | api | delete /ipam/ip   | p2       |       |       |
| F00007  | metric | check ip number    | p2       |       |       |
| F00008  | metric | check ip pool number   | p2       |       |       |
| F00009  | log | check spider-agent log   | p2       |       |       |
| F000010  | log | check spider-server log   | p2       |       |       |
| F000011  | log | check plugin log   | p2       |       |       |


<table class="relative-table wrapped confluenceTable"><colgroup><col style="width: 6.90782%;" /><col style="width: 8.29046%;" /><col style="width: 24.2051%;" /><col style="width: 37.567%;" /><col style="width: 13.4173%;" /><col style="width: 4.93391%;" /><col /><col style="width: 4.67846%;" /></colgroup><tbody><tr><th class="confluenceTh" style="width: 4.16667%;">Num</th><th class="confluenceTh" style="width: 5.92105%;">分类</th><th class="confluenceTh" style="width: 17.617%;">测试点</th><th class="confluenceTh" style="width: 31.7251%;">测试用例</th><th class="confluenceTh" style="width: 11.3304%;">选为e2e</th><th class="confluenceTh" style="width: 4.16667%;">优先级</th><th class="confluenceTh" colspan="1"><br /></th><th class="confluenceTh" style="width: 3.87427%;">备注</th></tr><tr><td class="confluenceTd" colspan="1">1</td><td class="confluenceTd" colspan="1">ipam-ip</td><td class="confluenceTd" colspan="1">单pod分配ip</td><td class="confluenceTd" colspan="1"><p>1 集群，ipam组件安装成功</p><p>2 ip pool创建成功</p><p>3 单个pod分配ip</p><p>期待：能够分配成功，pod running</p><p>4 进行联通性测试（跨主机ping pod）</p><p>期待：能够ping通</p></td><td class="confluenceTd" colspan="1">是</td><td class="confluenceTd" colspan="1"><br /></td><td class="confluenceTd" colspan="1"><br /></td><td class="confluenceTd" colspan="1"><br /></td></tr></tbody></table>

