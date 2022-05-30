# e2e test

***

you could follow the below steps to test:

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

3. when developing e2e case, please follow the below steps

        $ make e2e_init
          .......
          -----------------------------------------------------------------------------------------------------
             succeeded to setup cluster spider
             you could use following command to access the cluster
                export KUBECONFIG=/root/git/spiderpool/test/.cluster/spider/.kube/config
                kubectl get nodes
          -----------------------------------------------------------------------------------------------------        

        $ make e2e_test

        # after finish e2e case , you could test repeated for debugging flaky tests
        # example: run a case repeated
        $ make e2e_test -e GINKGO_OPTION=" --label-filter=CaseLabel --repeat=10 "

        # example: run a case until fails
        $ make e2e_test -e GINKGO_OPTION=" --label-filter=CaseLabel --until-it-fails "

        $ ls e2ereport.json

        $ make clean_e2e
