# 测试文档：IaaS Provider 预热 IP 池支持 (006-iaas-prewarm-pool)

**分支**: `006-iaas-prewarm-pool`
**关联设计文档**: `docs/develop/proposal-iaas-ip-provider.md`、`specs/006-iaas-prewarm-pool/{spec,plan,data-model,quickstart}.md`
**状态**: 代码已完成并本地提交（commit `2da172dd8`），**尚未部署到测试集群，尚未开始实际执行测试**
**最后更新**: 2026-08-06

---

## 1. 背景

本特性为 Spiderpool 侧（开源部分）新增对 IaaS 供应商预热（prewarm）IP 池的支持，
配合外部/私有的 `iaas-network-provider` 组件使用，两者的分工是：

- **iaas-network-provider（provider 组件，另行部署）**：调用云厂商 API（本环境用华为云
  mock）提前创建/绑定弹性网卡子 IP，并把结果写入 `SpiderIPPool.status.iaasIPs` 这个
  per-IP 台账（ledger）。
- **Spiderpool（本次改动）**：
  1. 新增两个 `SpiderIPPool` 注解：`ipam.spidernet.io/iaas-pool`（标记该池由 IaaS 管理）、
     `ipam.spidernet.io/pair-pool`（声明其双栈配对池），并通过 mutating webhook 把
     `iaas-pool` 注解同步为同名 label。
  2. validating webhook 新增配对校验规则（禁止自引用、禁止同 IP 版本配对、
     v4 静态容量 <= v6 静态容量、两个配对池的 `nodeName`/`podAffinity` 必须一致）。
  3. `SpiderIPPool.status` 新增 `iaasIPs` 台账字段与 `conditions`，由 provider 组件写入，
     Spiderpool IPAM 只读消费。
  4. IPAM 两处行为变化：
     - Pod 池候选解析阶段，若选中的池带 `pair-pool` 且 Pod 未显式请求对侧地址族，
       自动补全配对池（`pkg/ipam/pool_selections.go`）。
     - `AllocateIP`（`pkg/ippoolmanager/ippool_manager.go`）在池台账已populate 时，
       按台账做“ready 且未占用”过滤，并对配对池做原子的成对分配；没有台账数据的池
       完全走原有分配逻辑，不受影响。

本次改动**不引入新 CRD**，全部是对现有 `SpiderIPPool` 的增量字段/注解扩展，设计目标是
对未使用新注解的存量池零行为影响。

## 2. 测试集群连接方式

- 集群：两节点 Kubernetes（`10-20-1-50` 为 control-plane，`10-20-1-60` 为 worker），
  两节点均为 x86_64 / Ubuntu 24.04，containerd 运行时，K8s v1.35.0。
- SSH 连接：`ssh -p 2022 root@10.20.1.50`（**注意端口是 2022，不是默认 22**）；
  `10.20.1.60` 同理通过 `ssh -p 2022 root@10.20.1.60`（假定同一密钥/端口，需在实际操作时确认）。
- 该节点上已配置 `kubectl`、`helm`，可直接管理集群；`ghcr.io` 拉取凭据已在
  `/root/.docker/config.json` 中配置（用户 `cyclinder`）。
- 集群中已有命名空间 `spiderpool`（当前部署的是 Helm chart `spiderpool-1.0.5`，
  agent/controller 镜像为历史 commit 构建，**不是本次改动的代码**）以及
  `iaas-network-provider-system`（其中已跑着 `iaas-network-provider` 与
  `huawei-mockserver`，用于模拟华为云 API，供 provider 组件测试用）。
- 集群中已存在较多不相关的 `SpiderIPPool`（`abc`、`macvlan1.enp-pool` 等），
  测试时新建资源需使用不冲突的命名（建议加前缀，如 `iaas-t-*`），并在测试结束后清理。

## 3. 待部署内容（尚未执行）

1. 用本分支代码构建新的 `spiderpool-agent` / `spiderpool-controller` 镜像
   （`make build_image` 或国内网络用 `make build_docker_image E2E_CHINA_IMAGE_REGISTRY=true`），
   镜像 tag 使用当前 commit（`git show -s --format='%H'`）。
2. 将新镜像传输/推送到集群可拉取的位置（当前 `ghcr.io` 凭据可用，或改用
   `docker save` + `ctr/nerdctl` 导入两个节点的 containerd，避免依赖外网）。
3. 更新 CRD：`kubectl apply -f charts/spiderpool/crds/spiderpool.spidernet.io_spiderippools.yaml`
   （已包含本次新增的 `iaasIPs`/`conditions` 字段与注解校验相关 schema）。
4. 用 Helm 升级 `spiderpool` release 的镜像 tag 到新构建版本
   （`helm upgrade spiderpool ... --set agent.image.tag=... --set controller.image.tag=...`，
   具体 values 路径待在部署时对照 `charts/spiderpool/values.yaml` 确认）。
5. provider 组件（`iaas-network-provider`）后续由用户另行部署/更新，用于写入真实的
   `status.iaasIPs` 台账；在 provider 就绪前，可用 `kubectl patch --subresource=status`
   手工模拟台账数据进行 Spiderpool 侧独立验证（见 `quickstart.md` 第 3 步）。

> 本次会话按用户要求，**只完成到写测试文档为止，不执行上述部署步骤**。

## 4. 测试用例列表与进度

状态说明：`未开始` / `进行中` / `通过` / `失败` / `阻塞`。

