# 测试文档：IaaS Provider 预热 IP 池支持 (006-iaas-prewarm-pool)

**分支**: `006-iaas-prewarm-pool`
**关联设计文档**: `docs/develop/proposal-iaas-ip-provider.md`、`specs/006-iaas-prewarm-pool/{spec,plan,data-model,quickstart}.md`
**状态**: Spiderpool v6 agent/controller、CRD 及 provider v6 镜像均已部署到测试集群；
generation/cache、部分预热失败、批量双栈重建和零同步云调用测试均已通过
**最后更新**: 2026-08-21（池驱动判定重构 `4b9e89fe7` 已部署 `.50`/`.60` 并验证：manual SMC + iaas-global 池正常走 IaaS；IaaS 池 + macvlan 网络冷/热路径均 fail-closed 硬失败）

> **2026-08-21 池驱动判定重构验证（spiderpool `4b9e89fe7`）**：
>
> 重构（3 个提交）：`c2358e531` IaaS 介入改由池标记（`iaas-provider`/
> `iaas-global`）决定，vlanMode 仅保留 NAD 渲染开关；`60c43eea9` 修复
> subnet 缓存命中绕过 vlan 网络校验；`4b9e89fe7` 修复 warm path
> （metadata-sourced，零 RPC）绕过 vlan 网络校验——两处均改为先取 SMC
> 校验 vlan 类型（informer 缓存，无 provider RPC，SC-009 不受影响）。
> 部署验证（`.50`/`.60` agent/controller 均更新）：
>
> 1. **原陷阱场景转正（通过）**：SMC `gbasic-net-manual`（vlan、manual 模式、
>    无 vlanMode auto）+ iaas-global 池 `gbasic-v4` → Pod `gb-pod-manual`
>    正常触发 IaaS allocate（130.13/vlan 127/sub-ENI MAC）；删除重建走
>    warm path 零 RPC 复用同 IP 且通过校验。
> 2. **fail-closed 冷路径（通过）**：macvlan SMC + iaas-global 池 → CNI ADD
>    硬失败 `not a vlan CNI configuration`，全局池 claim 回滚。
> 3. **fail-closed 热路径（通过）**：同 IP 已有 metadata 条目时（warm path）
>    同样硬失败，不再静默放行。
> 4. **回归（通过）**：既有 `vlanMode: auto` Pod（gb-pod-50c/60/60b，default ns）
>    经两次滚动升级保持 Running；ENI 资源注入 `{}` 属预期
>    （`podResourceInject.namespacesExclude` 包含测试所用 ns）。

> **2026-08-21 全局池基本流程冒烟（spiderpool `0f63bf18` + provider `globalpool-either-marker` + mock `global-pool-97a7427`）**：
>
> 环境：`.50`/`.60` 集群 agent/controller 已更新为 `0f63bf18`（`cc8f30e1f` 之上
> 多一个 fix：iaas-global 池不再要求 iaas-provider label 即视为 IaaS 托管）；
> provider/mock 均为新镜像，mock 已内置干净子网 `192.168.130.0/24`、`140.0/24`
> （100/110/120 仍有种子数据）。provider `globalPool` 配置：
> `recycleStrategy=ttl, idleTTLSeconds=60, detach 0.6 / delete 0.9`。
>
> 测试资源：池 `gbasic-v4`（192.168.130.10-19，`iaas-global: "true"`）+
> SMC `spiderpool/gbasic-net`（vlan + `vlanMode: auto`）。全部场景通过：
>
> 1. **建池链路（通过）**：注解→label webhook 同步；provider globalpool-init
>    写入 v2 骨架（首次 apply 撞 resourceVersion 冲突 ERROR，30s 重试成功——
>    可建议 provider 对该冲突就地重试并降噪）。
> 2. **冷路径 + metadata 回写（通过）**：两节点各 1 Pod 6.8s Ready，
>    metadata 数秒内回写 mac/vlan/node，readyIPCount=2。
> 3. **SC-009 零 RPC 缓存命中（通过）**：删除重建同节点 Pod 3.5s Ready，
>    同 IP/MAC（Pod 内 eth0 实测 MAC 与 metadata 一致），mock 日志零增量，
>    agent `Skipping IaaS provider allocation call`。
> 4. **FR-021 冷路径分层（通过）**：.50 留有空闲缓存条目时，.60 新 Pod
>    走 Tier1 新建（130.12），不偷跨节点空闲条目。
> 5. **TTL 回收 detach（通过）**：空闲条目 130.11 约 100s 后被
>    globalpool-recycler `detached idle cached sub-ENI`，条目退化为仅剩
>    mac（无 vlan/node，sub-ENI 保留云侧未删）。
> 6. **detach 后 re-attach（通过）**：.50 新 Pod 复用 130.11——provider
>    `attached cached sub-ENI`（update 而非 create，同 sub-ENI/同 MAC、
>    新 vlan 273），HTTP 6.6ms。
>
> ⚠️ 踩坑（自身配置问题，非缺陷）：SMC 漏配 `vlanMode: auto` 时 agent 判定
> 非 provider 网络（`isProviderVLANSpiderMultusConfig` 要求 cniType=vlan +
> vlanMode=auto），静默跳过 IaaS 调用、Pod 仍能拿 IP 但无 sub-ENI——
> 全局池 SMC 必须显式 `vlanMode: auto`；且 patch 时需先 remove 默认的
> `vlanID: 0`（webhook 校验 auto 模式不允许 vlanID）。
> **后续修复**：已重构为池驱动判定——IaaS 介入由 Pod 所用池的
> `iaas-provider`/`iaas-global` 标记决定，vlanMode 仅保留 NAD 渲染开关职责；
> IaaS 池 + 无法解析父 NIC（缺 SMC/非 vlan 网络）改为分配硬失败（fail-closed），
> 不再静默跳过。
> 测试资源保留：池 `gbasic-v4`、SMC `gbasic-net`、Pod
> `gb-pod-50c/gb-pod-60/gb-pod-60b`，供后续性能测试复用。
> 注：`rateLimit` 仍是默认 `qps=2, burst=10`——性能测试前须先调大
> （参考 2026-08-19 livelock 记录，建议 qps=50/burst=100）。

> **2026-08-19 全局池第二轮联调记录（spiderpool `b7a04cb97` + provider `global-pool-4`）**：
>
> provider 更新到 `controller:global-pool-4` 后，首轮阻塞项（allocate 路径未接
> globalpool 状态机）已修复。全部核心场景通过：
>
> 1. **冷路径 + metadata 回写（通过）**：新 Pod（130.13/130.14）allocate 时
>    provider 日志出现 `allocate IPs routing item to global pool`（使用了
>    spiderpool 新发的 `ipv4PoolName` 字段路由）→ `created and attached
>    sub-ENI` → metadata 数秒内 flush 进 `status.ipMetaData.metadata.ips`
>    （含 mac/vlan/node），readyIPCount 同步递增。
> 2. **SC-009 零 RPC 缓存命中（通过）**：删除 Pod 后重建（同节点），agent 日志
>    `Skipping IaaS provider allocation call: all results are sourced from
>    prewarm ipMetaData`，provider 侧零 HTTP 请求，IPAM 全程 18ms，
>    IP/MAC/VLAN 与缓存条目完全一致。provider 重启后缓存命中依然生效。
> 3. **FR-021 冷路径分层（通过）**：.60 上的 Pod 未复用 node=.50 的空闲缓存
>    条目，而是优先选无 metadata 的未绑定地址（Tier 1，单次云调用），符合设计。
> 4. **跨节点 steal 迁移（通过）**：2-IP 小池 `iaas-steal-v4`（140.10/11）在
>    .50 占满释放后，.60 申请触发 Tier 2：provider 日志 `moved sub-ENI across
>    nodes`（8.8ms），140.10 迁至 .60 并获得新 VLAN，metadata 同步更新。
> 5. **watermark 回收（通过）**：steal 后残留的空闲缓存 140.11 被
>    globalpool-recycler 先 `detached idle cached sub-ENI` 再
>    `deleted unbound sub-ENI`，metadata 条目同步移除。
> 6. **重启 rebuild 认领（通过）**：provider 重启后
>    `global-pool state rebuild complete pools:2 cloudSubEnis:7 suspects:0`，
>    ownership tag 已正确打上，全部 sub-ENI 被认领，metadata 完整保留。
> 7. **fail-closed 边界（符合预期）**：新建池后立刻创建 Pod，首个 sandbox 因
>    `IaaS IP metadata not ready: pool status has no observed generation`
>    失败（provider 骨架尚未写入），kubelet 重试数秒后成功——fail-closed
>    行为正确，无需修复。
>
> 待办：还原 agent `SPIDERPOOL_LOG_LEVEL=info` 与 provider logLevel
> （备份 .50 `/tmp/prov-cm-backup-20260818.yaml`）；测试资源
> `iaas-g-pod1/2/5/7/9`、`iaas-s-c`、池 `iaas-g-v4`/`iaas-steal-v4` 保留。

