# 贡献指南

[**English**](./contributing.md) | **简体中文**

***

## 搭建集群并运行 E2E 测试

1. 检查本地主机上所需的开发工具。如有缺失，请运行 'test/scripts/install-tools.sh' 进行安装

        # make dev-doctor
        go version go1.17 linux/amd64
        check e2e tools 
        pass   'docker' installed
        pass   'kubectl' installed
        pass   'kind' installed
        pass   'p2ctl' installed
        finish checking e2e tools

2. 构建本地镜像

        # do some coding

        $ git add .
        $ git commit -s -m 'message'

        # !!! images is built by commit sha, so make sure the commit is submit locally
        $ make build_image
        # or (if buildx fail to pull images)
        $ make build_docker_image

3. 搭建集群并运行 E2E 测试

    不同的测试有不同的场景，共有四种场景，分别对应不同的集群搭建命令和测试命令

    | 目标                                                                    | 搭建集群的命令                                        | 运行 E2E 测试的命令                                   |
    |-------------------------------------------------------------------------|------------------------------------------------------|------------------------------------------------------|
    | 测试 spiderpool                                                         | make    e2e_init_spiderpool                          | make e2e_test_spiderpool                             |
    | 测试 calico 与 spiderpool 组成的双 CNI 集群                              | make    e2e_init_calico                              | make e2e_test_calico                                 |
    | 测试 cilium 与 spiderpool 组成的双 CNI 集群                              | make    e2e_init_cilium_legacyservice                | make e2e_test_cilium_legacyservice                   |
    | 测试 cilium（eBPF service，无 kube-proxy）与 spiderpool 组成的双 CNI 集群 | make    e2e_init_cilium_ebpfservice                  | make e2e_test_cilium_ebpfservice                     |

    如果您在中国，可以添加 `-e E2E_CHINA_IMAGE_REGISTRY=true` 从国内镜像仓库拉取镜像，添加 `-e HTTP_PROXY=http://${ADDR}` 获取 chart

    搭建集群的示例：

        # setup the kind cluster of dual-stack
        # !!! images is tested by commit sha, so make sure the commit is submit locally
        $ make e2e_init_spiderpool
          .......
          -----------------------------------------------------------------------------------------------------
             succeeded to setup cluster spider
             you could use following command to access the cluster
                export KUBECONFIG=$(pwd)/test/.cluster/spider/.kube/config
                kubectl get nodes
          -----------------------------------------------------------------------------------------------------

        # setup the kind cluster of ipv4-only
        $ make e2e_init_spiderpool -e E2E_IP_FAMILY=ipv4

        # setup the kind cluster of ipv6-only
        $ make e2e_init_spiderpool -e E2E_IP_FAMILY=ipv6

        # for china developer not able access ghcr.io
        # it pulls images from another image registry and just use http proxy to pull chart 
        $ make e2e_init_spiderpool  -e E2E_CHINA_IMAGE_REGISTRY=true -e HTTP_PROXY=http://${ADDR}

        # setup cluster with calico cni
        $ make e2e_init_calico -e E2E_CHINA_IMAGE_REGISTRY=true -e HTTP_PROXY=http://${ADDR}

        # setup cluster with cilium cni
        $ make e2e_init_cilium_legacyservice  -e E2E_CHINA_IMAGE_REGISTRY=true -e HTTP_PROXY=http://${ADDR}

    如果需要测试指定的已发布镜像，运行以下命令：

        # load images to docker
        $ docker pull ${AGENT_IMAGE_NAME}:${IMAGE_TAG}
        $ docker pull ${CONTROLLER_IMAGE_NAME}:${IMAGE_TAG}
        $ docker pull ${CONTROLLER_IMAGE_NAME}:${IMAGE_TAG}
        $ docker pull ${MULTUS_IMAGE_NAME}:${IMAGE_TAG}

        # setup the cluster with the specified image
        $ make e2e_init_spiderpool -e E2E_SPIDERPOOL_TAG=${IMAGE_TAG} \
                -e SPIDERPOOL_AGENT_IMAGE_NAME=${AGENT_IMAGE_NAME}   \
                -e SPIDERPOOL_CONTROLLER_IMAGE_NAME=${CONTROLLER_IMAGE_NAME} \
                -e E2E_MULTUS_IMAGE_NAME=${MULTUS_IMAGE_NAME}

        # run all e2e test
        $ make e2e_test

    运行 E2E 测试的示例：

        # run all e2e test on dual-stack cluster
        $ make e2e_test_spiderpool

        # run all e2e test on ipv4-only cluster
        $ make e2e_test_spiderpool -e E2E_IP_FAMILY=ipv4

        # run all e2e test on ipv6-only cluster
        $ make e2e_test_spiderpool -e E2E_IP_FAMILY=ipv6

        # run smoke test
        $ make e2e_test_spiderpool -e E2E_GINKGO_LABELS=smoke

        # after finishing e2e case , you could test repeated for debugging flaky tests
        # example: run a case repeatedly
        $ make e2e_test_spiderpool -e E2E_GINKGO_LABELS=CaseLabel -e GINKGO_OPTION="--repeat=10 "

        # example: run a case until fails
        $ make e2e_test_spiderpool -e GINKGO_OPTION=" --label-filter=CaseLabel --until-it-fails "

        # Run all e2e tests for enableSpiderSubnet=false cluster
        $ make e2e_test_calico

        # Run all e2e tests for enableSpiderSubnet=false cluster
        $ make e2e_test_cilium_legacyservice 

        $ ls e2ereport.json

        $ make clean_e2e

