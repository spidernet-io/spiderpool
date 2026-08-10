# 测试文档：IaaS Provider 预热 IP 池支持 (006-iaas-prewarm-pool)

**分支**: `006-iaas-prewarm-pool`
**关联设计文档**: `docs/develop/proposal-iaas-ip-provider.md`、`specs/006-iaas-prewarm-pool/{spec,plan,data-model,quickstart}.md`
**状态**: 代码已完成并本地提交，**Spiderpool 侧（agent/controller 新镜像 + CRD）已部署到测试集群**，
provider 组件（`iaas-network-provider`）尚未适配本特性，依赖它的用例暂缓
**最后更新**: 2026-08-10（部署 commit `b2cabef9e4d3e5459435c4eb6031ccd57c6cd889`）

> **2026-08-10 重新部署记录**：台账 API 重构（`status.iaasIPs` + Phase 字段拆分为
> `status.iaasReadyIPs`（在列即 ready，无 Phase）与 `status.iaasFailedIPs`（仅地址
> 标识、纯排除性），配对池台账只存在于主池/v4 池）后，按第 3 节同样流程重新构建并
> 部署：增量 git bundle 同步代码 → `make build_image E2E_CHINA_IMAGE_REGISTRY=true`
> 重新构建（基础镜像已缓存，仅约 4 分钟）→ 两节点 `ctr -n k8s.io images import` →
> `kubectl apply` 更新 CRD（已确认存量池无旧 `iaasIPs` 数据，无迁移问题；新 schema
> 含 `iaasReadyIPs`/`iaasFailedIPs`）→ `kubectl set image` 滚动更新。两节点 agent
> 2/2 Running、controller 1/1 Running，0 重启，日志正常。本地 `go build ./...` 与
> `go test ./pkg/ipam/... ./pkg/ippoolmanager/...` 在部署前均已通过。
> 注意：文中涉及旧字段 `status.iaasIPs`/`phase` 的用例描述需按新 API 理解
>（Ready 条目 = 出现在 `iaasReadyIPs` 中；NotReady/失败条目 = 出现在 `iaasFailedIPs`
> 或不在任何台账中）。

---

## 1. 背景

本特性为 Spiderpool 侧（开源部分）新增对 IaaS 供应商预热（prewarm）IP 池的支持，
配合外部/私有的 `iaas-network-provider` 组件使用，两者的分工是：

- **iaas-network-provider（provider 组件，另行部署）**：调用云厂商 API（本环境用华为云
  mock）提前创建/绑定弹性网卡子 IP，并把预热成功的地址写入
  `SpiderIPPool.status.iaasReadyIPs`（含 MAC/VLAN），预热失败/尚未就绪的地址写入
  `SpiderIPPool.status.iaasFailedIPs`（仅地址信息）。
- **Spiderpool（本次改动）**：
  1. 新增两个 `SpiderIPPool` 注解：`ipam.spidernet.io/iaas-pool`（标记该池由 IaaS 管理，
     即该池 `spec.ips` 中的地址需要 provider 预热确认后才可分配）、
     `ipam.spidernet.io/pair-pool`（声明其双栈配对池），并通过 mutating webhook 把
     `iaas-pool` 注解同步为同名 label。
  2. validating webhook 新增配对校验规则（禁止自引用、禁止同 IP 版本配对、
     v4 静态容量 <= v6 静态容量、两个配对池的 `nodeName`/`podAffinity` 必须一致）。
  3. `SpiderIPPool.status` 新增 `iaasReadyIPs`（已预热）/`iaasFailedIPs`（预热失败，
     仅地址）两个台账字段与 `conditions`，由 provider 组件写入，Spiderpool IPAM 只读消费；
     不再有单条目的 `Phase` 状态机——是否就绪由"存在于 `iaasReadyIPs`"这一事实本身决定。
  4. IPAM 两处行为变化：
     - Pod 池候选解析阶段，若选中的池带 `pair-pool` 且 Pod 未显式请求对侧地址族，
       自动补全配对池（`pkg/ipam/pool_selections.go`）。
     - `AllocateIP`（`pkg/ippoolmanager/ippool_manager.go`）对带 `iaas-pool` 标签的池，
       仍按 `spec.ips` 正常计算可用地址候选集，再与 `status.iaasReadyIPs` 求交集，
       按台账做“ready 且未占用”过滤，并对配对池做原子的成对分配；没有台账数据的池
       完全走原有分配逻辑，不受影响。

本次改动**不引入新 CRD**，全部是对现有 `SpiderIPPool` 的增量字段/注解扩展，设计目标是
对未使用新注解的存量池零行为影响。