> **2026-08-19 全局池规模测试（spiderpool `b7a04cb97` + provider `global-pool-4`）**：
>
> **【provider 反馈·限流 livelock】** 首次尝试 400 Pod（两节点各 200、单 400-IP
> 全局池）冷启动时，provider 默认 `rateLimit: qps=2, burst=10,
> queueTimeoutSeconds=30` 在高并发下形成 livelock：40 分钟内 allocate 成功 0 次、
> 队列超时 1598 次、sub-ENI 创建 0 次——每个请求排队 ~30s 拿到 token 后 agent 侧
> 已放弃（`context canceled`），云调用被浪费，无任何请求能完成。建议 provider：
> 全局池模式下用 `maxConcurrentPerPool` 控制云并发即可，HTTP 入口限流应放宽或
> 对缓存命中路径旁路。测试环境将 qps 临时调至 50/burst 100 后问题消失
> （原配置备份 .50 `/tmp/prov-cm-backup-20260819-prerate.yaml`）。
> 另：400 Pod 首测还踩中 mock 未定义子网 `192.168.170.0/23` 返回 404
> `subnet not found in cache`（环境限制，非缺陷）；以及 `nodeName` 直绑绕过
> 调度器导致 maxPods 满时 kubelet OutOfpods 失败风暴（7260 个 Failed Pod），
> 后续规模测试应使用 nodeSelector/默认调度。
>
> **64-IP 池 × 32 Pod 基准（默认调度，qps=50）**：单 Deployment 32 副本用
> 64-IP 全局池 `g64-v4`（192.168.130.30-93），不限制调度（实际两节点均衡 16/16）：
>
> | 场景 | 耗时 | 云请求 | 说明 |
> |------|------|--------|------|
> | T1 首次冷启动 | **7.58s** | 65（32 次真实创建） | 32/32 Ready |
> | T2 spec 更新滚动重启 ×5 | 25.09 / 21.82 / 20.25 / 18.84 / 18.68s，**平均 20.9s** | 每轮 ~49-65（~20 次新建 + 0 次迁移） | RollingUpdate surge/unavail 25% |
>
> 每轮重启中 agent 缓存命中（`Skipping IaaS provider allocation call`）~51 次；
> 仍有 ~20 次/轮新建的原因：recycler 按 detachThreshold=0.6 水位把空闲缓存裁到
> ~38 条，滚动 surge 的新 Pod 超出缓存量部分走冷路径。若想重启全程零云调用，
> 需缓存量 ≥ 32×1.25（surge 峰值），即调低回收水位或加大池内常驻缓存。
> 终态一致性审计：32/32 Pod 的 IP/节点与 metadata 完全匹配，两节点缓存分布
> 16/16 与 Pod 分布一致。对比 2026-08-14 节点级预热池（400 Pod 重启 ~62s）：
> 全局池 32 Pod 规模下滚动重启 ~21s，量级一致（主要由 RollingUpdate 分批节奏
> 主导），冷启动 7.6s 显著优于非预热实时模式同规模水平（预估 >60s @qps=2 时代）。
>
> **水位线调优复测（detach 0.6→0.9、delete 0.9→1.0，即默认不删除空闲
> sub-ENI、90% 占用才解绑）**：provider 配置改为 `detachThreshold: 0.9,
> deleteThreshold: 1.0` 后（原配置备份 .50
> `/tmp/prov-cm-backup-20260819-thresholds.yaml`），重启 rebuild 认领 52 个
> sub-ENI（包括此前已解绑未删的），继续滚动重启 3 轮：
>
> | 轮次 | 耗时 | 云请求 | 缓存条目 |
> |------|------|--------|----------|
> | 1（缓存回填轮） | 18.19s | 27 | 38→51 |
> | 2 | 17.82s | **1（零云调用**，仅测量 GET） | 51 |
> | 3 | 16.13s | 3 | 52 |
>
> 缓存一旦覆盖 surge 峰值（40=32×1.25），后续重启即全程缓存命中（agent
> `Skipping IaaS provider allocation call` 82 次/2 轮），耗时也从 ~21s 降至
> ~16-18s。SC-009 零 RPC 重启在真实滚动更新场景下达成。结论：对 IP 富余的
> 全局池，推荐 `detachThreshold ≥ 0.9`、`deleteThreshold = 1.0` 作为默认。
>
> **全保留模式（detach=1.0/delete=1.0）**：再调为永不解绑/永不删除后连测 3 轮
> 重启：18.63 / 21.40 / 6.83s，云调用全部为 0；缓存稳定 52 条（每节点 26），
> 调度均衡时连 steal 都不触发。性能最优，代价是 sub-ENI 永久占用云侧 port
> 配额，适合 IP 配额充足、节点集稳定的场景。
>
> **400 Pod（每节点 200，IP:Pod 1:1）复测（qps=50 + 全保留水位）**：
> 池 `gscale-50-v4`（130.30-229）/`gscale-60-v4`（140.20-219）各 200 IP，
> Deployment nodeSelector 各绑一节点 200 副本，busybox 缓存镜像：
>
> | 场景 | 耗时 | 云请求 | 对比 2026-08-14 节点级预热池 |
> |------|------|--------|------------------------------|
> | T1 首次冷启动（400 次真实创建） | **36.12s** | 801 | 预热池冷启动 22.3s（预热已提前完成）；qps=2 时代此场景直接 livelock |
> | T2 spec 更新滚动重启 ×3 | 65.73 / 73.62 / 67.55s，**平均 68.97s** | 每轮 ~41-51（含 ~8 次真实新建） | 节点级预热池 ~62-63s，同量级 |
>
> 1:1 满负荷下重启的主导耗时是"新 Pod 等旧 Pod 释放 IP"的滚动节奏（与节点级
> 预热池一致）；每轮 561 次缓存命中 vs 仅 ~8 次冷路径漏出（约 2%，滚动竞态窗口
> 内 informer 快照落后于释放事件所致，provider 幂等兜底）。
> **云侧终态审计：mock 云 130/140 子网恰好 400 个 sub-ENI，无重复无泄漏**
> （请求史上 104 个 IP 有 2-3 次重复 create，均为 T1 首波 CNI 超时重试与滚动
> 竞态重试，被幂等吸收）；Pod IP/节点与 metadata 对账 200/200 匹配。

> **2026-08-19 P0 补充用例记录（spiderpool `b7a04cb97` + provider `global-pool-4`）**：
>
> **P0-1 双栈全局池配对（FR-024/SC-012）——通过** ✅：
> 创建互相 `ipam.spidernet.io/pair-pool` 注解的全局池 `gpair-v4`
> （192.168.130.230-245，16 IP）+ `gpair-v6`（fd00:130::1-::10，16 IP），
> 8 副本双栈 Deployment：
> - 8/8 Pod 双栈就绪，Pod 实际 v4/v6 与 v4 池 metadata `entry.ipv6` 对账全匹配；
>   v6 池 allocatedIPs 与 v4 侧一一对应。
> - 第一轮滚动重启：6 个 Pod 走新 IP 冷路径（非 1:1 有空闲 IP 属预期），
>   **新分配 v6 严格跳过已被 metadata 锁定的 ::1-::8**（FR-024 v6 排除生效）；
>   缓存增至 14 条、14 个 v6 无重复占用。
> - 第二轮重启（缓存 14 ≥ surge 峰值 10）：**allocate 云调用 0 次**
>   （mock delta 仅 GET/PUT/DELETE 各 1，为 16/16=100% 触发 delete 水位回收
>   1 条空闲），8/8 配对保持。双栈零 RPC 重启达成（SC-009 双栈版）。
> - 期间发现两点：① informer 滞后时 agent 冷路径选出与既有配对不符的 v6，
>   provider 返回 400 `does not match the pool's pairing` 正确拦截，kubelet
>   重试自愈（配对唯一性由 provider 兜底，符合设计）；② `.60` 节点宿主缺
>   `fd00:130::/112` 直连路由导致 coordinator `RouteGet network unreachable`，
>   补 `ip -6 route add fd00:130::/112 dev enp11s0f0np0` 后恢复（环境问题）。
>
> **P0-2 RPC 失败回滚 + 恢复自愈（SC-010）——通过** ✅：
> provider 缩容至 0 后用全新池 `gfail-v4` 创建 Pod 触发冷路径：
> - 分配失败，**池 allocatedIPs 始终为 0，无泄漏 claim**（rollbackGlobalPoolClaims
>   生效）；kubelet 周期重试不产生脏数据。
> - provider 恢复后 kubelet 下一次重试即成功获得 IP 并 Running；
>   期间一次 CNI ADD 中断在 Pod netns 留下已建 vlan 接口导致后续重试报
>   `create vlan: file exists`，删 Pod 重建即恢复（cmdDel 会清理 netns；
>   同 UID 重试撞残留是 vlan 插件已知行为，非本特性缺陷）。
>
> **P0-3 混合模式共存（SC-011）——通过** ✅：
> 同一节点同时运行三种模式 Pod：全局池双栈（`gpair-v4/v6`，跨节点 steal 迁移
> 条目后仍保持 v4+v6 配对 .232/::3）、节点级预热池（`gmix-node50-v4`，
> `nodeName: [10-20-1-50]` + prewarm，scope=节点、条目无 node 字段，Pod 秒级
> 命中预热条目）、普通非 IaaS macvlan 池（`macvlan1.enp-pool`）。三者互不干扰，
> 各池 metadata/allocatedIPs 形态符合各自模式定义。
> 清理阶段复现 2026-08-17 已记录的 provider 缺陷：v4 主池先删完后，
> 孤儿 v6 从池 `gpair-v6` 的 `iaas-cleanup` finalizer 无人摘除（provider
> 日志对 v6 池零输出），再次手动摘除后删除完成——该缺陷仍未修复，需提醒
> provider。
>
> **【测试环境教训·mock 无持久化】** P0-2 误将 mockserver 一并缩容重启，
> mock 内存态云资源（400+ sub-ENI）全部丢失；provider 重建后以云为事实源
> 如实清空了 gscale 池的 metadata（行为正确），但 mock 重新随机发放 vlan id
> 撞上仍在运行的旧 Pod 占用的 vlan 893（同父接口 vlan id 内核全局唯一，
> 跨 netns 也冲突）导致新 Pod `create vlan: file exists`。教训：mock 重启
> 等价"云侧数据全失"演练；真实云不会发生，但 vlan id 复用冲突提示 provider
> 分配 vlan 时应避开节点上仍存活的旧值。

> **2026-08-19 P1 补充用例记录（spiderpool `b7a04cb97` + provider `global-pool-4`）**：
> 基础负载：全局池 `p1-v4`（192.168.130.30-61，32 IP）+ 16 副本 Deployment，
> 默认调度两节点均衡 8/8。
>
> - **P1-1 数据面连通性——环境受限，可验部分通过**：Pod↔本机宿主
>   （coordinator veth + table 500 路由）双向 OK；Pod↔Pod 与跨节点不通——
>   Pod eth0 是 mock 随机 vlan id 的子接口，物理交换机未配置这些 vlan，
>   属 mock 环境限制（真实云由 IaaS vlan fabric 承载），非特性缺陷。
> - **P1-2 负载中 provider 故障切换——通过** ✅：滚动重启进行中
>   `rollout restart` provider：p1 滚动 32.6s 正常完成、16/16 Running、
>   Pod/metadata 对账 16/16，provider 重启后 rebuild
>   `cloudSubEnis:21 suspects:0`。缓存命中路径不依赖 provider 存活，
>   冷路径请求由 kubelet 重试吸收。
> - **P1-3 GC 回收——通过** ✅：把 agent 从 .60 摘除后 `--force` 强删 Pod
>   （CNI DEL 无法执行）：陈旧 claim 约 45s 内被 controller GC 释放，
>   且释放的 IP 立即被新 Pod 缓存命中复用；正常 force-delete（agent 在位）
>   路径同样即时释放。⚠️ 环境提示：agent DaemonSet 内捆绑 multus，摘除
>   agent 的节点 multus 失效，新 Pod 会静默回退 cilium 默认 CNI 拿到
>   非池内 IP（注解被绕过）——运维时对 agent 缺位节点应先 cordon。
> - **P1-4 节点排水——通过** ✅：drain .60 后 16 Pod 全部迁至 .50，
>   ~120s 内全部 Running；6 次冷路径分配全部走 tier1 新建（池内尚有
>   空闲 IP，未动 .60 的 11 条空闲缓存条目，符合 FR-021 "steal 为最后
>   手段"分级）；迁移后 Pod/metadata 对账 16/16。已 uncordon 复原。


