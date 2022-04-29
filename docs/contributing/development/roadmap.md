# spiderpool road map

## feature

---

* [ ] (alpha) a pod use multiple ippools, in case that an ippool is out of use

* [ ] (alpha) cooperate with macvlan

* [ ] (alpha) support pod / namespace annotations to customize ip

* [ ] (alpha) dual stack support. For ipv4-only, ipv6-only, dual-stack cluster,
  * [ ] assign ipv4/ipv6 ip to pod
  * [ ] all component could service on ipv4/ipv6 ip

* [ ] (alpha) ippool selector
  * [ ] select namespace. Each namespace could occupy non-shared ippool
  * [ ] select pod. For deployment and statefulset case, pod could occupy some ip range
  * [ ] select node. For different zone, pod have specified ip range
  
---

* [ ] (beta) cooperate with multus. multus may call multiple CNI to assign different interface and assign ip

* [ ] (beta) fixed ip for application, especially for statefulset

* [ ] (beta) health check

* [ ] (beta) CLI for debug

* [ ] (beta) retrieve leaked ip
  * [ ] for graceful timeout of terminating state
  * [ ] for finished job
  * [ ] trigger by hand or automatically
  * [ ] trigger by pod event and happen at interval

* [ ] (beta) DCE5 integration

---

* [ ] (GA) metrics

* [ ] (GA) reserved ip

* [ ] (GA) administrator edit ip safely, preventing from race with IPAM CNI, and avoid ip conflicting between ippools

* [ ] (GA) good performance
  * [ ] take xxx second at most, to assign a ip
  * [ ] take xxx second at most, to assign 1000 ip

* [ ] (GA) good reliability

* [ ] (GA) cooperate with spiderflat

---

* [ ] Unit-Test
  * [ ] (alpha) 40% coverage at least
  * [ ] (beta) 70% coverage at least
  * [ ] (GA) 90% coverage at least

* [ ] e2e test
  * [ ] (alpha) 30% coverage for test case of alpha feature
  * [ ] (beta) 80% coverage for test case of  beta/alpha feature
  * [ ] (GA) 100% coverage for test case of all feature
  * [ ] (GA) chaos test case, performance test case

* [ ] All CICD pipeline. nightly ci, auto release chart/image/release, code lint, doc lint, unitest, e2e test
  * [X] (alpha) 80% CICD pipeline
  * [ ] (GA) 100% CICD pipeline

* [ ] documentation
  * [ ] (alpha) architecture, contributing
  * [ ] (beta) concept, get started, configuration
  * [ ] (GA) command reference

---

## goal of April

* [x] 第一个 Helm Release，可以一键部署（可持续 CICD）

* [x] 输出 Road Map，完成 6个月以上的规划

* [x] Unit-Test 覆盖率 10 %

* [x] 第一个 自动化的 e2e 测试（每天晚上都要自动跑）

* [x] OpenSSF(开源最佳实践) 完成度 10%

* [x] GitHub (Star) 100

* [x] webhook 精通及分享

* [ ] go builder 的 SDK 生成，所有的 client 都能工作，生成 CRD yaml。 能做到 CI 自动化 SDK 校验

* [x] 完成 openapi 接口和 SDK ，验证都能工作。能做到 CI 自动化 SDK 校验

* [x] 完成 spiderpool ipam plugin ，agent、 controller进程能够跑 ，确保 helm apply 能够部署（不要求能跑业务）

* [x] 搭建 e2e 框架

* [x] 精读 golang、ginkgo、开源项目 e2e， 以macvlan + whereabout 完成第一个e2e测试

---

## goal of May

* [ ] 完成 IPAM plug in

* [ ] 完成 Agent 主要代码，能分配出 ipv4/ipv6 地址

* [ ] 以 macvlan + whereabout 为方案，完成 50% alpha feature 的 E2E 用例

* [ ] 完成 100% alpha doc

* [ ] OpenSSF(开源最佳实践) 完成度 100%

* [ ] 5 个外部反馈用户，10个 ISSUE ?????

* [ ] 3个外部贡献者 ????

* [ ] GitHub (Star) 200 ???

* [ ] Unit-Test 覆盖率 30%

* [ ] 开放的 独立网站 和 下载，反馈。  ????

* [ ] 开发的 独立网站 QuickStart， 下载，反馈。 ???

* [ ] Spider Pool e2e 测试用例设计 (包括 自动升级，性能，可靠性，老化) 结合基线 和过往的 L3 事故。(评审会之前，开会review)

* [ ] 充分的 e2e 测试 设计(包括 自动升级，性能，可靠性，老化) 结合基线 和过往的 L3 事故  ??

---

## goal of June

* [ ] 完成所有满足 alpha feature 的业务代码

* [ ] 以 macvlan + spiderpool 为方案，完成 100% alpha feature 的 E2E 用例

* [ ] 收集 10个以上 alpha 用户的反馈，并解决其反馈的问题(刚性)

* [ ] Unit-Test 覆盖率 80%

* [ ] 性能测试，可靠性测试，???

* [ ]  充分的灾难场景测试。和 性能基准测试 ??

* [ ] 兼容性测试验证（涉及的开源软件：第五代产品开源项目 和各个公有云环境）

* [ ] 审视“L3事故”，确认不会出现之前发生过的事故

* [ ] 灾难恢复演练 ?? beta

* [ ] 完成 和 KubeGrid 的 会师 ?? beta

* [ ] 解决所有重大bug

* [ ] 合作伙伴 1 个，反馈用户 1个

---

## goal of July

* [ ] 完成所有满足 beta feature 的代码

* [ ] 完成 100% beta doc

---

## goal of August

* [ ] finish most of GA feature , wait for spiderflat ready to debug

* [ ] 收集 10 个以上 beta 用户的反馈，并解决其反馈的问题(刚性)

* [ ] 将所有已知 BUG 解决

* [ ] OpenSSF(开源最佳实践) 完成度 100 %

---

## goal of September

* [ ] start spiderflat