4. 可以在您电脑的浏览器中访问 "<http://HostIp:4040>" 获取火焰图

5. 清理：`make clean_e2e`

***

## 提交 Pull Request

Pull Request 会经过以下工作流的检查，这些检查是合并的必要条件。

- 只有带有以下标签之一的 PR 才允许合并，这些标签用于在发布时生成 changelog。

    | 标签名 | 说明                                               |
    |----------------------------------------------------------|---------------|
    | release/bug           | 该 PR 修复了一个 bug                                  |
    | release/none           | 发布时不生成 changelog                  |
    | release/feature-new           | 该 PR 新增了一个功能                          |
    | release/feature-changed           | 该 PR 修改了已有功能的实现 |

    发布时会自动生成 changelog。

    changelog 会附加到 Github RELEASE 中，并提交到 'github_pages' 分支的 /changelogs 目录。

- 您的 PR 需要签名（sign off）。提交修改时，请在 commit 命令中添加 `-s`：`git commit -s`

- CI 会检查 yaml 格式。如果检查失败，请参考 [yaml 规则](https://yamllint.readthedocs.io/en/stable/rules.html)。

    问题修复后，可以在本地使用命令 `make lint-yaml` 进行验证。

    注意：如需忽略某条 yaml 规则，可以将其添加到 `.github/yamllint-conf.yml` 中。

- 所有 golang 和 shell 文件都必须包含正确的 license。

- CI 会检查 markdown 格式，如果检查失败，请参考 [Markdown 规则](https://github.com/DavidAnson/markdownlint/blob/main/doc/Rules.md)

  可以在本地使用命令 `make lint-markdown-format` 进行检查。

  可以在本地使用命令 `make fix-markdown-format` 进行修复。

  如果您认为可以忽略，可以将其添加到 `.github/markdownlint.yaml` 中。

- 如果 markdown 拼写检查出错，可以使用命令 `make lint-markdown-spell-colour` 进行检查。

  如果您认为可以忽略，可以将其添加到 `.github/.spelling` 中。

- 如果 CI 中 yaml 文件的 lint 检查失败，请参考 <https://yamllint.readthedocs.io/en/stable/rules.html> 了解原因。

    可以在本地使用命令 `make lint-yaml` 进行检查。

- golang 文件中的代码拼写错误都会被检查。

    可以在本地使用命令 `make lint-code-spell` 进行检查。

    可以在本地使用命令 `make fix-code-spell` 自动修复。

    如果您认为可以忽略，请编辑 `.github/codespell-ignorewords`，并确保所有字母均为小写。
