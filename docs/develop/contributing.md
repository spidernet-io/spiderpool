# Contribution

***

## setup cluster and run E2E test

1. check required developing tools on you local host. If something missing, please run 'test/scripts/install-tools.sh' to install them

        # make dev-doctor
        go version go1.17 linux/amd64
        check e2e tools 
        pass   'docker' installed
        pass   'kubectl' installed
        pass   'kind' installed
        pass   'p2ctl' installed
        finish checking e2e tools

2. build the local image

        # do some coding

        $ git add .
        $ git commit -s -m 'message'

        # !!! images is built by commit sha, so make sure the commit is submit locally
        $ make build_image
        # or (if buildx fail to pull images)
        $ make build_docker_image

3. set up the cluster and run E2E test

    What you should know is that there are some scenarios for different test. There are three scenarios mapping to different setup and test commands

    | Goal                                                 | Command for setup cluster                            | Command for running E2E test                         |
    |------------------------------------------------------|------------------------------------------------------|------------------------------------------------------|
    | test spiderpool                                      | make    e2e_init_spiderpool                            | make e2e_test_spiderpool                               |
    | test for dual-CNI cluster with calico and spiderpool | make    e2e_init_overlay_calico                      | make e2e_test_overlay_calico                         |
    | test for dual-CNI cluster with calico and spiderpool | make    e2e_init_overlay_cilium                      | make e2e_test_overlay_cilium                         |

    if you are in China, it could add `-e E2E_CHINA_IMAGE_REGISTRY=true` to pull images from china image registry, add `-e HTTP_PROXY=http://${ADDR}` to get chart

    Examples for setup :

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

    if it is expected to test a specified released images, run following commands :

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

    Example for running the e2e test:

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

4. It could visit "<http://HostIp:4040>" from the browser of your computer and get flame graph

5. clean `make clean_e2e`

***

## Submit Pull Request

A pull request will be checked by following workflow, which is required for merging.

- Only the PR labeled with the following is allowed to be merged, which is used to generate changelog when releasing.

    | label name | description                                               |
    |----------------------------------------------------------|---------------|
    | release/bug           | this PR is to fix a bug                                  |
    | release/none           | do not generate changelog when relasing                  |
    | release/feature-new           | this PR is to add a new feature                          |
    | release/feature-changed           | theis PR is to modify the implementation of an exsited feature |

    When releasing, the changelog will be created automatically.

    The changelog will be attached to Github RELEASE and submitted to /changelogs of branch 'github_pages'.

- Your PR should be signed off. When you commit your modification, add `-s` in your commit command `git commit -s`

- The CI check yaml format. If this check fails, see the [yaml rule](https://yamllint.readthedocs.io/en/stable/rules.html).

    Once the issue is fixed, it could be verified on your local host by command `make lint-yaml`.

    Note: To ignore a yaml rule, you can add it into `.github/yamllint-conf.yml`.

- Any golang or shell file should be licensed correctly.

- The CI check markdown format, if fails, See the [Markdown Rule](https://github.com/DavidAnson/markdownlint/blob/main/doc/Rules.md)

  You can test it on your local machine with the command `make lint-markdown-format`.

  You can fix it on your local machine with the command `make fix-markdown-format`.

  If you believe it can be ignored, you can add it to `.github/markdownlint.yaml`.

- when markdown spell go error, you can test it with the command `make lint-markdown-spell-colour`.

  If you believe it can be ignored, you can add it to `.github/.spelling`.

- when CI failing for lint yaml file, see <https://yamllint.readthedocs.io/en/stable/rules.html> for reasons.

    You can test it on your local machine with the command `make lint-yaml`.

- Any code spell error of golang files will be checked.

    You can check it on your local machine with the command `make lint-code-spell`.

    It could be automatically fixed on your local machine with the command `make fix-code-spell`.

    If you believe it can be ignored, edit `.github/codespell-ignorewords` and make sure all letters are lower-case.