> **2026-08-18 全局池首轮联调记录（spiderpool `d341424f7` + provider `global-pool-2`）**：
>
> 环境准备：清理全部历史 iaas 测试资源（6 个 Deployment、11 个池、4 个 SMC；
> 3 个 v6 从池再次复现"孤儿配对池 finalizer 不回收"已知缺陷，手动摘除）。
> provider `controller:global-pool-2` 已带 `globalPool` 配置（detach 0.6 /
> delete 0.9 / flushDebounceMs 500）。
>
> **踩坑：mock 种子数据**——mock 配置 seed 了 192.168.100（38 条）/110（200 条）
> /120（200 条）三个子网的 sub-ENI；全局池若用这些子网，provider allocate 恒走
> ips-cache 幂等命中（不真正建 sub-ENI）。全局池测试须用干净的
> 192.168.130.0/24 或 140.0/24。
>
> 验证结果（池 `iaas-g-v4` 192.168.130.10-29 + SMC `iaas-g-net`）：
>
> 1. **建池链路（通过）**：`iaas-provider` 注解→label 自动同步；provider
>    globalpool-init 正确写入 v2 骨架
>    `{"scope":"","parentNic":"enp11s0f0np0","ips":{}}`。schema 与 spiderpool
>    `IPMetadataEntry`（scope/parentNic/ips/ipv6/mac/vlan/node）经二进制
>    json tag 核对一致。
> 2. **冷路径实时分配（通过）**：Pod 2-6s Ready，cache miss →
>    create-sub-network-interface（2-4s）→ 返回 mac/vlan → endpoint 记录正确。
> 3. **DEL 粘性（通过）**：删 Pod 无任何 release 调用，重建 Pod 复用同一
>    IP/MAC/VLAN（由 provider ips-cache 幂等命中兜底，0.3ms）。
> 4. **【阻塞·provider 缺陷】metadata 永不回写**：allocate 热路径
>    （handler→ipam-service→cloud→ips-cache）全程未触碰
>    `globalpool.Allocator/FlushQueue`（debug 级日志确认零 globalpool 调用）；
>    sub-ENI 创建时未打 `{pool, ip}` ownership tag，重启 rebuild 亦无法认领
>    （`cloudSubEnis:0`）。后果：`ips` 恒为 `{}`，SC-009 零 RPC 缓存命中永远
>    不触发，每次 ADD 都发 RPC。provider 二进制中 `globalpool.Allocator`、
>    `GenerateGlobalOwnershipID` 及日志文案 "no global-pool handler is wired"
>    均已存在——需 provider 把 ipam handler 与 globalpool allocator 接线。
> 5. **spiderpool 侧配合改动（已实现待部署）**：allocate API `subEniRequests[]`
>    新增可选 `ipv4PoolName`/`ipv6PoolName`，release API 新增可选 `poolName`，
>    供 provider 直接归因（免 `{subnet, ip}` 反查）；旧 provider 忽略即兼容。
>
> 环境备注：provider logLevel 临时改为 debug（原配置备份
> .50 `/tmp/prov-cm-backup-20260818.yaml`，测毕还原）；测试 Pod
> `iaas-g-pod1/2/3`（130.10/11/12）保留供 provider 修复后回归。

> **2026-08-20 部署记录（spiderpool `cc8f30e1f`，全局池显式标记 iaas-global）**：
>
> 部署内容：`b7a04cb97` → `cc8f30e1f` 增量（feat: 全局池识别改为显式
> `ipam.spidernet.io/iaas-global: "true"` 注解/label——注解权威、mutating
> webhook 同步 label，validating webhook 拒绝非 `"true"` 值；不再由
> iaas-provider 注解 + 空 `spec.nodeName` 推断，`IsGlobalIaaSPool` 也不再
> 要求 iaas-provider）。流程同前：增量 git bundle → .50
> `/root/spiderpool-build` 构建（缓存命中，约 3 分钟）→ `docker save` +
> `ssh .50 cat | ssh .60` 中转 → 两节点 `ctr -n k8s.io images import` →
> `kubectl set image` 滚动更新（本增量无 CRD 变更）。agent 2/2、controller
> 1/1，0 重启。冒烟验证：创建带 `iaas-global: "true"` 注解的池后 label 被
> webhook 自动同步；注解值 `"yes"` 被 validating webhook 拒绝（报错
> `the only valid value is "true"`）。临时 bundle/tar 已清理，冒烟池
> `iaas-t-global-smoke` 已删除。⚠️ 注意：存量全局池需补打
> `iaas-global: "true"` 注解方可继续走全局池逻辑。

> **2026-08-18 部署记录（spiderpool `d341424f7`，US4 全局池模式代码上环境）**：
>
> 部署内容：`59d10f53b` → `d341424f7` 增量（`8c14da2c2`/`3b4a1a545`/`f1a6ffa7b`
> 全局池设计文档、`df67e4243` feat: 全局 IaaS 池实时分配 + 粘性 sub-ENI 缓存、
> `d341424f7` fix: 全局池 metadata 契约加固 + 池类互斥校验）。流程同前：增量
> git bundle → .50 `/root/spiderpool-build` 构建（缓存命中，约 3 分钟）→
> `docker save` + `ssh .50 cat | ssh .60` 中转 → 两节点 `ctr -n k8s.io images
> import` → `kubectl apply -f charts/spiderpool/crds/`（本增量无 CRD schema
> 变更，幂等确认）→ `kubectl set image` 滚动更新。agent 2/2、controller 1/1，
> 0 重启；日志无新增错误（仅遗留 SMC `vlan-auto2` 无效 master 的历史 WARN）。
> 临时 bundle/tar 已清理。全局池模式 e2e 用例（SC-009..SC-012 对应链路）待执行。

> **2026-08-14 部署与验证记录（地址族透传，spiderpool `59d10f53b` + provider 镜像 `singlestack-1`）**：
>
> 部署内容：spiderpool `59d10f53b`（fix: `callIaaSAllocate` 不再强制
> v4/v6 成对，按实际分配的地址族透传给 provider；`callIaaSRelease` 与
> GC tracePod 释放路径 v4 缺失时回退 v6 地址）；provider 侧由使用者
> 部署 `controller:singlestack-1`（支持 ipv4Address/ipv6Address 均可选）。
> 测试资源：池 `iaas-fam-v4`（192.168.130.230-234）/`iaas-fam-v6`
> （fd00:130::230-234），SMC `iaas-fam-{v4,v6,ds}-net`。
>
> 验证结果：
>
> 1. **单栈 v4 Pod（通过）**：Pod 7s Ready，分得 192.168.130.230，
>    provider 回填 mac fa:16:3e:39:6c:95 / vlan 1129；allocate 请求 item
>    仅含 `ipv4Address`，200/4.2s。删除后 cmdDel 同步 release 429（预算
>    不足，预期）→ GC 兜底 202 完成，缓存 404、池 allocated 归零。
> 2. **单栈 v6 Pod（spiderpool 侧通过，provider 侧受限）**：spiderpool
>    正确透传 v6-only 请求（item 仅含 `ipv6Address`，subnet 取 v6 池
>    CIDR `fd00:130::/112`），provider 通过请求形状校验但返回 500
>    `cache miss: subnet not found: fd00:130::/112`——provider 的
>    `iaasnet_cache` 子网缓存仅按 IPv4 CIDR 索引（代码中无任何 v6 CIDR
>    字段），v6-only 需 provider 侧补充 v6 子网索引后方可打通。
> 3. **双栈 Pod（通过）**：Pod 2s Ready，分得 192.168.130.230 +
>    fd00:130::231，v4/v6 两条 ips-cache 共享同一 mac
>    fa:16:3e:f5:b8:cf / vlan 2548（成对语义保持）；删除后一次 release
>    级联释放两族，v4/v6 缓存均 404。
>
> 结论：透传语义符合预期——spiderpool 不再施加任何地址族限制；单栈
> v6 的剩余阻塞点在 provider（子网缓存无 v6 索引），非 spiderpool 问题。
> 测试资源已全部清理（v6 池因失败保留分配记录残留 finalizer，手工移除）。

> **2026-08-14 部署与验证记录（X-Request-Timeout-Ms 权威预算链路，spiderpool `b1cddc4c9` + provider 工作区构建 `subeni-header-dirty`）**：
>
> 部署内容：spiderpool agent/controller 更新至 `b1cddc4c9`（含 `cbe82ab35`
> GC 预热池跳过修复 + `b1cddc4c9` 移除 48s 本地预检）；provider 以本地
> 工作区（未提交）重建镜像 `subeni-header-dirty`，包含从 main 同步的
> `parseRequestTimeoutBudget`（读取 `X-Request-Timeout-Ms` header 做权威
> 预算校验，本地分支此前用 `ctx.Deadline()` 在裸 net/http 下为死代码）。
>
> 验证结果（全部符合预期）：
>
> 1. **allocate 链路**：provider 日志 `requestTimeoutEnabled: true,
>    requestTimeoutSec: 50`——header 被正确解析；双栈整对分配 200/1.2s，
>    Pod 分得 192.168.130.220 + fd00:130::220（mac fa:16:3e:38:2f:9a /
>    vlan 1506），与 ips-cache 两条目一致。
> 2. **cmdDel 同步 release 新行为**：不再本地跳过（旧日志 "parent budget
>    insufficient" 消失），实际发出请求并携带 `requestTimeoutSec: 24.997`；
>    provider 预检以 required=46s（30s queue + 16s txn）判定预算不足，
>    **在消耗限流令牌前** 0.15ms 内返回 429 RateLimitTimeout；agent 记录
>    错误但 CNI DEL 正常完成（fail-open）。
> 3. **GC 兜底**：约 2s 后 controller tracePod 以 50s 预算重发 release，
>    202 受理，异步 4.2s 完成；v4/v6 缓存条目均 404，池 allocated 归零。
> 4. 测试资源已清理（`iaas-ds-*`）。环境备注：两节点 `enp11s0f0np0`
>    物理链路 NO-CARRIER，宿主机缺 v6 路由导致 coordinator
>    GetGatewayIP(v6) 失败,已在 .50 手工补 `ip -6 route add fd00:130::/112
>    dev enp11s0f0np0`（与本次改动无关，后续双栈测试若换子网需同样处理）。
>
> 结论：方案一（删除 spiderpool 侧 `IaaSProviderWorstCase` 常量预检，由
> provider 依据自身 rateLimit 配置做单一事实源校验）端到端行为正确。
> 默认宽限期（25s）下同步 release 仍被 provider 拒绝、由 GC 兜底，若需
> 同步释放生效可建议 provider 对 202 异步的 release 将 required 放宽为
> queueTimeout。

