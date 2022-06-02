include ../Makefile.defs
include Makefile.defs


#============ kind-e2e ====================

.PHONY: all
all: usage

.PHONY: e2e
e2e:  kind-init e2e-test


.PHONY: kind-init
kind-init: check_env prepare
	@echo -e "\033[35m [Step 3] Init kind for the cluster: $(E2E_CLUSTER_NAME) \033[0m"
	-@ kind delete cluster --name ${E2E_CLUSTER_NAME} &>/dev/null
	-@ rm -rf $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)
	$(QUIET) mkdir -p -v $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)
	$(QUIET) kube_proxy_mode=$(E2E_KUBE_PROXY_MODE) ip_family=$(E2E_IP_FAMILY) \
			 standard_cluster=$(E2E_STANDARD_CLUSTER) kind_image_tag=$(E2E_KIND_IMAGE_TAG) \
			 disable_default_cni=$(E2E_DISABLE_DEFAULT_CNI) \
			 p2ctl -t $(ROOT_DIR)/test/yamls/kind-config.tmpl > $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/kind-config.yaml
	$(QUIET) cat $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/kind-config.yaml
	$(QUIET) kind create cluster \
		--config $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/kind-config.yaml \
		--name $(E2E_CLUSTER_NAME) --kubeconfig $(E2E_KUBECONFIG)
	@echo -e "\033[35m [Step 4] Install The CNI-Plugins: $(CNI_PACKAGE_VERSION)  \033[0m"
	$(QUIET) bash scripts/cni-install.sh $(E2E_CLUSTER_NAME) $(DOWNLOAD_DIR)
ifeq ($(INSTALL_SPIDER),true)
	@echo -e "\033[35m [Step 5] Install Macvlan + Spiderpool \033[0m"
	@if [ "$(USE_TLS_METHOD)" == "certmanager" ] ; then \
			echo "USE_TLS_METHOD $(USE_TLS_METHOD) " ; \
			$(QUIET) IMAGE_CERT_MANAGER="$(IMAGE_CERT_MANAGER_NAME)" scripts/install-cert-manager.sh "$(E2E_CLUSTER_NAME)" "$(CERT_MANAGER_ISSUER_NAME)" ; \
			$(MAKE) -C $(ROOT_DIR) install -e HELM_OPTION="--set spiderpoolController.tls.certManagerIssuerName=$(CERT_MANAGER_ISSUER_NAME) --set spiderpoolController.tls.method=certmanager" ; \
        elif [ "$(USE_TLS_METHOD)" == "provided" ] ; then \
			echo "USE_TLS_METHOD $(USE_TLS_METHOD) " ; \
			CA=` cat scripts/exampleTls/ca.crt  | tr -d '\n' ` ; \
			SERVER_CERT=` cat scripts/exampleProvidedTls/server.crt  | tr -d '\n' ` ; \
			SERVER_KEY=` cat scripts/exampleProvidedTls/server.key  | tr -d '\n' ` ; \
			$(MAKE) -C $(ROOT_DIR) install -e HELM_OPTION="--set ca.cert=\"$${CA}\" --set spiderpoolController.tls.server.cert=\"$${SERVER_CERT}\" --set spiderpoolController.tls.server.key=\"$${SERVER_KEY}\" --set spiderpoolController.tls.method=provided" ; \
        elif [ "$(USE_TLS_METHOD)" == "helm" ] ; then \
			echo "USE_TLS_METHOD $(USE_TLS_METHOD) " ; \
			CA_CERT=` cat scripts/exampleProvidedTls/ca.crt  | tr -d '\n' ` ; \
			CA_KEY=` cat scripts/exampleProvidedTls/ca.crt  | tr -d '\n' ` ; \
			$(MAKE) -C $(ROOT_DIR) install -e HELM_OPTION="--set ca.cert=\"$${CA_CERT}\" --set ca.key=\"$${CA_KEY}\" --set spiderpoolController.tls.method=helm" ; \
        else  \
            echo "miss tls cert" ;  \
			exit 1 ; \
        fi
	type=spiderpool p2ctl -t yamls/10-macvlan.tmpl > $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/10-macvlan-spider.conflist
	$(QUIET) bash scripts/install-macvlan.sh \
		$(E2E_CLUSTER_NAME) $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/10-macvlan-spider.conflist
	@echo "waiting for deployment/spiderpool-controller ready"
	kubectl rollout status --kubeconfig $(E2E_KUBECONFIG) -n $(RELEASE_NAMESPACE)  deployment/spiderpool-controller -w --timeout=60s || { echo "deployment/spiderpool-controller failed to start" ; exit 1 ; }
	@echo "waiting for daemonset/spiderpool-agent ready"
	kubectl rollout status --kubeconfig $(E2E_KUBECONFIG) -n $(RELEASE_NAMESPACE)  daemonset/spiderpool-agent -w --timeout=60s || { echo "daemonset/spiderpool-agent failed to start" ; exit 1 ; }