## 2. 测试集群连接方式

- 集群：两节点 Kubernetes（`10-20-1-50` 为 control-plane，`10-20-1-60` 为 worker），
  两节点均为 x86_64 / Ubuntu 24.04，containerd 运行时，K8s v1.35.0。
- SSH 连接：`ssh -p 2022 root@10.20.1.50` 和 `ssh -p 2022 root@10.20.1.60`
  （**两个节点端口都是 2022，不是默认 22**，已实测验证可连通）。
- 该节点上已配置 `kubectl`、`helm`，可直接管理集群；`ghcr.io` 拉取凭据已在
  `/root/.docker/config.json` 中配置（用户 `cyclinder`）。**节点上没有预装 Go**
  （构建镜像走 Docker 多阶段构建，不依赖宿主机 Go 环境，无影响）。
- **重要**：节点容器运行时是**独立的 containerd（`k8s.io` 命名空间）**，与
  `docker build`/`docker load` 使用的 Docker daemon 镜像存储是**分离**的——
  `docker load` 加载的镜像 kubelet/crictl 看不到，必须额外执行
  `ctr -n k8s.io images import <tar>` 才能让新镜像对 Kubernetes 可用（已在本次
  部署中验证并踩坑修正，见第 3 节步骤 4）。
- 集群中已有命名空间 `spiderpool`（当前部署的是 Helm chart `spiderpool-1.0.5`，
  agent/controller 镜像为历史 commit 构建，**不是本次改动的代码**）以及
  `iaas-network-provider-system`（其中已跑着 `iaas-network-provider` 与
  `huawei-mockserver`，用于模拟华为云 API，供 provider 组件测试用）。
- 集群中已存在较多不相关的 `SpiderIPPool`（`abc`、`macvlan1.enp-pool` 等），
  测试时新建资源需使用不冲突的命名（建议加前缀，如 `iaas-t-*`），并在测试结束后清理。

## 3. 部署记录（已完成，2026-08-06）

> 根据用户明确要求，本次部署**未使用 `helm upgrade`**，而是采用“直接 apply CRD +
> 核对 ConfigMap（对应 `values.yaml`）+ 直接替换 Deployment/DaemonSet 镜像”的手工
> 方式，对已运行的集群做最小侵入、可控的变更。以下步骤均已实际执行并验证成功：

1. **上传代码到 10.20.1.50**（已完成）：本地用 `git bundle` 打包分支
   `006-iaas-prewarm-pool`（含 commit `2da172dd8`/`b51e1b89f`/`6a29ebf2b` 等），
   `scp -P 2022` 传输后在 `10.20.1.50:/root/spiderpool-build` 用
   `git clone <bundle> && git checkout 006-iaas-prewarm-pool` 展开，与该机器上
   已存在的、带有未提交改动的 `/home/cyclinder/spiderpool` 工作区完全隔离，避免互相干扰。
2. **在 10.20.1.50 上本地构建镜像**（已完成）：
   `make build_image E2E_CHINA_IMAGE_REGISTRY=true`（buildx，自动使用
   `docker.m.daocloud.io/library/golang:1.25.7` 与
   `ghcr.m.daocloud.io/spidernet-io/spiderpool/spiderpool-base:*` 国内镜像源）。
   耗时约 1 小时（主要卡在国内镜像源带宽约 30-40KB/s 拉取 golang 基础镜像层，
   非构建本身问题）。构建产物：
   - `ghcr.io/spidernet-io/spiderpool/spiderpool-agent:6a29ebf2b55945b835750bb877512930e87b702c`（298MB）
   - `ghcr.io/spidernet-io/spiderpool/spiderpool-controller:6a29ebf2b55945b835750bb877512930e87b702c`（213MB）
   - `go build ./...`、`go test -count=1 ./pkg/ipam/... ./pkg/ippoolmanager/...` 本地均已提前验证通过。
3. **同步镜像到 worker 节点 10.20.1.60**（已完成）：
   `docker save <agent> <controller> -o spiderpool-images-6a29ebf2b.tar`，
   经本地沙箱中转 `scp -P 2022` 传输到 `.60`（同一局域网，约 5 秒传完 518MB）。
4. **导入 containerd（关键步骤，两节点均执行）**（已完成）：
   `ctr -n k8s.io images import spiderpool-images-6a29ebf2b.tar`——
   仅 `docker load` 是不够的，必须此步骤 kubelet 才能识别新镜像；已用
   `crictl images | grep 6a29ebf2b` 在两节点分别确认可见。
   传输完成后已清理两节点及本地沙箱上的临时 tar 包。