> **2026-08-13 部署与验证记录（双栈 sub-ENI 实时创建链路，commit `b04c790ba`）**：
>
> 部署内容：commit `b04c790ba`（feat: adopt provider dual-stack sub-ENI
> allocate API，客户端 `SubEniRequests`/`SubEniResponses` 双栈成对语义 +
> `callIaaSAllocate` 按 NIC 配对 v4/v6、共享 MAC/VLAN 合并回写）。流程：
> 增量 git bundle（基点 `2f5130f3e`，远端 `/root/spiderpool-build` 先 reset
> 旧 patch）→ `make build_image E2E_CHINA_IMAGE_REGISTRY=true`（约 2 分钟）→
> `docker save` + 本地中转 scp（.50 无法直连 .60，`ssh .50 cat tar | ssh .60`）
> → 两节点 `ctr -n k8s.io images import` → `kubectl set image` 滚动更新
> （本次无 CRD 变更）。agent 2/2、controller 1/1，0 重启。
>
> provider 侧：镜像 `subeni-contract-d84cca2-dirty`（已实现新 subEniRequests
> API）扩容回 1 副本。注意其 prewarm controller 的 CRD 兼容性检查仍要求池级
> `status.ipMetaData.parentNic`（`pkg/controller/ippool/discovery.go`），与
> 集群新 CRD（parentNic 折入 metadata JSON）不兼容导致 CrashLoop——临时将
> configmap `controller.iaasIPPrewarm.enabled` 改为 false 只跑 ipam HTTP
> server（原配置已备份到 .50 `/tmp/provider-cm-backup.yaml`）；**provider 侧
> discovery.go 适配新 schema 后需恢复 enabled: true**。
>
> 实时创建（非预热同步 allocate）双栈链路验证（全部通过）：
>
> 1. **双栈整对分配**：非预热池 `iaas-ds-50-v4`（192.168.130.220-229）+
>    `iaas-ds-50-v6`（fd00:130::220-229）+ 双栈 SMC `iaas-ds-net`
>    （cniType vlan / vlanMode auto / master enp11s0f0np0）。Pod 创建后一次
>    allocate-ips 调用（单个 subEniRequest 含成对 ipv4Address+ipv6Address）
>    返回 200；Pod 分得 192.168.130.220 + fd00:130::220，endpoint 记录
>    mac=fa:16:3e:31:ab:40 / vlan=2809，与 provider ips-cache 中 v4、v6 两条
>    缓存条目完全一致（两条共享同一 MAC/VLAN，确认服务端成对建 sub-ENI）。
> 2. **单次按 IPv4 释放级联清除整个 sub-ENI**：删除 Pod 后仅发一次
>    release-ip（ipAddress=192.168.130.220），provider 返回 202 并异步完成，
>    v4 与 v6 两条缓存条目均被清除（ips-cache 查询均 404 NotFound），池
>    allocated 归零。cmdDel 同步释放因 CNI 时间预算不足（24.997s < 48s
>    worst-case）按设计跳过，由 controller GC 兜底异步释放成功——降级链路
>    亦得到验证。
> 3. **负路径**：曾误用 mockserver 未注册的 192.168.150.0/24 子网，provider
>    正确返回 500 "cache miss: subnet not found"，spiderpool 分配失败且不产生
>    endpoint。分配失败后删 Pod，池 `status.allocatedIPs` 会暂留一条记录
>    （IaaS 同步分配失败时池侧记录不立即回滚，进入内存 failure cache 供重试
>    复用）；**后续复现确认这不是泄漏**：GC scanAll（默认 600s 周期）在下一轮
>    正确回收了 v4/v6 池残留记录（日志 "scan all successfully reclaimed"），
>    池删除随之解除阻塞。首次测试中"GC 未回收"系观察窗口不足（仅等 60s，且
>    上一轮 scanAll 恰在失败发生前 1 秒跑过）。真实遗留问题见下。
>
> 复现调查中发现的真实问题（待跟进）：
>
> 1. **scanAll GC 的 IaaS release 在 endpoint 缺失时发送空 nodeName**：
>    `pkg/gcmanager/scanAll_IPPool.go` GCIP 分支中 endpoint 为 nil 时
>    `NodeName=""`，provider 返回 400 "nodeName, ipAddress, and subnet are
>    required"，云侧资源无法经此路径释放（本例中云侧本无资源，无实际泄漏）。
> 2. **GC 两条兜底路径未跳过预热池**：`callIaaSRelease`（cmdDel 同步路径）
>    对 `IsIaaSPool` 预热池跳过 IaaS release 以保留云侧预留，但
>    `scanAll_IPPool.go` 与 `tracePod_worker.go` 的 IaaS release 无此检查，
>    GC 兜底时会误拆预热池的云侧 sub-ENI 预留。
> 3. **cmdDel 同步 IaaS release 在默认宽限期下必然跳过**（设计后果，非 bug）：
>    `pkg/ipam/release.go` 将 DEL ctx 压缩为 `DeletionGracePeriodSeconds-5`
>    （默认 30-5=25s），恒小于 `IaaSProviderWorstCase`（30s 限流等待+16s 云
>    API+2s=48s）预算检查，同步释放 fail-fast 跳过，实际释放全部由 GC
>    tracePod 异步兜底完成（已验证成功）。如需同步释放生效，Pod 宽限期需
>    ≥53s，或重新评估 worst-case 预算与 CNI DEL 时间窗的匹配。
>    **已处理（方案一）**：移除 spiderpool 客户端的 48s 本地预检常量
>    （`IaaSProviderWorstCase` 及组成常量），剩余预算仅经
>    `X-Request-Timeout-Ms` header 传给 provider，由 provider（main 分支
>    `parseRequestTimeoutBudget`）以自身 rateLimit 配置做权威预算校验，
>    避免两侧配置漂移。默认宽限期下同步 release 仍会被 provider 以预算
>    不足拒绝（required=queueTimeout+txnTimeout≈46s > 25s），由 GC 兜底；
>    后续可建议 provider 对 202 异步的 release 放宽 required 至
>    queueTimeout。
>
> 测试资源已全部清理（`iaas-ds-*`）。mockserver 可用子网：
> 10.20.1.0/24、192.168.{100,110,120,130,140}.0/24（新建测试池须落在其中）。

> **2026-08-13 部署与验证记录（pair-or-nothing 配对修复 + sub-ENI 双栈 API）**：
>
> 部署内容：commit `2f5130f3e`（含 `c5304474b` pair-or-nothing 配对分配重构、
> `d4d5f7d8e` 重复/外来 IPv6 加固、`2f5130f3e` parentNic 折入 metadata JSON +
> 取消 vendor 白名单）+ 未提交的 eni api ipv6 改动（`pkg/iaas/client` 同步
> allocate API 重构为 `subEniRequests`/`subEniResponses` 双栈 sub-ENI 语义，
> 以 patch 方式应用于远端构建仓库）。流程沿用既有方式：增量 git bundle（基点
> `8fada0841`）+ patch → `make build_image E2E_CHINA_IMAGE_REGISTRY=true`
> （约 2 分钟，基础镜像已缓存）→ 本地沙箱中转 scp + 两节点
> `ctr -n k8s.io images import` → `kubectl apply` 更新 CRD（schema 移除池级
> `status.ipMetaData.parentNic`）→ `kubectl set image` 滚动更新。两节点 agent
> 2/2 Running、controller 1/1 Running，0 重启。
>
> **注意**：因新 schema 移除池级 `parentNic`（改为 metadata JSON 内保留键），
> 旧版 provider 的 SSA status 写入会被拒，部署前已将 `iaas-network-provider`
> 缩容为 0（待 provider 适配新 schema 与新 allocate API 后再恢复）。验证使用
> `iaas-t-v4` 存量台账（6 条 ready，observedGeneration=18 匹配）。
>
> 配对分配修复验证（全部通过）：
>
> 1. **单池声明整对分配**：Pod 仅声明 `{"ipv4":["iaas-t-v4"]}`，分得
>    192.168.100.30 + fd00:100::30（同一台账条目），endpoint 记录
>    mac=fa:16:3e:51:15:19/vlan=1965 与台账一致；v4/v6 两池 status 均记录同一
>    PodUID。
> 2. **误配 v6 候选池被忽略（issue 1 修复）**：Pod 同时声明配对 v4 池 + 普通
>    v6 池 `iaas-fix-plain-v6`，实际分得配对条目 .31/::31（仅一个 IPv6，无重复
>    分配），普通 v6 池 allocatedIPCount=0；agent 日志输出预期 WARN
>    "Ignore the configured IPv6 IPPools ... pair allocation already supplies
>    the IPv6 address"。
> 3. **并发无交叉**：Deployment 5 副本并发创建，5 个 Pod 的 v4/v6/MAC/VLAN 与
>    各自台账条目 5/5 精确匹配，无交叉配对、无重复 IPv6；两池 allocated 均为 5。
> 4. **整对释放**：删除全部测试 Pod 后两池 allocatedIPCount 归零，无 endpoint
>    残留。
> 5. **乱序映射整池耗尽（reviewer 原始 bug 场景）**：手工将台账改为完全乱序映射
>    （.30→::35、.31→::33、.32→::34、.33→::30、.34→::32、.35→::31），并发创建
>    6 个仅声明 v4 池的 Pod 恰好耗尽整池；6/6 Pod 的 IPv6/MAC/VLAN 均与其 v4
>    地址对应的台账条目精确一致（旧逻辑按两侧地址各自升序独立选取必然交叉，
>    如 .30 会错配 ::30）。测试后台账已恢复为原顺序映射，两池分配归零。
>
> eni api ipv6（同步 allocate 路径）改动已随镜像部署，`pkg/iaas/...` 单测通过；
> 实时创建路径的端到端验证依赖 provider/mockserver 适配新
> `subEniRequests` API，暂缓。测试资源已清理（`iaas-fix-*`）。