else
	@echo -e "\033[35m [Step 5] Install Macvlan + Whereabouts \033[0m"
	$(QUIET) IMAGE_WHEREABOUTS=$(IMAGE_WHEREABOUTS_NAME) \
			scripts/install-whereabouts.sh $(E2E_CLUSTER_NAME) $(E2E_KUBECONFIG)
	ip_family=$(E2E_IP_FAMILY) type=whereabouts p2ctl -t yamls/10-macvlan.tmpl > $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/10-macvlan.conflist
	$(QUIET) bash scripts/install-macvlan.sh $(E2E_CLUSTER_NAME) $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/10-macvlan.conflist
endif
ifeq ($(INSTALL_MULTUS),true)
	@echo -e "\033[35m [Step 6] Install Multus \033[0m"
	@ IMAGE_MULTUS=$(IMAGE_MULTUS_NAME) \
		TEST_IMAGE=$(TEST_IMAGE_NAME) \
		CLUSTER_PATH=$(CLUSTER_DIR)/$(E2E_CLUSTER_NAME) \
		scripts/install-multus.sh $(E2E_CLUSTER_NAME) $(E2E_KUBECONFIG)
endif
	-$(QUIET) kubectl --kubeconfig $(E2E_KUBECONFIG) delete namespace local-path-storage &>/dev/null
	@echo ""
	@echo "-----------------------------------------------------------------------------------------------------"
	@echo ""
	@echo -e "    succeeded to setup cluster $(E2E_CLUSTER_NAME) "
	@echo -e "    you could use following command to access the cluster "
	@echo -e ""
	@echo -e "         export KUBECONFIG=$(E2E_KUBECONFIG) "
	@echo -e "         kubectl get nodes "
	@echo ""
	@echo "-----------------------------------------------------------------------------------------------------"
	@echo ""


.PHONY: clean
clean:
	@rm -rf $(CLUSTER_DIR)
	-@  kind get clusters | xargs -n1  kind delete cluster --name


.PHONY: check_env
check_env:
	$(QUIET) [ -n "${E2E_CLUSTER_NAME}" ] || { echo "error, miss E2E_CLUSTER_NAME " ; false ; }
	$(QUIET) ( [ "$(E2E_IP_FAMILY)" == "ipv4" ] || [ "$(E2E_IP_FAMILY)" == "ipv6" ]  || [ "$(E2E_IP_FAMILY)" == "dual" ] ) \
			|| { echo "error, E2E_IP_FAMILY=$(E2E_IP_FAMILY) must be ipv4/ipv6/dual" ;  exit 1 ; }


.PHONY: prepare
prepare:
	@echo -e "\033[35m [Step 2] Check The Tools For Ready: \033[0m"
	$(QUEIT) JUST_CLI_CHECK=true scripts/install-tools.sh
	$(QUEIT) mkdir -p $(DOWNLOAD_DIR)
	$(QUEIT) IMAGE_LIST="" ; \
		 [ "$(USE_TLS_METHOD)" == "certmanager" ] && IMAGE_LIST+=" $${IMAGE_LIST} $(IMAGE_CERT_MANAGER_NAME) " ; \
		 IMAGE_LIST+=" $${IMAGE_LIST} $(IMAGE_MULTUS_NAME) " ; \
		 IMAGE_LIST+=" $${IMAGE_LIST} $(IMAGE_WHEREABOUTS_NAME) " ; \
		 IMAGE_LIST+=" $${IMAGE_LIST} $(TEST_IMAGE_NAME) " ; \
		 ARCH=$(ARCH) IMAGE_LIST="$${IMAGE_LIST}" \
		 	CNI_PACKAGE_VERSION=$(CNI_PACKAGE_VERSION)  scripts/prepare.sh $(DOWNLOAD_DIR)