5. **直接更新 CRD**（已完成）：
   `kubectl apply -f charts/spiderpool/crds/spiderpool.spidernet.io_spiderippools.yaml`
   ——已用 `kubectl get crd ... -o jsonpath` 确认 `status.iaasReadyIPs`/`status.iaasFailedIPs`/`status.conditions`
   schema 已生效，且 diff 核实这是纯新增字段（无删除/重命名），存量池
   （如 `abc`）不受影响。
6. **ConfigMap 核对**（已完成，无需修改）：
   diff 确认本次改动未涉及 `charts/spiderpool/values.yaml` 或任何 chart
   template，因此 `spiderpool-conf` 等 ConfigMap **无需改动**。
7. **直接替换 Deployment/DaemonSet 镜像**（已完成）：
   - `spiderpool-controller`（Deployment）：其容器 `imagePullPolicy` 原为
     `Always`——由于新镜像只存在于本地，未推送到 `ghcr.io`，**必须先 patch 为
     `IfNotPresent`**，否则会触发联网拉取失败（`ImagePullBackOff`）。已执行：
     ```
     kubectl patch deploy spiderpool-controller -n spiderpool --type=json \
       -p='[{"op":"replace","path":"/spec/template/spec/containers/0/imagePullPolicy","value":"IfNotPresent"}]'
     kubectl set image deploy/spiderpool-controller -n spiderpool \
       spiderpool-controller=ghcr.io/spidernet-io/spiderpool/spiderpool-controller:6a29ebf2b55945b835750bb877512930e87b702c
     ```
   - `spiderpool-agent`（DaemonSet，含 `spiderpool-agent` 与 `multus-cni` 两个
     容器，`imagePullPolicy` 已是 `IfNotPresent`，无需 patch）：只替换
     `spiderpool-agent` 容器，不动 `multus-cni`：
     ```
     kubectl set image ds/spiderpool-agent -n spiderpool \
       spiderpool-agent=ghcr.io/spidernet-io/spiderpool/spiderpool-agent:6a29ebf2b55945b835750bb877512930e87b702c
     ```
   - `kubectl rollout status` 确认两者均 rollout 成功，两节点 agent Pod
     均 `2/2 Running`、controller `1/1 Running`，0 次重启；日志显示新代码路径
     （如 `ipam/iaas.go` 的 prewarm mac 缓存逻辑）已生效运行，webhook
     mutating/validating 正常工作。观察到的一条 `macvlan1.enp-pool` subnet
     校验 ERROR 日志是**历史遗留问题**（该池 `spec.ips` 本就超出其 Subnet
     范围），与本次改动无关。
8. provider 组件（`iaas-network-provider`）**尚未适配/部署本特性所需的新逻辑**
   （集群里已有的 `iaas-network-provider`/`huawei-mockserver` 是旧版本，不写入
   本特性的 `status.iaasReadyIPs`/`status.iaasFailedIPs` 台账字段）；在其就绪前，可用
   `kubectl patch --subresource=status` 手工模拟台账数据进行 Spiderpool 侧
   独立验证（见 `quickstart.md` 第 3 步），对应第 4 节用例 #1-#11、#13、#14
   均可在无 provider 情况下执行。
## 4. 测试用例列表与进度

状态说明：`未开始` / `进行中` / `通过` / `失败` / `阻塞`。