> **2026-08-12 v6 设计修订（已实现、部署并完成联调）**：
>
> 1. `status.ipMetaData.metadata` 从结构化 map 改为 JSON string；解码后的
>    逻辑形状不变（`map[主地址族IP]{ipv6,mac,vlan}`）。
> 2. `status.ipMetaData` 新增 provider 写入的 `observedGeneration`。IPAM 仅在
>    `observedGeneration == metadata.generation` 时允许分配；spec 更新到 provider
>    完成新一轮预热期间 fail closed，不新增 phase/condition。
> 3. agent 在 IPPool informer Add/Update（含纯 status Update）时对每个权威 metadata
>    修订只解析一次，按 pool UID + observedGeneration 保存不可变 map 快照；Pod 分配
>    直接读快照，不逐 Pod 解析 JSON。仅 allocatedIPs/resourceVersion 变化时复用缓存。
> 4. provider 必须在完整、可信地评估当前 generation 后，原子发布 metadata、两个计数及
>    observedGeneration。单 IP 失败属于有效完整结果（成功项进入 metadata，失败项缺席并
>    计入 unready）；只有 reconcile 中断、无法形成可信全量快照或 status 写入失败时不得推进。
>
> 64/1000 条目本地初测：1000 条模拟分配周期结构化 map 约 2.11ms，JSON string
> 无缓存约 5.29ms，JSON string + 缓存约 0.54ms。结论是 string 与解析缓存必须作为
> 一个整体实现。以下 v5 实测结果仍作为历史记录保留；v6 新增用例 #17-#23
> 已全部通过。

> **⚠️ 2026-08-11 v5 设计修订（待重新适配/重测）**：设计已按 proposal Draft v5 修订，
> 本文档以下内容中的旧 API 描述均已过时，涉及相关字段/注解的用例需在 v5 代码落地后
> 重新执行：
>
> 1. `status.iaasReadyIPs`/`status.iaasFailedIPs`/`status.conditions` **全部移除**，
>    替换为云中立的 `status.ipMetaData`：`parentNic`（池级父网卡）+ `metadata` map
>    （key = 主地址族地址，通常 IPv4；value 含 `ipv6`/`mac`/`vlan`；**有条目即就绪**，
>    失败 = 缺席）+ provider 写入的 `readyIPCount`/`unreadyIPCount` 两个观测计数。
> 2. 注解/label `ipam.spidernet.io/iaas-pool: "true"` 改为
>    `ipam.spidernet.io/iaas-provider: "<vendor>"`（当前支持 `huaweicloud`，
>    validating webhook 校验 vendor 白名单）。
> 3. 不再有任何 condition（`IaasReady` 等）。
>
> 模拟 provider 写台账的用例改为 patch `status.ipMetaData`（示例见
> `specs/006-iaas-prewarm-pool/quickstart.md` 第 3 节）。以下历史记录按当时的
> API 保留，不再逐条改写。

> **2026-08-11 v5 部署记录**：v5 修订代码（commit
> `8fada0841b405e7a89dfbac5235c85a320968206`）已按既有流程重新构建部署：增量
> git bundle（基于远端已有 `873806a8` 基点）同步代码 → `make build_image
> E2E_CHINA_IMAGE_REGISTRY=true`（基础镜像已缓存，约 4 分钟）→ 本地沙箱中转
> scp + 两节点 `ctr -n k8s.io images import` → 先 patch 清除 `iaas-t-v4` 旧
> status 字段（`iaasReadyIPs`/`iaasFailedIPs`/`conditions`）→ `kubectl apply`
> 更新 CRD（新 schema 仅含 `status.ipMetaData`）→ `kubectl set image` 滚动更新。
> 两节点 agent 2/2 Running、controller 1/1 Running，0 重启；parent-nics 注解
> 正常。冒烟验证：`iaas-provider=badvendor` 被 webhook 拒绝（"supported
> values: huaweicloud"）、`huaweicloud` 通过且 label 自动同步/移除。已手工清理
> `iaas-t-v4` 上旧版遗留 label `ipam.spidernet.io/iaas-pool`。本地
> `go build ./...` 与 `go test ./pkg/ipam/... ./pkg/ippoolmanager/...` 部署前
> 已通过。v5 相关用例待逐项重测。
>
> **2026-08-11 v5 全量重测记录（全部通过）**：provider 侧同步适配 v5 后完成
> 全用例重测（发现并修复一处契约不匹配：provider 曾在每个 metadata 条目写
> `parentNicName`，与 schema 的池级 `parentNic` 冲突，被 SSA typed patch 拒绝
> `field not declared in schema`；provider 修正为池级字段后 reconcile 自动收敛）。
>
> - **预热闭环**：切换 spec.ips 到全新段后 provider 预热并写 `ipMetaData`
>   （池级 `parentNic: enp11s0f0np0` + 每 IP `ipv6/mac/vlan=2014` +
>   `readyIPCount/unreadyIPCount`）；v6 配对池自身台账为空（符合"台账只在主池"）。
> - **#1-#6 webhook**：`iaas-provider` 注解↔label 双向同步；vendor 白名单拒绝
>   `alicloud`；自引用/同版本/容量超限/nodeName 不一致/podAffinity 不一致均被拒；
>   引用不存在的池允许创建。
> - **#8/#10 配对分配**：单栈声明 Pod 整对分得 v4+v6（同一 metadata 条目），
>   endpoint 记录 mac/vlan 与台账一致。
> - **#9 台账过滤**：暂停 provider 后手工将 .30 条目移除（unready），新 Pod 精确
>   分得唯一 ready 的 .31 配对，unready 地址从未被选中。
> - **#11/#13 耗尽 fallback**：置空 metadata 后，多候选池 Pod 的 v4 侧成功
>   fallback 到普通池，v6 配对侧报标准 `all IP addresses used out`，整体失败回滚；
>   普通池无 label/无配对校验，回归正常。注意：回滚后普通池 `allocatedIPs` 中的
>   复用记录（reuse allocation）需等 scanAll GC 周期（~10 分钟）兜底回收，
>   trace 路径因 endpoint 已被 cmdDel 清理而跳过——非泄漏，属预期 GC 行为。
> - **#12 零云调用**：Pod 分配全程 mockserver 0 新增日志行。
> - **#15 扩缩容回收**：缩容 spec.ips → provider `DELETE sub-network-interfaces`
>   回收云侧 sub-ENI 并同步移除台账条目；并观察到配对中间态保护：仅缩 v4 时
>   provider 报 `InvalidPair (v4=1 v6=2)` 挂起预热，v6 同步缩容后恢复——缩容
>   需两池同步操作。
> - **已知限制再次确认**：手工清台账后云侧残留 sub-ENI 导致 DriftDetected
>   （`0/N ready`），按既定方法切换全新地址段恢复。
> - 测试资源：保留 `iaas-t-v4`/`iaas-t-v6`（spec.ips 现为 .34/::34，台账 1/1
>   ready）与普通池 `iaas-t-plain`；测试 Pod 均已清理。