#============ e2e ====================
.PHONY: e2e-test
e2e-test:
	@echo -e "\033[35m Run e2e test on the cluster $(E2E_CLUSTER_NAME) \033[0m "
	@ echo -e "\033[35m [E2E] Run E2E with ginkgo label=${E2E_GINKGO_LABELS} , timeout=${E2E_TIMEOUT} GINKGO_OPTION=${GINKGO_OPTION} \033[0m"
	@  NODE_LIST=` docker ps | egrep " kindest/node.* $(E2E_CLUSTER_NAME)-(control|worker)" | awk '{print $$NF }' ` ; \
		[ -n "$$NODE_LIST" ] || { echo "error, failed to find any kind nodes, please setup kind cluster $(E2E_CLUSTER_NAME) first" ; exit 1 ; } ; \
		NODE_LIST=` echo "$${NODE_LIST}" | tr -d ' ' | tr '\n' ',' ` ; \
		NODE_LIST=$${NODE_LIST%%,} ; \
		echo "find cluster node: $${NODE_LIST}" ; \
		export E2E_KIND_CLUSTER_NODE_LIST="$${NODE_LIST}" ; \
		export E2E_CLUSTER_NAME=$(E2E_CLUSTER_NAME) ; \
		if [ "$(E2E_IP_FAMILY)" == "ipv4" ] ; then \
			export E2E_IPV4_ENABLED=true ; export E2E_IPV6_ENABLED=false ; \
		elif [ "$(E2E_IP_FAMILY)" == "ipv6" ] ; then \
			export E2E_IPV4_ENABLED=false ; export E2E_IPV6_ENABLED=true ; \
		else \
			export E2E_IPV4_ENABLED=true ; export E2E_IPV6_ENABLED=true ; \
		fi ; \
		export E2E_KUBECONFIG_PATH=$(E2E_KUBECONFIG) ; [ -f "$(E2E_KUBECONFIG)" ] || { echo "error, does not exist KUBECONFIG $(E2E_KUBECONFIG)" ; exit 1 ; } ; \
        if [ "$(INSTALL_MULTUS)" == "true" ] ; then export E2E_MULTUS_CNI_ENABLED=true ; else  export E2E_MULTUS_CNI_ENABLED=false ; fi ; \
        if [ "$(INSTALL_SPIDER)" != "true" ] ; then \
          	export E2E_WHEREABOUT_IPAM_ENABLED=true ; \
          	export E2E_SPIDERPOOL_IPAM_ENABLED=false ; \
        else  \
        	export E2E_WHEREABOUT_IPAM_ENABLED=false ;  \
        	export E2E_SPIDERPOOL_IPAM_ENABLED=true ; \
       	fi ; \
		$(ROOT_DIR)/tools/scripts/ginkgo.sh  \
			--race --timeout=$(E2E_TIMEOUT) --output-interceptor-mode=none \
			--json-report e2ereport.json --output-dir $(ROOT_DIR) --procs $(E2E_GINKGO_PROCS) \
			--label-filter="$(E2E_GINKGO_LABELS)" -randomize-suites -randomize-all  -vv --keep-going  ${GINKGO_OPTION} \
			-r e2e/*   ; \
		(($$?!=0)) && echo "failed to run all test"  && exit 1 ; \
		echo "" ; \
		echo "============================================" ; \
		echo "succeeded to run all test" ; \
		echo "output report to e2ereport.json"


.PHONY: usage
usage:
	@echo "usage:"
	@echo -e "  \033[35m make prepare \033[0m:       --- Check some required tools is exist like docker/helm.etc and download cni-plugins"
	@echo -e "  \033[35m make init \033[0m:          --- Setup a kind cluster, Such as: kind-init E2E_CLUSTER_NAME=spider,More config refer to Makefile.defs(e2e-kind-config)"
	@echo -e "  \033[35m make e2e-test \033[0m:      --- Ginkgo test,Such as: make e2e-test, More config refer to Makefile.defs(e2e-kind-config)"
	@echo -e "  \033[35m make clean \033[0m:    --- Clean kind cluster and some config file, Such as: make clean E2E_CLUSTER_NAME=spider"
	@echo -e "  \033[35m make e2e \033[0m:           --- prepare -> kind-init -> e2e-test "
