# contributing

***

## setup cluster and run test

1. check required developing tools on you local host. If something missing, please run 'test/scripts/install-tools.sh' to install them

        # make dev-doctor
        go version go1.17 linux/amd64
        check e2e tools 
        pass   'docker' installed
        pass   'kubectl' installed
        pass   'kind' installed
        pass   'p2ctl' installed
        finish checking e2e tools

2. run the e2e

        # make e2e

   if your run it for the first time, it will download some images, you could set the http proxy

        # ADDR=10.6.0.1
        # export https_proxy=http://${ADDR}:7890 http_proxy=http://${ADDR}:7890
        # make e2e

   run a specified case

        # make e2e -e E2E_GINKGO_LABELS="lable1,label2"

3. you could do it step by step with the follow

    before start the test, you shoud know there are test scenes as following 

    | kind                                     | setup cluster                    | test                          |
    |------------------------------------------|----------------------------------|-------------------------------|
    | test underlay CNI without subnet feature | make    e2e_init_underlay        | make e2e_test_underlay        |
    | test underlay CNI with subnet feature    | make    e2e_init_underlay_subnet | make e2e_test_underlay_subnet |
    | test overlay CNI for calico              | make    e2e_init_overlay_calico  | make e2e_test_overlay_calico  |
    | test overlay CNI for cilium              | make    e2e_init_overlay_cilium  | make e2e_test_overlay_cilium  |

    if you are in China, it could add `-e E2E_CHINA_IMAGE_REGISTRY=true` to pull images from china image registry, add `-e HTTP_PROXY=http://${ADDR}` to get chart

    build the image

        # do some coding

        $ git add .
        $ git commit -s -m 'message'

        # !!! images is built by commit sha, so make sure the commit is submit locally
        $ make build_image
        # or (if buildx fail to pull images)
        $ make build_docker_image

    setup the cluster

        # setup the kind cluster of dual-stack
        # !!! images is tested by commit sha, so make sure the commit is submit locally
        $ make e2e_init_underlay
          .......
          -----------------------------------------------------------------------------------------------------
             succeeded to setup cluster spider
             you could use following command to access the cluster
                export KUBECONFIG=$(pwd)/test/.cluster/spider/.kube/config
                kubectl get nodes
          -----------------------------------------------------------------------------------------------------

        # setup the kind cluster of ipv4-only
        $ make e2e_init_underlay -e E2E_IP_FAMILY=ipv4

        # setup the kind cluster of ipv6-only
        $ make e2e_init_underlay -e E2E_IP_FAMILY=ipv6

        # for china developer not able access ghcr.io
        # it pulls images from another image registry and just use http proxy to pull chart 
        $ make e2e_init_underlay  -e E2E_CHINA_IMAGE_REGISTRY=true -e HTTP_PROXY=http://${ADDR}

        # setup cluster with subnet feature
        $ make e2e_init_underlay_subnet -e E2E_CHINA_IMAGE_REGISTRY=true -e HTTP_PROXY=http://${ADDR}

        # setup cluster with calico cni
        $ make e2e_init_calico -e E2E_CHINA_IMAGE_REGISTRY=true -e HTTP_PROXY=http://${ADDR}

        # setup cluster with cilium cni
        $ make e2e_init_cilium  -e E2E_CHINA_IMAGE_REGISTRY=true -e HTTP_PROXY=http://${ADDR}

    run the e2e test

        # run all e2e test on dual-stack cluster
        $ make e2e_test_underlay

        # run all e2e test on ipv4-only cluster
        $ make e2e_test_underlay -e E2E_IP_FAMILY=ipv4

        # run all e2e test on ipv6-only cluster
        $ make e2e_test_underlay -e E2E_IP_FAMILY=ipv6

        # run smoke test
        $ make e2e_test_underlay -e E2E_GINKGO_LABELS=smoke

        # after finishing e2e case , you could test repeated for debugging flaky tests
        # example: run a case repeatedly
        $ make e2e_test_underlay -e E2E_GINKGO_LABELS=CaseLabel -e GINKGO_OPTION="--repeat=10 "

        # example: run a case until fails
        $ make e2e_test_underlay -e GINKGO_OPTION=" --label-filter=CaseLabel --until-it-fails "

        # Run all e2e tests for enableSpiderSubnet=false cluster
        $ make e2e_test_underlay_subnet 

        # Run all e2e tests for enableSpiderSubnet=false cluster
        $ make e2e_test_overlay_calico

        # Run all e2e tests for enableSpiderSubnet=false cluster
        $ make e2e_test_overlay_cilium 

        $ ls e2ereport.json

        $ make clean_e2e