> **2026-08-10（第二次）部署记录**：新增 parent-nics 上报功能后按相同流程重新构建
> 部署（增量 bundle → 构建约 4 分钟 → 两节点 `ctr -n k8s.io images import` →
> `kubectl set image`）。额外变更：`kubectl patch clusterrole spiderpool-admin` 为
> `nodes` 资源追加 `patch/update` 权限（对应 chart role.yaml 变更）。集群
> `spiderpool-conf` 中 `iaasNetworkProvider.serverUrl` 已有值（huawei-mockserver），
> 功能自动启用。滚动更新后两节点 Node 注解 `ipam.spidernet.io/parent-nics` 均写入
> 物理网卡 name→MAC map（.50 九块 / .60 八块，无 veth/bridge/cali* 虚拟接口），
> 用例 #16 通过。
> 踩坑记录：`ssh host "... :$0" TAG` 远端 `$0` 展开为 `bash` 导致 set image 打了
> `:bash` 错误 tag（ImagePullBackOff），远端命令内应使用单引号+变量赋值。

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
| 8 | 单栈 Pod 从带 `pair-pool` 的池请求 IP：自动补全配对池候选，但不强制分配对侧地址 | FR-004, FR-005 | 创建仅声明 v4 池的 Pod，检查其只获得 v4 地址，v6 侧地址仍未分配 | **通过**（2026-08-11：Pod 仅声明 `{"ipv4":["iaas-t-v4"]}`，实际按"整对分配"获得 v4+v6 双地址（net1 同时配置 192.168.100.10 与 fd00:100::10），endpoint 记录 mac/vlanID 来自同一台账条目——配对池强制整对分配，符合最终实现语义） |
| 9 | 台账过滤：不在 `iaasReadyIPs` 中的地址（即使在 `spec.ips` 内，含 `iaasFailedIPs` 条目）不可被选中 | FR-009 | 构造 `iaasFailedIPs` 含条目、`iaasReadyIPs` 不含该地址的台账，创建 Pod，确认从不分配该地址 | **通过**（2026-08-11：构造台账 ready 仅 .12、failed 含 .10/.11（均在 spec.ips 内），新 Pod 精确分得 .12/fd00:100::12，failed 与不在台账的地址从未被选中） |
| 10 | 双栈 Pod 原子成对分配：v4/v6 必须来自同一台账条目，不可交叉 | FR-010 | 创建双栈 Pod，检查 `status.allocatedIPs`/Pod 注解中 v4、v6 地址确实成对（同一条目） | **通过**（2026-08-11：双栈声明 Pod 分得 192.168.100.10 + fd00:100::10，mac=fa:16:3e:41:62:33、vlan=2014 与台账条目完全一致，无交叉配对） |
| 11 | 台账已耗尽（无 ready 且未占用条目）时，走标准“无可用 IP”失败并允许多池 fallback | FR-013 | 耗尽台账后创建 Pod（声明多个候选池），确认该池报无可用 IP 但整体调度可 fallback 到其他池而非卡死 | **通过**（2026-08-11：置空 iaasReadyIPs 后创建 Pod（候选 ["iaas-t-v4","iaas-t-plain"]），agent 报标准 `all IP addresses used out`（v6 配对侧），v4 侧成功 fallback 到普通池；失败后正确回滚、无 IP 泄漏、无 endpoint 残留；池 condition 转为 `IaasReady=False PartialPrewarmFailed 0/3 ready`） |
| 12 | 台账驱动的分配跳过原有同步云 API 调用路径 | FR-012, FR-015 | 结合 provider/mock 观察：从 ready 台账条目分配时不应触发实时云 API 调用（可通过 mockserver 请求日志或 provider 日志验证无新调用） | **通过**（2026-08-11：以 mockserver 访问日志行数为基准，Pod 创建/就绪全程 mockserver 0 次新请求，分配纯走台账） |
| 13 | 回归：不带 `iaas-pool` 注解的存量池行为完全不变 | FR-006, FR-011 | 对集群中已有的普通池（如 `abc`）执行常规分配，确认无 label 被添加、无配对校验触发、分配结果与升级前一致 | **通过**（2026-08-11：普通池 `iaas-t-plain` 无 label/无配对校验（8-10 已验），并在 #11 中作为 fallback 池实际参与 Pod 分配路径，行为正常） |
| 14 | Webhook/Controller 升级后原有 e2e / 存量工作负载不受影响的冒烟检查 | 兼容性 | 观察 `spiderpool-agent`/`spiderpool-controller` Pod 升级后状态、日志无异常，抽查若干已有 Pod 网络正常 | **通过**（2026-08-06：两节点 agent 2/2 Running、controller 1/1 Running，0 次重启；日志无异常，webhook mutating/validating 正常工作；观察到的唯一 ERROR 是 `macvlan1.enp-pool` 历史遗留 subnet 校验问题，与本次改动无关） |
| 15 | 与 provider 组件联调：provider 真实写入台账 → Spiderpool 分配 → 云侧（mock）状态一致性 | 端到端 | provider 组件就绪后，创建声明 podAffinity 匹配的工作负载，观察 provider 日志/mockserver 记录的绑定操作与 Spiderpool 分配结果一致 | **通过**（2026-08-11：provider+mockserver 就绪后端到端验证——扩容 spec.ips → provider POST sub-network-interfaces 预热并写入 iaasReadyIPs（含 mac/vlanID=2014）；缩容 → provider DELETE 回收并移除条目；Pod 分配的 ipv4/ipv6/mac/vlan 与云侧 sub-ENI 一致；condition AllReady/PartialPrewarmFailed 切换正确） |
| 16 | agent 启动时上报 parent-nics：`iaasNetworkProvider.serverUrl` 配置后，Node 注解 `ipam.spidernet.io/parent-nics` 写入本机物理网卡 `名称: MAC` map；`excludeReportNics` 生效；无物理网卡（如 kind 虚拟网卡环境）时如实上报空注解 `{}`（不 Fatal），可手动维护注解；未配置 serverUrl 则不写注解 | 提案 §parent port 解析 | 在 ConfigMap 中配置 `iaasNetworkProvider.serverUrl` 后重启 agent，`kubectl get node -o jsonpath='{.metadata.annotations.ipam\.spidernet\.io/parent-nics}'` 检查两节点注解内容（应只含物理网卡，不含 veth/bridge/cali*）；配置 `excludeReportNics` 排除管理口后重启验证被排除 | **通过**（2026-08-10：部署 `873806a8f` 后两节点注解均写入，10-20-1-50 含 9 块物理网卡、10-20-1-60 含 8 块（含 ibp8s0 IB 卡），均为 name→MAC map，无虚拟接口混入；agent 日志有 "Reported parent NICs" 记录；`excludeReportNics` 生效性与空网卡上报空注解行为已由单元测试覆盖，环境侧暂未单独演练） |
| 17 | generation 一致性门控：spec 更新后、新 status 发布前拒绝分配 | v6 FR-016 | 记录 generation/observedGeneration，修改 spec.ips 后立即创建 Pod，确认 mismatch 时返回可重试错误；provider 完成并推进 observedGeneration 后 Pod 自动重试成功 | **通过**（2026-08-12：暂停 provider 后将 `iaas-rate-limit-8` 扩到 9 个 IP，观测 `generation=2/observedGeneration=1`；Pod 明确报 `IaaS IP metadata not ready` 且 allocated=0。恢复 provider 后原子发布 9/0、observed=2，同一 Pod 经 kubelet 重试 Ready） |
| 18 | metadata JSON 缓存：每修订只解析一次，allocatedIPs 更新不触发重解析 | v6 FR-017 | 通过单测/计数日志/benchmark 连续创建 Pod；验证相同 metadata+observedGeneration 下复用不可变快照，纯 status.allocatedIPs Update 不重解析 | **通过**（2026-08-12：单测覆盖 allocatedIPs-only 复用和 status-only 原子替换；agent DS 重启后 informer replay 重建缓存，无需修改 status 即可从池成功分配；64/1000 benchmark 的 cache hit 分别约 110ns/80ns） |
| 19 | 缓存 fail closed：畸形 JSON、缓存缺失、版本不匹配均不得使用旧台账 | v6 FR-009/FR-017 | 分别注入畸形 JSON、清空缓存模拟 agent 重启窗口、构造 observedGeneration mismatch；确认均拒绝分配且不回退普通 IaaS 分配路径 | **通过**（2026-08-12：同 generation 注入 `metadata: "{"` 后 Pod 明确报 malformed JSON、allocated=0；generation mismatch 同样拒绝；cache miss 由单测覆盖。清空坏 status 并重启 provider 后，通过纯 status Update 恢复有效快照，Pod 自动重试成功） |
| 20 | 大池性能回归：64/1000 条 metadata 与 1:1 Pod 重建 | v6 性能预算 | benchmark 结构化基线/string 无缓存/string 有缓存；测试环境执行两个节点池容量=Pod 数量的多轮 Recreate，确认全 Ready、地址/VLAN 唯一且无逐 Pod JSON 解析 | **通过**（2026-08-12：本地 64/1000 benchmark 覆盖 provider build、DeepCopy、outer marshal/unmarshal、cache hit 与无缓存 decode；测试集群两个 8-IP 双栈池执行 5 轮 Recreate，每轮 16/16 Ready、v4 allocated 8+8、32 个双栈地址/16 MAC/16 VLAN 全部唯一） |
| 21 | provider 单 IP 预热失败仍发布可信完整结果 | v6 provider contract | 对新增的一个地址注入一次 `create_subeni` partial 故障，确认成功项保留、失败项缺席且计入 unready，并推进 observedGeneration | **通过**（2026-08-12：`iaas-rate-limit-8` 从 8 扩到 9 个地址，mock 对 `.98` 返回一次 500；provider 发布 `generation=4/observedGeneration=4`、ready=8、unready=1、metadata=8 条，失败地址缺席。测试后已恢复为 8/0、generation/observedGeneration=5/5） |
| 22 | v6 双栈批量分配精确配对且不触发同步云调用 | v6 FR-010/FR-015 | 记录 mock 请求游标后重建两个节点上的 16 个 Pod，逐 Pod 对比 network-status 与权威 metadata，并检查新增云请求 | **通过**（2026-08-12：16/16 Pod Ready；16 组 IPv4/IPv6/MAC 全部与对应 metadata 条目一致且各自唯一。重建窗口 `/__requests` 无任何云 API 请求，新增记录仅为测试结束时读取请求历史的 `GET /__requests`） |
| 23 | 双节点 200-Pod 大规模冷启动和滚动更新 | v6 性能/稳定性 | 每节点准备 200 组双栈 IP、并发启动 100 Pod；分别执行 3 轮 hostNetwork 和预热网络冷启动，再执行 5 轮 Pod template RollingUpdate | **通过**（2026-08-12：T1 hostNetwork 平均 11.801s，T2 预热双栈网络平均 10.984s，RollingUpdate 平均 25.624s；所有轮次 200/200 Ready，最终 200 组 IPv4/IPv6/MAC 全部精确匹配且唯一，分配及更新期间云 API 调用为 0） |
| 24 | 非预热 IaaS 实时创建模式规模对比 | 传统同步 provider 路径 | 两节点各使用一个无 `iaas-provider` 注解的 200-IP 普通 IPv4 节点池，每节点并发启动 100 Pod；验证每 Pod 同步创建/释放 sub-ENI，并执行冷启动及 RollingUpdate | **部分通过/发现问题**（2026-08-12：3 轮冷启动均最终 200/200 Ready，平均 101.814s，每轮精确产生 200 次云端创建，Pod IP/MAC 与 mock sub-ENI 200/200 匹配；首轮 RollingUpdate 161.183s 后 200/200 Ready，但云侧仅 196 条与当前 Pod 匹配，4 个 Running Pod 对应 sub-ENI 已被旧 Pod 延迟释放流程误删。因发现一致性问题停止后续轮次） |
| 25 | 地址族透传：单栈 v4 / 单栈 v6 / 双栈 Pod 的实时创建与释放 | 非预热同步路径 | 分别用仅 v4 池、仅 v6 池、双栈 SMC 创建 Pod，观察 allocate item 地址族、MAC/VLAN 回填与释放级联 | **v4/双栈通过，v6 阻塞于 provider**（2026-08-14：v4-only 与双栈端到端通过；v6-only spiderpool 透传正确，provider 500 `cache miss: subnet not found`——其子网缓存仅索引 IPv4 CIDR，待 provider 补 v6 子网索引） |

### 2026-08-12 v6 联调发现

- provider 镜像 `controller:v6-metadata-b3a5b0a2` 能正确发布 JSON string、
  `observedGeneration` 和 ready/unready 计数。
- 若外部或故障注入将 `status.ipMetaData.metadata` 写成畸形 JSON，provider 的普通
  reconcile 和启动全量 reconcile 都会先解码旧 status 并失败；仅重启无法自愈。
  测试中必须先将 `status.ipMetaData` 清空，再重启 provider 才能重建权威快照。
  provider 后续应在旧 status 无法解码时从 spec + 云侧库存重建完整快照，而不是让
  非权威旧输出阻断 reconcile。
- mockserver 的故障管理接口为 `/__faults`；本次使用
  `{"target":"create_subeni","type":"partial","remaining":1}` 精确触发一次创建失败。
  provider 正确把单 IP 失败作为完整评估结果发布，并推进 `observedGeneration`。