| # | 用例 | 对应需求 | 验证方式 | 状态 |
|---|------|---------|---------|------|
| 0 | 单元测试：`pkg/ipam`、`pkg/ippoolmanager` 全量通过 | 代码正确性基线 | `go test -count=1 ./pkg/ipam/... ./pkg/ippoolmanager/...` | **通过**（本地已跑，2026-08-06） |
| 0.1 | 全仓库编译无误 | 基线 | `go build ./...` | **通过**（本地已跑，2026-08-06） |
| 1 | 创建一对 `iaas-pool` + `pair-pool` 的 v4/v6 池，注解正确同步为 label | FR-001 | `kubectl apply` 建池 + `kubectl get sp <name> -o jsonpath='{.metadata.labels}'` 校验含 `ipam.spidernet.io/iaas-pool=true` | **通过**（2026-08-10：`iaas-t-v4`/`iaas-t-v6` 创建成功，两池 label 均自动同步；附加验证删除注解后 label 也被自动移除。注意：需先将 `spiderpool-conf` 的 `enableIPv6` 改为 `true` 并重启组件，否则 v6 池被"IPv6 is disabled"拒绝——已改，见第 5 节） |
| 2 | 配对校验：拒绝自引用 (`pair-pool` 指向自身) | FR-003 | `kubectl annotate sp <name> ipam.spidernet.io/pair-pool=<same-name> --overwrite`，期望被 admission 拒绝 | **通过**（2026-08-10：拒绝，报 "cannot reference itself as a pair pool"） |
| 3 | 配对校验：拒绝同 IP 版本配对（两个池都是 v4 或都是 v6） | FR-003 | 构造两个同版本池互相 pair，期望拒绝 | **通过**（2026-08-10：拒绝，报 "must reference a pool of the opposite IP version"） |
| 4 | 配对校验：v4 静态容量 > v6 静态容量时拒绝 | FR-003 | 扩大 v4 池 `spec.ips` 使其地址数超过配对 v6 池，期望 `kubectl patch` 被拒绝 | **通过**（2026-08-10：拒绝，报 "v4 pool ... static capacity (4) must be <= v6 pool ... static capacity (2)"） |
| 5 | 配对校验：`nodeName`/`podAffinity` 不一致时拒绝 | FR-003 | 使两个配对池的 `nodeName` 或 `podAffinity.matchLabels` 不同，期望拒绝 | **通过**（2026-08-10：nodeName 与 podAffinity 两种不一致均被拒绝） |
| 6 | 配对校验：引用尚不存在的池时不应拒绝 | FR-003 | `pair-pool` 指向一个还未创建的池名，创建应成功 | **通过**（2026-08-10：`iaas-t-v4-future` 引用不存在的池创建成功） |
| 7 | 模拟写入 `status.iaasReadyIPs`/`status.iaasFailedIPs` 台账（含已就绪/预热失败条目） | FR-008 | `kubectl patch sp <name> --subresource=status --type=merge -p '...'`（见 quickstart.md 第 3 步） | **通过**（2026-08-10：向 `iaas-t-v4` status 写入 1 条 ready（含 mac/vlanID）+ 1 条 failed 条目成功，读回一致） |
| 8 | 单栈 Pod 从带 `pair-pool` 的池请求 IP：自动补全配对池候选，但不强制分配对侧地址 | FR-004, FR-005 | 创建仅声明 v4 池的 Pod，检查其只获得 v4 地址，v6 侧地址仍未分配 | 未开始 |
| 9 | 台账过滤：不在 `iaasReadyIPs` 中的地址（即使在 `spec.ips` 内，含 `iaasFailedIPs` 条目）不可被选中 | FR-009 | 构造 `iaasFailedIPs` 含条目、`iaasReadyIPs` 不含该地址的台账，创建 Pod，确认从不分配该地址 | 未开始 |
| 10 | 双栈 Pod 原子成对分配：v4/v6 必须来自同一台账条目，不可交叉 | FR-010 | 创建双栈 Pod，检查 `status.allocatedIPs`/Pod 注解中 v4、v6 地址确实成对（同一条目） | 未开始 |
| 11 | 台账已耗尽（无 ready 且未占用条目）时，走标准“无可用 IP”失败并允许多池 fallback | FR-013 | 耗尽台账后创建 Pod（声明多个候选池），确认该池报无可用 IP 但整体调度可 fallback 到其他池而非卡死 | 未开始 |
| 12 | 台账驱动的分配跳过原有同步云 API 调用路径 | FR-012, FR-015 | 结合 provider/mock 观察：从 ready 台账条目分配时不应触发实时云 API 调用（可通过 mockserver 请求日志或 provider 日志验证无新调用） | 未开始（依赖 provider 组件部署） |
| 13 | 回归：不带 `iaas-pool` 注解的存量池行为完全不变 | FR-006, FR-011 | 对集群中已有的普通池（如 `abc`）执行常规分配，确认无 label 被添加、无配对校验触发、分配结果与升级前一致 | **部分通过**（2026-08-10：新建普通池 `iaas-t-plain` 无 iaas label 被添加、无配对校验触发；实际 Pod 分配回归待做） |
| 14 | Webhook/Controller 升级后原有 e2e / 存量工作负载不受影响的冒烟检查 | 兼容性 | 观察 `spiderpool-agent`/`spiderpool-controller` Pod 升级后状态、日志无异常，抽查若干已有 Pod 网络正常 | **通过**（2026-08-06：两节点 agent 2/2 Running、controller 1/1 Running，0 次重启；日志无异常，webhook mutating/validating 正常工作；观察到的唯一 ERROR 是 `macvlan1.enp-pool` 历史遗留 subnet 校验问题，与本次改动无关） |
| 15 | 与 provider 组件联调：provider 真实写入台账 → Spiderpool 分配 → 云侧（mock）状态一致性 | 端到端 | provider 组件就绪后，创建声明 podAffinity 匹配的工作负载，观察 provider 日志/mockserver 记录的绑定操作与 Spiderpool 分配结果一致 | 未开始（**阻塞**：依赖 provider 组件适配本特性并部署，用户后续提供） |
| 16 | agent 启动时上报 parent-nics：`iaasNetworkProvider.serverUrl` 配置后，Node 注解 `ipam.spidernet.io/parent-nics` 写入本机物理网卡 `名称: MAC` map；`excludeReportNics` 生效；未配置 serverUrl 则不写注解 | 提案 §parent port 解析 | 在 ConfigMap 中配置 `iaasNetworkProvider.serverUrl` 后重启 agent，`kubectl get node -o jsonpath='{.metadata.annotations.ipam\.spidernet\.io/parent-nics}'` 检查两节点注解内容（应只含物理网卡，不含 veth/bridge/cali*）；配置 `excludeReportNics` 排除管理口后重启验证被排除 | 未开始 |

