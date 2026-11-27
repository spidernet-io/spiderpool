# 发布流程

[**English**](./release.md) | **简体中文**

## 前置步骤

* 更新 'charts/*/Chart.yaml' 中的 'version' 和 'appVersion' 字段
* 更新 '/VERSION' 中的版本号
* 在正确的分支上打版本 tag，版本号格式如下：
  * v0.1.0-rc0
  * v0.1.0-rc1
  * v0.1.0
  * v0.1.1
  * v0.1.2
  * v0.2.0-rc0
  * v0.2.0
* 更新 [roadmap](./roadmap.md)

## 推送版本 tag

推送 tag vx.x.x 后，以下步骤会自动执行：

1. 检查 tag 名称与 <https://github.com/spidernet-io/spiderpool/blob/main/VERSION> 中的版本一致

2. 创建名为 'release-vx.x.x' 的分支

3. 使用推送的 tag 构建镜像，并推送到 ghcr 镜像仓库 <https://github.com/orgs/spidernet-io/packages?repo_name=spiderpool>

4. 根据带有 "pr/release/*" 标签的历史 PR 生成 changelog

    将 changelog 文件提交到 'github_pages' 分支的 'changelogs' 目录，对应的 PR 带有 "pr/release/robot_update_githubpage" 标签。

    changelog 根据历史 PR 的标签生成：

    带有 "release/feature-new" 标签的 PR 归类到 "New Features"

    带有 "release/feature-changed" 标签的 PR 归类到 "Changed Features"

    带有 "release/bug" 标签的 PR 归类到 "Fixes"

5. 使用推送的 tag 构建 chart 包，并向 'github_pages' 分支提交 PR

    可以通过命令 `helm repo add spiderpool https://spidernet-io.github.io/spiderpool` 获取 chart

6. 将 '/docs' 提交到 'github_pages' 分支的 '/docs'

7. 创建 GitHub Release，附带 chart 包和 changelog

8. 最后，需要手动批准带有 "pr/release/robot_update_githubpage" 标签的 chart PR 和 changelog PR

详细信息请参考 <https://github.com/spidernet-io/spiderpool/blob/main/.github/workflows/auto-version-release.yaml>

## 发布后

1. 向文档站点提交版本更新的 issue --> <https://github.com/DaoCloud/DaoCloud-docs>