- 以 `/__requests` 的记录序号为边界重建 16 个双栈 Pod，分配窗口没有产生任何云 API
  请求，证明 Spiderpool 分配直接消费 agent informer 缓存，不依赖同步 provider 调用。

### 2026-08-12 双节点 200-Pod 规模测试

**环境与计时方法**：

- 两节点各创建一个 200 条目的主 IPv4 池及其 200 条目 IPv6 配对池：
  `iaas-scale-50-v4/v6` 使用 `192.168.110.0/24` + `fd00:110::/112`，
  `iaas-scale-60-v4/v6` 使用 `192.168.120.0/24` + `fd00:120::/112`。
  provider 在默认 `rateLimit.qps=2` 下按池串行预热，两个池各自耗时约
  297.364s 和 300.876s，最终均为 ready=200、unready=0、
  `observedGeneration=1`。预热耗时不计入 Pod 启动 T2。
- 每节点并发启动 100 个 Pod，总计 200 个。因节点原 `maxPods=110` 且已有系统
  工作负载，测试前经确认将两节点 kubelet `maxPods` 调整为 250 并逐节点重启；
  原配置备份为 `/var/lib/kubelet/config.yaml.copilot-scale-20260812`。
- 测试镜像为两节点已缓存的 `docker.m.daocloud.io/library/busybox:latest`，
  `imagePullPolicy=IfNotPresent`，因此结果衡量 Pod 编排、容器启动及 CNI/IPAM
  路径，不包含不稳定的镜像下载时间。
- 冷启动每轮先缩容到 0，等待全部 Pod 删除；IaaS 场景额外等待两个池
  `allocatedIPCount` 均归零，再同时将两个 Deployment 扩到各 100。
  T1/T2 从发起两个 scale API 请求前开始，到两个 Deployment 均达到
  ready/updated/available=100 为止。
- 滚动更新使用 `RollingUpdate`（`maxSurge=25%`、`maxUnavailable=25%`），每轮修改
  Pod template 中的环境变量模拟客户修改镜像或其他 Pod spec 字段；从 patch 前开始，
  到两个 Deployment 新 revision 均为 ready/updated/available=100 为止。

| 场景 | 轮次 | 10-20-1-50 | 10-20-1-60 | 两节点全部完成 |
|------|------|-------------|-------------|----------------|
| hostNetwork 冷启动（T1） | 1 | 9.849s | 11.653s | 11.653s |
| hostNetwork 冷启动（T1） | 2 | 10.261s | 11.635s | 11.635s |
| hostNetwork 冷启动（T1） | 3 | 10.306s | 12.115s | 12.115s |
| **T1 平均** | **3 轮** | **10.139s** | **11.801s** | **11.801s** |
| 预热双栈网络冷启动（T2） | 1 | 9.647s | 11.472s | 11.472s |
| 预热双栈网络冷启动（T2） | 2 | 8.705s | 10.728s | 10.728s |
| 预热双栈网络冷启动（T2） | 3 | 8.923s | 10.752s | 10.752s |
| **T2 平均** | **3 轮** | **9.092s** | **10.984s** | **10.984s** |
| 预热双栈网络 RollingUpdate | 1 | 25.718s | 25.189s | 25.718s |
| 预热双栈网络 RollingUpdate | 2 | 24.115s | 25.271s | 25.271s |
| 预热双栈网络 RollingUpdate | 3 | 25.309s | 26.470s | 26.470s |
| 预热双栈网络 RollingUpdate | 4 | 24.294s | 25.514s | 25.514s |
| 预热双栈网络 RollingUpdate | 5 | 25.146s | 25.146s | 25.146s |
| **RollingUpdate 平均** | **5 轮** | **24.916s** | **25.518s** | **25.624s** |

**结果**：

- T2 比 T1 平均少 0.817s（约 6.9%），两者处于同一量级；在本环境中，
  informer metadata cache + Spiderpool/VLAN CNI 没有表现出可测的额外冷启动开销。
- 每一轮均为 200/200 Pod Ready 且 network-status 含双栈地址。最终轮逐 Pod 对账为
  200/200 IPv4、IPv6、MAC 与权威 metadata 精确匹配，三者均 200 个唯一值。
- 最后一轮单 Pod 从 creationTimestamp 到 Ready 的延迟：节点 50 平均 1.66s、
  p50=2s、p95=3s；节点 60 平均 1.59s、p50=2s、p95=2s。约 25.6s 的整轮更新时间
  主要来自 RollingUpdate 的分批替换策略，而不是单 Pod CNI 延迟。
- 三轮 T2 和五轮 RollingUpdate 的 mock 请求窗口均无云 API 请求；agent 日志和
  Kubernetes Warning Events 中无对应错误。
- 测试结束后四个 benchmark Deployment 均缩容为 0，两个主池分配计数均归零；
  200-IP 预热池、NAD、Deployment 及 kubelet `maxPods=250` 保留，便于后续快速复测。

### 2026-08-14 四组网络 200-Pod 对比测试（hostNetwork / Calico / macvlan / 预热）

**目的与方法**：在同一集群、同一批次内，对四种网络路径做一致口径的规模对比：
hostNetwork（基线）→ Calico（集群默认 CNI）→ 非 IaaS 普通池 macvlan → IaaS 预热
双栈池。每组每节点 200 Pod（两节点共 400），镜像为已缓存的 busybox +
`imagePullPolicy=IfNotPresent`，`terminationGracePeriodSeconds=0`，`nodeName` 直绑。
每组 3 轮冷启动（缩 0 →等 Pod 全删、Spiderpool 池 allocatedIPCount 归零→ 同时扩到
2×200，计时到两个 Deployment ready/updated/available=200）+ 3 轮滚动重启（patch Pod
template 注解，`RollingUpdate maxSurge=25%/maxUnavailable=25%`，计时到新 revision
全就绪）。另为 macvlan/预热两组补测 3 轮 `maxSurge=0/maxUnavailable=25%` 的重启对照。
所有轮次同时记录 mockserver `/__requests` 计数（预热组用于确认无云 API 调用）。

**新增/复用资源**：

- 复用：`scale-host-50/60`（hostNetwork）、`iaas-scale-50/60` + 200 条目双栈预热池
  `iaas-scale-{50,60}-v4/v6`（ready=200、unready=0）。
- 新建：`calico-scale-50/60`（无 multus 注解，走集群默认 Calico）；
  普通（非 IaaS）双栈池 `mv-scale-50-v4/v6`（`192.168.150.0/24`+`fd00:150::/112`）、
  `mv-scale-60-v4/v6`（`192.168.160.0/24`+`fd00:160::/112`）各 200 IP，NAD
  `mv-scale-{50,60}-net`（macvlan bridge on `enp11s0f0np0` + coordinator，
  Deployment `mv-scale-50/60`）。资源清单备份在 .50 `/root/bench-setup-20260814.yaml`，
  跑分脚本 `/root/bench-run.sh`、`/root/bench-surge0.sh`，原始日志
  `/root/bench-20260814-results.log`。

| 场景 | 冷启动 3 轮（两节点全就绪） | 冷平均 | 重启 surge=25% 3 轮 | 重启平均 | 重启 surge=0 3 轮 | surge=0 平均 |
|------|------------------------------|--------|----------------------|----------|--------------------|---------------|
| hostNetwork | 43.47 / 22.20 / 22.37s | **29.35s**¹ | 51.10 / 51.52 / 51.29s | **51.30s** | 未测 | — |
| Calico | 26.63 / 35.42 / 35.33s | **32.46s** | 49.10 / 49.08 / 49.01s | **49.06s** | 未测 | — |
| macvlan 普通池 | 24.52 / 22.48 / 22.85s² | **23.29s** | 210.79 / 216.63 / 219.31s | **215.58s** | 219.10 / 208.19 / 216.42s | **214.57s** |
| IaaS 预热双栈 | 22.29 / 22.30 / 22.40s | **22.33s** | 60.26 / 67.06 / 62.50s | **63.27s** | 62.46 / 60.14 / 62.48s | **61.70s** |

¹ hostNetwork 第 1 轮 43.5s 为该批次首轮（含 kubelet 缓存冷态），其余两轮 ~22s。
² macvlan 原第 1 轮（595.9s）作废：v6 池 `fd00::10-::d1` 按十六进制只有 194 个地址
  （不足 200），200 Pod 中 6 个卡 `all IPv6 used out`；已把上界改为 `::d7`（恰 200 个）
  并补测一轮干净冷启动 22.85s 计入。

**结论**：

1. **冷启动四组同量级（22-33s）**：预热双栈组（22.33s）与 hostNetwork/macvlan 持平
   甚至最快、比 Calico 略快，重申预热台账路径无可测冷启动开销；且全部预热轮次
   mock `/__requests` 增量为 0（表中逐轮 +1 均为测试脚本自身的读计数 GET）。
2. **1:1 池容量下重启差异显著**：macvlan 普通池重启 ~215s，预热池 ~62s（约 3.5 倍差距），
   Calico/hostNetwork ~50s。macvlan 与预热组均出现 `all IP addresses used out`
   的 kubelet 重试（旧 Pod 释放 IP 与新 Pod 抢建的竞争），但重试规模差 ~7 倍：
   事件计数 mv 组 12,694 次 vs 预热组 1,878 次——预热池 Pod DEL 只释放内部 claim、
   台账分配直接走 informer 缓存，IP 回收→再分配收敛显著更快。
3. **`maxSurge=0` 对照与推理不符**：两组 surge=0 与 surge=25% 耗时基本相同
   （215 vs 216s、62 vs 63s）。原因是 Deployment 控制器在 surge=0 时同样按
   maxUnavailable 批量先删后建，新 Pod 创建仍与旧 Pod 的 CNI DEL/IP 释放窗口重叠，
   IP 竞争未消除。真正有效的做法是**给池预留 headroom**（容量 ≥ replicas + surge 量），
   1:1 硬约束场景则建议接受重启期的重试收敛（预热模式 ~60s 可接受，普通池 ~215s
   偏慢，主因是 kubelet FailedCreatePodSandBox 的指数退避）。