## 5. 环境相关注意事项

- **2026-08-10 配置变更**：为测试 v4/v6 配对特性，已将 `spiderpool-conf` ConfigMap
  的 `enableIPv6` 从 `false` 改为 `true`（`kubectl patch --type=merge`，注意
  `kubectl get -o yaml | sed | apply` 会因 yaml 折行匹配失败,需用 JSON patch），
  并 `kubectl rollout restart` 了 controller/agent。如需还原请改回 `false`。
- **遗留测试资源**：配对池 `iaas-t-v4`/`iaas-t-v6`（`iaas-t-v4` status 带模拟台账:
  ready=192.168.100.10/fd00:100::10, failed=192.168.100.11/fd00:100::11）保留在集群中,
  供后续 IPAM 分配用例（#8-#11）使用；用例 #2-#6、#13 的临时池已清理。

- 采用“本机构建 + 本机 containerd 直接使用”的方式，无需推送镜像到远程仓库；
  已通过 `docker save` + `scp`（经本地沙箱中转）+ `ctr -n k8s.io images import`
  同步到 worker 节点 `10.20.1.60`（**必须用 `ctr import` 而非仅 `docker load`**，
  因为该集群 kubelet 走独立 containerd，与 Docker daemon 镜像存储不共享，
  见第 2 节说明）。
- 构建镜像**必须**加 `E2E_CHINA_IMAGE_REGISTRY=true`，否则默认拉取
  `golang`/`ghcr.io` 官方镜像在当前网络环境下大概率构建失败或超时；即使加了该参数，
  daocloud 镜像源带宽也偏慢（本次构建约 1 小时，主要耗时在拉取 golang 基础镜像层）。
- `spiderpool-controller` Deployment 的 `imagePullPolicy` 默认是 `Always`，
  用本地构建、未推送远程仓库的镜像替换时**必须先 patch 为 `IfNotPresent`**，
  否则 Pod 会尝试联网重新拉取导致失败；`spiderpool-agent` DaemonSet 已经是
  `IfNotPresent`，无需处理。
- 集群通过公网访问 `ghcr.io` 拉取历史镜像（如 provider 组件相关）可能仍不稳定，
  与本次本地构建方式无关，遇到时再单独排查。
- 集群上原先的 `spiderpool` release 由 Helm chart `spiderpool-1.0.5` 部署，
  本次未使用 `helm upgrade`，而是手工替换 Deployment/DaemonSet 镜像 +
  `kubectl apply` CRD，因此 Helm release 记录的 chart 版本/values 仍是旧的
  （`helm list` 会显示与实际运行镜像不一致，这是预期的、刻意选择的手工升级方式，
  不是异常）。若后续需要用 Helm 管理，需手工同步 values 或改回 `helm upgrade`。
- 有若干 `spiderpool-init-*` Pod 处于 `Unknown` 状态（历史遗留），与本次测试无关，
  不需处理，但注意不要误判为本次改动引入的问题。

## 6. 后续步骤

1. ~~将本分支代码上传/同步到 `10.20.1.50`~~ ✅ 已完成。
2. ~~在 `10.20.1.50` 上构建 agent/controller 镜像~~ ✅ 已完成
   （commit `6a29ebf2b55945b835750bb877512930e87b702c`）。
3. ~~直接 `kubectl apply` CRD、核对 ConfigMap、`kubectl set image` 替换镜像~~
   ✅ 已完成，两节点 rollout 成功。
4. 按第 4 节用例表逐项执行用例 #1-#11、#13（Spiderpool 侧独立可测，无需 provider），
   并在本文件中更新每项的“状态”列（通过/失败/阻塞及原因）。
5. provider 组件适配本特性并部署完成后，补充执行依赖它的用例（#12、#15）。