4. you could test specified images with the follow

        # load images to docker
        $ docker pull ${AGENT_IMAGE_NAME}:${IMAGE_TAG}
        $ docker pull ${CONTROLLER_IMAGE_NAME}:${IMAGE_TAG}
        $ docker pull ${CONTROLLER_IMAGE_NAME}:${IMAGE_TAG}
        $ docker pull ${MULTUS_IMAGE_NAME}:${IMAGE_TAG}

        # setup the cluster with the specified image
        $ make e2e_init_underlay -e E2E_SPIDERPOOL_TAG=${IMAGE_TAG} \
                -e SPIDERPOOL_AGENT_IMAGE_NAME=${AGENT_IMAGE_NAME}   \
                -e SPIDERPOOL_CONTROLLER_IMAGE_NAME=${CONTROLLER_IMAGE_NAME} \
                -e E2E_MULTUS_IMAGE_NAME=${MULTUS_IMAGE_NAME}

        # run all e2e test
        $ make e2e_test

5 finally, you could visit "<http://HostIp:4040>" the in the browser of your desktop, and get flamegraph

***

## Submit Pull Request

A pull request will be checked by following workflow, which is required for merging.

### Action: your PR should be signed off

When you commit your modification, add `-s` in your commit command `git commit -s`

### Action: check yaml files

If this check fails, see the [yaml rule](https://yamllint.readthedocs.io/en/stable/rules.html).

Once the issue is fixed, it could be verified on your local host by command `make lint-yaml`.

Note: To ignore a yaml rule, you can add it into `.github/yamllint-conf.yml`.

### Action: check golang source code

It checks the following items against any updated golang file.

* Mod dependency updated, golangci-lint, gofmt updated, go vet, use internal lock pkg

* Comment `// TODO` should follow the format: `// TODO (AuthorName) ...`, which easy to trace the owner of the remaining job

* Unitest and upload coverage to codecov

* Each golang test file should mark ginkgo label

### Action: check licenses

Any golang or shell file should be licensed correctly.

### Action: check markdown file

* Check markdown format, if fails, See the [Markdown Rule](https://github.com/DavidAnson/markdownlint/blob/main/doc/Rules.md)

  You can test it on your local machine with the command `make lint-markdown-format`.

  You can fix it on your local machine with the command `make fix-markdown-format`.

  If you believe it can be ignored, you can add it to `.github/markdownlint.yaml`.

* Check markdown spell error.

  You can test it with the command `make lint-markdown-spell-colour`.

  If you believe it can be ignored, you can add it to `.github/.spelling`.

### Action: lint yaml file

If it fails, see <https://yamllint.readthedocs.io/en/stable/rules.html> for reasons.

You can test it on your local machine with the command `make lint-yaml`.

### Action: lint chart

### Action: lint openapi.yaml

### Action: check code spell

Any code spell error of golang files will be checked.

You can check it on your local machine with the command `make lint-code-spell`.

It could be automatically fixed on your local machine with the command `make fix-code-spell`.

If you believe it can be ignored, edit `.github/codespell-ignorewords` and make sure all letters are lower-case.

## Changelog

How to automatically generate changelogs:

1. All PRs should be labeled with "pr/release/***" and can be merged.

2. When you add the label, the changelog will be created automatically.

   The changelog contents include:

   * New Features: it includes all PRs labeled with "pr/release/feature-new"

   * Changed Features: it includes all PRs labeled with "pr/release/feature-changed"

   * Fixes: it includes all PRs labeled with "pr/release/bug"

   * All historical commits within this version

3. The changelog will be attached to Github RELEASE and submitted to /changelogs of branch 'github_pages'.