4. **环境修正记录**：(a) 两节点 kubelet `maxPods` 由 250 上调至 400 并重启
   （`nodeName` 直绑跳过调度器，重启批次的新 Pod 与 Terminating 旧 Pod 叠加超过 250
   触发 kubelet `OutOfpods` 拒绝风暴，产生数千 Failed Pod 记录，已全部清理；原配置
   备份 `/var/lib/kubelet/config.yaml.copilot-scale-20260814`）。(b) coordinator
   `mode: auto` 对双栈 Pod 需宿主机具备覆盖 Pod v6 网段的路由（`GetGatewayIP` 走
   `RouteGet`），已在 .50/.60 分别补
   `ip -6 route add fd00:150::/112 dev enp11s0f0np0`、`fd00:160::/112 dev enp11s0f0np0`
   （与既有 2026-08-13 记录同款问题）。(c) 测试后四组 Deployment 均缩 0、
   八个池 allocated 归零、RollingUpdate 策略还原为 25%/25%，资源保留供复测。
5. **清理记录（2026-08-17）**：本轮及后续 td 场景的全部测试资源已删除
   （calico-scale/mv-scale/td-a/td-b Deployment 与 NAD、mv/td 全部池、
   iaas-scale-50/60 预热池——为释放 mock 父网卡 256 sub-ENI 上限容量删除，
   备份 `/root/iaas-scale-pools-backup-20260814.yaml`），两节点 kubelet
   `maxPods` 已还原 250。**发现 provider 缺陷**：孤儿配对 v6 池的 finalizer
   不回收——若 v4 主池先完成删除（其清理已释放全部双栈 sub-ENI），随后/同时
   删除的 v6 从池不再被 reconcile，`ipam.spidernet.io/iaas-cleanup` finalizer
   永久残留（iaas-scale-50/60-v6 卡 3 天，注解触发无效，手动摘除后删除完成；
   同批 td 的 v6 从池因主池尚在而正常删除）。provider 需对"pair-pool 指向的
   主池已不存在"的删除中从池直接摘 finalizer。另外池删除清理为全局串行队列，
   200 IP 双栈池约需 508s（qps=2），多池并发删除时排队显著。

### 2026-08-12 非预热 IaaS 实时创建模式规模测试

**配置与路径确认**：

- 创建两个无 `ipam.spidernet.io/iaas-provider`、无配对注解的普通 IPv4 节点池：
  `iaas-realtime-50-v4`（`192.168.130.10-209`）和
  `iaas-realtime-60-v4`（`192.168.140.10-209`），容量均为 200。
- 对应 SpiderMultusConfig 使用 `vlanMode: auto`。该配置使普通池分配结果走
  Spiderpool agent 的同步 `/ipam/allocate-ips` provider 调用；Pod 删除时走同步
  `/ipam/release-ip`。传统 provider 模式当前只支持 IPv4，因此未创建 IPv6 配对池。
- 测试前发现 `spiderpool-conf` 的 `iaasNetworkProvider.serverUrl` 指向
  `huawei-mockserver`，而非 provider Service。该错误不影响预热测试（预热分配明确
  跳过同步调用），但会阻断传统模式；已修正为
  `https://iaas-network-provider.iaas-network-provider-system.svc` 并滚动重启
  agent/controller。
- 两个单 Pod 冒烟分别触发一次 mock `POST sub-network-interfaces`，总就绪耗时
  6.963s；缩容后两个 sub-ENI 均经 DELETE 回收，耗时 3.794s。

| 场景 | 轮次 | 10-20-1-50 | 10-20-1-60 | 两节点全部完成 | 云端 CREATE |
|------|------|-------------|-------------|----------------|-------------|
| 非预热实时创建冷启动 | 1 | 99.253s | 99.971s | 99.971s | 200 |
| 非预热实时创建冷启动 | 2 | 104.109s | 93.431s | 104.109s | 200 |
| 非预热实时创建冷启动 | 3 | 101.363s | 101.363s | 101.363s | 200 |
| **冷启动平均** | **3 轮** | **101.575s** | **98.255s** | **101.814s** | **200/轮** |
| 非预热实时创建 RollingUpdate | 1 | 161.183s | 149.037s | 161.183s | 196（请求成功 200） |

**结果与发现**：

- 非预热冷启动平均 101.814s，是预热 T2（10.984s）的约 **9.27 倍**。
  三轮均最终 200/200 Ready；每轮 mock 精确收到 200 次创建。第三轮完成后逐 Pod
  对账为 200 个 Pod、200 个 cloud sub-ENI，IP 和 MAC **200/200 匹配**。
- provider 默认 `rateLimit.qps=2`、`burst=10`、队列超时 30s。三轮冷启动和冒烟期间，
  provider 成功处理 602 个 allocate 请求，同时返回 894 次 HTTP 429；成功请求平均
  耗时 25.886s、p95 34.103s、最大 34.879s。kubelet/CNI 重试使工作负载最终收敛，
  但大量 429 是实时模式约 100s 启动耗时的主要来源。
- RollingUpdate 使用 `maxSurge=25%`、`maxUnavailable=25%` 并修改 Pod template。
  新版本 200/200 Ready 用时 161.183s；provider allocate 成功 200、429 共 418，
  release 接受（202）132、release 429 共 73。
- **一致性缺陷**：RollingUpdate 完成后池内 allocatedIPCount 已回到 100+100，
  但一次检查中 cloud 尚有 231 条、Endpoint 231 条，说明旧 Pod 清理明显滞后；
  后续收敛到 200 个当前 Endpoint/200 个 Running Pod 时，cloud 仅剩 196 条。
  逐 Pod 对账确认 4 个 Running Pod 地址（`.130.55`、`.130.76`、`.140.50`、
  `.140.68`）在 mock cloud 中缺失，而 Pod 仍保留 provider 返回的 MAC。
  这表明旧 Pod 的延迟 release 可能在地址已被新 Pod 复用后误删新 sub-ENI。
  该问题不出现在预热模式，因为预热池跨 Pod 生命周期保留云侧 sub-ENI，Pod DEL
  只释放 Spiderpool 内部 claim。
- 发现问题后停止第 2/3 轮实时 RollingUpdate。按用户要求未继续等待或清理测试环境。

## 5. 环境相关注意事项

### 2026-08-11 全量功能测试记录（provider + mockserver 就绪后）

- **测试配置**：NAD `spiderpool/iaas-test-vlan`（cniType=vlan、vlanMode=auto、
  `master: enp5s0f0np0` 真实物理网卡）；池 `iaas-t-v4`/`iaas-t-v6` 最终 spec.ips
  为 `.20-.21`/`::20-::21`（云侧全新段），vlanID=2014，nodeName `10-20-1-50`，
  podAffinity `app: iaas-t`。按用户要求只验证功能，不验证数据面联通性。
- **预热闭环已验证**：扩容 spec.ips → provider 调 mock 云 API
  （POST /vpc/sub-network-interfaces）预热并写 `iaasReadyIPs`；缩容 → DELETE 回收
  并移除台账条目；两侧级联 reconcile 收敛正常。
- **已知限制（用户提示，已实测确认）**：同一父网卡不能同时存在两个相同 vlanID
  子接口——同节点第二个使用同 vlanID 的 Pod 卡在 ContainerCreating，CNI 报
  `plugin type="vlan" failed (add): failed to create vlan: file exists`。
  该失败发生在 vlan CNI 调 IPAM **之前**（agent 无 ADD 日志），因此无 IP 泄漏。
  测试期间所有 Pod 用例均串行（同一时刻单 Pod）。
- **发现/待确认**：Pod `net1`（vlan 子接口）实际 MAC 继承父卡
  （`e8:eb:d3:93:ae:10`），而台账/云侧 sub-ENI MAC 为 `fa:16:3e:*`。mock 环境无影响，
  但真实华为云上"流量源 MAC ≠ sub-ENI MAC"可能被云侧防欺骗机制丢包，
  需确认 CNI 是否应把 sub-ENI MAC 设置到子接口上。
- **provider 行为注意**：`iaasFailedIPs` 不自动重试（符合设计）。手动清台账重
  预热时，若云侧已有同地址存量 sub-ENI 会持续判 failed（`0/N ready`）；本次通过
  把 spec.ips 切换到全新地址段恢复 `AllReady`。真实运维中需先清理云侧残留。
- **provider 就绪探针**：provider 启动后到 leader 选举完成前 `/ready` 返回 503
  （约 70s），属正常启动过程。

### 历史注意事项

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

### 2026-08-17 US4 全局池模式 SC-009..SC-012 单测交叉核对

针对 spec.md 成功标准的实现级核对（单元测试层面，e2e 待 provider 适配后补充）：

- **SC-009（本地缓存命中零 RPC）**：`pkg/ippoolmanager` 单测验证
  `node == localNode && vlan != -1` 的条目直接命中并返回缓存的
  `{ipv6, mac, vlan}`，`fromIPMetadata == true`（`pkg/ipam` 现有
  `filterNonPrewarmedResults` 逻辑保证不触发 provider Allocate 调用）；
  其他节点条目、未绑定条目、`vlan == -1` 的 detaching 条目均不命中。✅
- **SC-010（RPC 失败回滚）**：`pkg/ipam/rollback_test.go` 验证
  `rollbackGlobalPoolClaims` 仅回滚全局池 claim（含双栈两侧、裸 IP 格式、
  release 失败时留给 GC 而不进 failure cache），节点级/静态池 claim 保留
  原有 failure-cache 重试行为。✅
- **SC-011（节点级池行为逐字节不变）**：Phase 2-6 既有全部单测未改动即通过
  （无 `scope` 的 metadata 一律 fail closed 拒绝；非 IaaS 池
  仍走 count=1 快路径）。✅
- **SC-012（粘性 v4/v6 配对）**：`FindGlobalColdPathIPv6` 单测验证冷路径
  v6 候选排除一切已被 metadata `entry.ipv6` 引用的地址（未被占用也排除）；
  `AllocateIPPair` 全局命中用例验证命中时两族来自同一条目。✅

## 6. 后续步骤

1. ~~将本分支代码上传/同步到 `10.20.1.50`~~ ✅ 已完成。
2. ~~在 `10.20.1.50` 上构建 agent/controller 镜像~~ ✅ 已完成
   （commit `6a29ebf2b55945b835750bb877512930e87b702c`）。
3. ~~直接 `kubectl apply` CRD、核对 ConfigMap、`kubectl set image` 替换镜像~~
   ✅ 已完成，两节点 rollout 成功。
4. 按第 4 节用例表逐项执行用例 #1-#11、#13（Spiderpool 侧独立可测，无需 provider），
   并在本文件中更新每项的“状态”列（通过/失败/阻塞及原因）。
5. provider 组件适配本特性并部署完成后，补充执行依赖它的用例（#12、#15）。
