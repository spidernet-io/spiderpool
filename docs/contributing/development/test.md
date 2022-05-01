# e2e test

1. check required tools, if miss something, you could run 'test/scripts/install-tools.sh' to install them

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

    if your first run it, it will download some images, you could set the http proxy for it

        # ADDR=10.6.0.1
        # export https_proxy=http://${ADDR}:7890 http_proxy=http://${ADDR}:7890
        # make e2e

    could run specified case

        # make e2e -e E2E_GINKGO_LABELS="lable1,label2"

3. for developing e2e case, could do it step by step

        # make e2e_init
          .......
          -----------------------------------------------------------------------------------------------------
             succeeded to setup cluster spider
             you could use following command to access the cluster
                export KUBECONFIG=/root/git/spiderpool/test/.cluster/spider/.kube/config
                kubectl get nodes
          -----------------------------------------------------------------------------------------------------        

        # make e2e_test

        # ls e2ereport.json

        # make clean_e2e