| # | 用例 | 对应需求 | 验证方式 | 状态 |
|---|------|---------|---------|------|
| 0 | 单元测试：`pkg/ipam`、`pkg/ippoolmanager` 全量通过 | 代码正确性基线 | `go test -count=1 ./pkg/ipam/... ./pkg/ippoolmanager/...` | **通过**（本地已跑，2026-08-06） |
| 0.1 | 全仓库编译无误 | 基线 | `go build ./...` | **通过**（本地已跑，2026-08-06） |
| 1 | 创建一对 `iaas-pool` + `pair-pool` 的 v4/v6 池，注解正确同步为 label | FR-001 | `kubectl apply` 建池 + `kubectl get sp <name> -o jsonpath='{.metadata.labels}'` 校验含 `ipam.spidernet.io/iaas-pool=true` | 未开始 |
| 2 | 配对校验：拒绝自引用 (`pair-pool` 指向自身) | FR-003 | `kubectl annotate sp <name> ipam.spidernet.io/pair-pool=<same-name> --overwrite`，期望被 admission 拒绝 | 未开始 |
| 3 | 配对校验：拒绝同 IP 版本配对（两个池都是 v4 或都是 v6） | FR-003 | 构造两个同版本池互相 pair，期望拒绝 | 未开始 |
| 4 | 配对校验：v4 静态容量 > v6 静态容量时拒绝 | FR-003 | 扩大 v4 池 `spec.ips` 使其地址数超过配对 v6 池，期望 `kubectl patch` 被拒绝 | 未开始 |
| 5 | 配对校验：`nodeName`/`podAffinity` 不一致时拒绝 | FR-003 | 使两个配对池的 `nodeName` 或 `podAffinity.matchLabels` 不同，期望拒绝 | 未开始 |
| 6 | 配对校验：引用尚不存在的池时不应拒绝 | FR-003 | `pair-pool` 指向一个还未创建的池名，创建应成功 | 未开始 |
| 7 | 模拟写入 `status.iaasIPs` 台账（含 ready/not-ready 条目） | FR-008 | `kubectl patch sp <name> --subresource=status --type=merge -p '...'`（见 quickstart.md 第 3 步） | 未开始 |
| 8 | 单栈 Pod 从带 `pair-pool` 的池请求 IP：自动补全配对池候选，但不强制分配对侧地址 | FR-004, FR-005 | 创建仅声明 v4 池的 Pod，检查其只获得 v4 地址，v6 侧地址仍未分配 | 未开始 |
| 9 | 台账过滤：`NotReady`/`Releasing` 条目不可被选中 | FR-009 | 构造包含 `NotReady` 条目的台账，创建 Pod，确认从不分配该条目地址 | 未开始 |
| 10 | 双栈 Pod 原子成对分配：v4/v6 必须来自同一台账条目，不可交叉 | FR-010 | 创建双栈 Pod，检查 `status.allocatedIPs`/Pod 注解中 v4、v6 地址确实成对（同一条目） | 未开始 |
| 11 | 台账已耗尽（无 ready 且未占用条目）时，走标准“无可用 IP”失败并允许多池 fallback | FR-013 | 耗尽台账后创建 Pod（声明多个候选池），确认该池报无可用 IP 但整体调度可 fallback 到其他池而非卡死 | 未开始 |
| 12 | 台账驱动的分配跳过原有同步云 API 调用路径 | FR-012, FR-015 | 结合 provider/mock 观察：从 ready 台账条目分配时不应触发实时云 API 调用（可通过 mockserver 请求日志或 provider 日志验证无新调用） | 未开始（依赖 provider 组件部署） |
| 13 | 回归：不带 `iaas-pool` 注解的存量池行为完全不变 | FR-006, FR-011 | 对集群中已有的普通池（如 `abc`）执行常规分配，确认无 label 被添加、无配对校验触发、分配结果与升级前一致 | 未开始 |
| 14 | Webhook/Controller 升级后原有 e2e / 存量工作负载不受影响的冒烟检查 | 兼容性 | 观察 `spiderpool-agent`/`spiderpool-controller` Pod 升级后状态、日志无异常，抽查若干已有 Pod 网络正常 | 未开始 |
| 15 | 与 provider 组件联调：provider 真实写入台账 → Spiderpool 分配 → 云侧（mock）状态一致性 | 端到端 | provider 组件就绪后，创建声明 podAffinity 匹配的工作负载，观察 provider 日志/mockserver 记录的绑定操作与 Spiderpool 分配结果一致 | 未开始（依赖 provider 组件部署，用户后续提供） |

## 5. 环境相关注意事项

- 集群通过公网/受限网络访问 `ghcr.io` 等镜像仓库可能不稳定，必要时改用
  `E2E_CHINA_IMAGE_REGISTRY=true` 走 daocloud 镜像，或直接 `docker save`/`scp` +
  containerd 本地导入两节点。
- 集群上已有一个较旧的 `spiderpool` release（chart 1.0.5, agent 镜像为
  `main-fix-dual-port-2` 分支构建），本次测试需要的是升级/替换为本分支代码构建的
  agent/controller 镜像，升级前建议记录当前镜像 digest 以便回滚。
- 有若干 `spiderpool-init-*` Pod 处于 `Unknown` 状态（历史遗留），与本次测试无关，
  不需处理，但注意不要误判为本次改动引入的问题。

## 6. 后续步骤

1. 用户确认部署方式（是否直接在 `10.20.1.50`/`10.20.1.60` 上 `docker build` + 本地
   containerd 导入，还是推送到可访问的镜像仓库）。
2. 按第 3 节步骤实际部署新镜像 + CRD + Helm 升级。
3. 按第 4 节用例表逐项执行，并在本文件中更新每项的“状态”列（通过/失败/阻塞及原因）。
4. provider 组件部署完成后，补充执行依赖它的用例（#12、#15）。
