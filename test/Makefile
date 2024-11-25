include ../Makefile.defs
include Makefile.defs


#============ kind-e2e ====================

.PHONY: all
all: usage

.PHONY: e2e
e2e:  kind-init e2e-test

.PHONY: init_env_with_release
init_env_with_release:
	VERSION=`cat ../VERSION | tr -d '\n' ` ; [ -n "$${VERSION}" ] || { echo "error, wrong version" ; exit 1 ; } ; \
		echo "check version $${VERSION}" ; \
		for NAME in $(SPIDERPOOL_IMAGES); do \
			if $(CONTAINER_ENGINE) images $${NAME}:$${VERSION} | grep -q $${VERSION} &>/dev/null ; then \
				echo "test's image $${NAME}:$${VERSION} found" && continue ; \
			fi ; \
			if $(CONTAINER_ENGINE) pull $${NAME}:$${VERSION} ; then \
				echo "Successfully pulled test image $${NAME}:$${VERSION}" && continue ; \
			fi ; \
			echo "error, failed to pull $${NAME}:$${VERSION}, please check if the image exists or try running it with `-e E2E_CHINA_IMAGE_REGISTRY=true`." >&2 && false ; \
		 done
	$(QUIET)  make kind-init -e E2E_SPIDERPOOL_TAG=${VERSION}

.PHONY: kind-init
kind-init: check_env prepare
	@echo -e "\033[35m [Step 3] Init kind for the cluster: $(E2E_CLUSTER_NAME) \033[0m"
	E2E_KIND_IMAGE_TAG=$(E2E_KIND_IMAGE_TAG) make setup_kind
	@echo -e "\033[35m [Step 4] Init network config for kind-node \033[0m"
	E2E_IP_FAMILY=$(E2E_IP_FAMILY) \
		E2E_CLUSTER_NAME=$(E2E_CLUSTER_NAME)  \
		E2E_VLAN_GATEWAY_IMAGE=$(E2E_VLAN_GATEWAY_IMAGE) \
		VLAN_GATEWAY_CONTAINER=$(VLAN_GATEWAY_CONTAINER) \
		$(QUIET) bash scripts/config-network.sh
ifeq ($(INSTALL_OVS),true)
	@echo -e "\033[35m [Step 5] Install openvswitch \033[0m"
	DOCKER_ADDITIONAL_NETWORK=$(DOCKER_ADDITIONAL_NETWORK) \
		E2E_CLUSTER_NAME=$(E2E_CLUSTER_NAME)  \
		HOST_ADDITIONAL_INTERFACE=$(HOST_ADDITIONAL_INTERFACE) \
		BRIDGE_INTERFACE=$(BRIDGE_INTERFACE) \
		E2E_IP_FAMILY=$(E2E_IP_FAMILY) \
		HTTP_PROXY=$(HTTP_PROXY) \
		$(QUIET) bash scripts/install-ovs.sh
endif
ifeq ($(INSTALL_OVERLAY_CNI),true)
	@echo -e "\033[35m [Step 6] Install Default-CNI: Calico-$(CALICO_VERSION) Cilium-$(CILIUM_VERSION) \033[0m"
	E2E_KUBECONFIG=$(E2E_KUBECONFIG)  \
		E2E_IP_FAMILY=$(E2E_IP_FAMILY) \
		E2E_CLUSTER_NAME=$(E2E_CLUSTER_NAME) \
		INSTALL_CALICO=$(INSTALL_CALICO) \
		INSTALL_CILIUM=$(INSTALL_CILIUM) \
		CALICO_VERSION=$(CALICO_VERSION) \
		DEFAULT_CALICO_VERSION=$(DEFAULT_CALICO_VERSION) \
		CALICO_IMAGE_REPO=$(CALICO_IMAGE_REPO) \
		CLUSTER_PATH=$(CLUSTER_DIR)/$(E2E_CLUSTER_NAME) \
		CALICO_AUTODETECTION_METHOD=$(CALICO_AUTODETECTION_METHOD) \
		E2E_CILIUM_IMAGE_REPO=$(E2E_CILIUM_IMAGE_REPO) \
		CILIUM_VERSION=$(CILIUM_VERSION) \
		CILIUM_CLUSTER_POD_SUBNET_V4=$(CILIUM_CLUSTER_POD_SUBNET_V4) \
		CILIUM_CLUSTER_POD_SUBNET_V6=$(CILIUM_CLUSTER_POD_SUBNET_V6) \
		DISABLE_KUBE_PROXY=$(DISABLE_KUBE_PROXY) \
		CALICO_CLUSTER_POD_SUBNET_V4=$(CALICO_CLUSTER_POD_SUBNET_V4) \
		CALICO_CLUSTER_POD_SUBNET_V4=$(CALICO_CLUSTER_POD_SUBNET_V4) \
		HTTP_PROXY=$(HTTP_PROXY) \
		$(QUIET) bash scripts/install-default-cni.sh
endif
	@echo -e "\033[35m [Step 7] Install Spiderpool \033[0m"
	@ make setup_spiderpool
	@echo -e "\033[35m [Step 8] Install Multus \033[0m"
	@ E2E_IP_FAMILY=$(E2E_IP_FAMILY) \
		MULTUS_DEFAULT_CNI_NAME=$(MULTUS_DEFAULT_CNI_VLAN0) \
		MULTUS_DEFAULT_CNI_VLAN100=$(MULTUS_DEFAULT_CNI_VLAN100) \
		MULTUS_DEFAULT_CNI_VLAN200=$(MULTUS_DEFAULT_CNI_VLAN200) \
		MULTUS_OVS_CNI_VLAN30=$(MULTUS_OVS_CNI_VLAN30) \
		MULTUS_OVS_CNI_VLAN40=$(MULTUS_OVS_CNI_VLAN40) \
		INSTALL_OVS=$(INSTALL_OVS) \
		RELEASE_NAMESPACE=$(RELEASE_NAMESPACE) \
		DISABLE_KUBE_PROXY=$(DISABLE_KUBE_PROXY) \
		CLUSTER_PATH=$(CLUSTER_DIR)/$(E2E_CLUSTER_NAME) \
		E2E_SPIDERPOOL_ENABLE_SUBNET=${E2E_SPIDERPOOL_ENABLE_SUBNET} \
		scripts/install-multus.sh $(E2E_CLUSTER_NAME) $(E2E_KUBECONFIG)
ifeq ($(INSTALL_KRUISE),true)
	@echo -e "\033[35m [Step 9] Install operator: kurise \033[0m"
	@ make setup_kurise
endif
ifeq ($(INSTALL_KDOCTOR),true)
	@echo -e "\033[35m [Step 10] Install kdoctor \033[0m"
	E2E_KUBECONFIG=$(E2E_KUBECONFIG)  \
		E2E_IP_FAMILY=$(E2E_IP_FAMILY) \
		E2E_CLUSTER_NAME=$(E2E_CLUSTER_NAME) \
		KDOCTOR_VERSION=$(KDOCTOR_VERSION) \
		KDOCTOR_REPORT_PATH=$(KDOCTOR_REPORT_PATH) \
		E2E_KDOCTOR_IMAGE_REPO=$(E2E_KDOCTOR_IMAGE_REPO) \
		HTTP_PROXY=$(HTTP_PROXY) \
		$(QUIET) bash scripts/install-kdoctor.sh
endif
ifeq ($(INSTALL_KUBEVIRT),true)
	@echo -e "\033[35m [Step 11] Install kubevirt \033[0m"
	E2E_KUBECONFIG=$(E2E_KUBECONFIG)  \
		E2E_CLUSTER_NAME=$(E2E_CLUSTER_NAME) \
		HTTP_PROXY=$(HTTP_PROXY) \
		KUBEVIRT_VERSION=$(KUBEVIRT_VERSION) \
		E2E_KUBEVIRT_IMAGE_REPO=$(E2E_KUBEVIRT_IMAGE_REPO) \
		$(QUIET) bash scripts/install-kubevirt.sh
endif
	@ echo "wait for the cluster ready" ; \
		TEST_IMAGE_NAME=$(TEST_IMAGE_NAME) \
		RELEASE_NAMESPACE=$(RELEASE_NAMESPACE) \
		E2E_IP_FAMILY=$(E2E_IP_FAMILY) \
		MULTUS_DEFAULT_CNI_NAME=$(MULTUS_DEFAULT_CNI_VLAN0) \
		E2E_CLUSTER_NAME=$(E2E_CLUSTER_NAME) \
		INSTALL_OVERLAY_CNI=$(INSTALL_OVERLAY_CNI) \
		scripts/preCheckClusterReady.sh $(E2E_KUBECONFIG)
	@echo ""
	@echo "-----------------------------------------------------------------------------------------------------"
	@echo "       ip family: $(E2E_IP_FAMILY)"
	@echo "       multus default CNI: $(RELEASE_NAMESPACE)/$(MULTUS_DEFAULT_CNI_NAME) "
	@echo "       multus addon CNI: $(RELEASE_NAMESPACE)/$(MULTUS_ADDITIONAL_CNI_NAME) "
	@echo ""
	@echo -e "    succeeded to setup cluster $(E2E_CLUSTER_NAME) "
	@echo -e "    you could use following command to access the cluster "
	@echo -e ""
	@echo -e "         export KUBECONFIG=$(E2E_KUBECONFIG) "
	@echo -e "         kubectl get pod -o wide -A "
	@echo ""
	@echo "-----------------------------------------------------------------------------------------------------"
	@echo ""
	@ KUBECONFIG=$(E2E_KUBECONFIG) kubectl get pods -o wide -A

.PHONY: setup_kind
setup_kind:
	-@ kind delete cluster --name $(E2E_CLUSTER_NAME) &>/dev/null
	-@ rm -rf $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)
	- sudo sysctl -w fs.inotify.max_user_watches=524288 || true
	- sudo sysctl -w fs.inotify.max_user_instances=8192 || true
	- sudo sysctl -w net.ipv6.conf.all.disable_ipv6=0
	$(QUIET) mkdir -p -v $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)
	NEW_KIND_YAML=$(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/kind-config.yaml ;\
		INSERT_LINE=` grep "insert inform" $(GLOBAL_KIND_CONFIG_PATH) -n | awk -F':' '{print $$1}' ` ; \
		echo "insert after line $${INSERT_LINE}" ;\
		sed  ''"$${INSERT_LINE}"' a \  kubeProxyMode: $(E2E_KUBE_PROXY_MODE)' $(GLOBAL_KIND_CONFIG_PATH) > $${NEW_KIND_YAML} ; \
		sed -i ''"$${INSERT_LINE}"' a \  disableDefaultCNI: $(E2E_DISABLE_DEFAULT_CNI)' $${NEW_KIND_YAML} ;\
		sed -i ''"$${INSERT_LINE}"' a \  ipFamily: $(E2E_IP_FAMILY)' $${NEW_KIND_YAML} ;\
		if [ "$(E2E_IP_FAMILY)" == "ipv4" ] ; then \
			sed -i  ''"$${INSERT_LINE}"' a \  podSubnet: "$(CLUSTER_POD_SUBNET_V4)"' $${NEW_KIND_YAML} ;\
			sed -i  ''"$${INSERT_LINE}"' a \  serviceSubnet: "$(K8S_IPV4_SERVICE_CIDR)"' $${NEW_KIND_YAML} ;\
		elif [ "$(E2E_IP_FAMILY)" == "ipv6" ] ; then \
			sed -i  ''"$${INSERT_LINE}"' a \  podSubnet: "$(CLUSTER_POD_SUBNET_V6)"' $${NEW_KIND_YAML} ; \
			sed -i  ''"$${INSERT_LINE}"' a \  serviceSubnet: "$(K8S_IPV6_SERVICE_CIDR)"' $${NEW_KIND_YAML} ; \
		else \
			sed -i  ''"$${INSERT_LINE}"' a \  podSubnet: "$(CLUSTER_POD_SUBNET_V4),$(CLUSTER_POD_SUBNET_V6)"' $${NEW_KIND_YAML}  ; \
			sed -i  ''"$${INSERT_LINE}"' a \  serviceSubnet: "$(K8S_IPV4_SERVICE_CIDR),$(K8S_IPV6_SERVICE_CIDR)"' $${NEW_KIND_YAML}  ; \
  		fi ; \
		MultiCIDRServiceGateVersion="v1.29.0" ; \
		if [ $$(echo -e "$(E2E_KIND_IMAGE_TAG)\n$${MultiCIDRServiceGateVersion}" | sort -V | tail -n1) == "$(E2E_KIND_IMAGE_TAG)" ]; then \
			echo "the following features are only fully supported in versions higher than $${MultiCIDRServiceGateVersion}." ; \
			sed -i  '$$  a\featureGates: ' $${NEW_KIND_YAML}  ; \
			sed -i  '$$  a\   MultiCIDRServiceAllocator: true' $${NEW_KIND_YAML}  ; \
			sed -i  '$$  a\runtimeConfig: ' $${NEW_KIND_YAML}  ; \
			sed -i  '$$  a\   api/alpha: "true"' $${NEW_KIND_YAML}  ; \
		fi
	$(QUIET) cat $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/kind-config.yaml
	KIND_OPTION="" ; \
       		[ -n "$(E2E_KIND_IMAGE_TAG)" ] && KIND_OPTION=" --image $(E2E_KIND_IMAGE_NAME):$(E2E_KIND_IMAGE_TAG) " && echo "setup kind with $(E2E_KIND_IMAGE_NAME):$(E2E_KIND_IMAGE_TAG)"; \
            kind create cluster --config $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/kind-config.yaml \
			--name $(E2E_CLUSTER_NAME) --kubeconfig $(E2E_KUBECONFIG) $${KIND_OPTION}
	- kubectl --kubeconfig $(E2E_KUBECONFIG) taint nodes --all node-role.kubernetes.io/master- || true
	- kubectl --kubeconfig $(E2E_KUBECONFIG) taint nodes --all node-role.kubernetes.io/control-plane- || true
	for ((N=0;N<=30;N++)); do \
		sleep 1 ; \
		kubectl get node --kubeconfig $(E2E_KUBECONFIG) &>/dev/null && break ; \
		echo "wait for node ready" ; \
	done ; \
    kubectl get node --kubeconfig $(E2E_KUBECONFIG) &>/dev/null || { echo "error, cluster is not ready" ; exit 1 ; }
	echo "show kubernetes node image " && docker ps
	@echo "===================== deploy prometheus CRD ========== "
	{ timeout 10 kubectl apply --kubeconfig $(E2E_KUBECONFIG) -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/example/prometheus-operator-crd/monitoring.coreos.com_servicemonitors.yaml ; } \
		|| kubectl apply --kubeconfig $(E2E_KUBECONFIG)  -f ./yamls/monitoring.coreos.com_servicemonitors.yaml
	{ timeout 10 kubectl apply --timeout 10s --kubeconfig $(E2E_KUBECONFIG) -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/example/prometheus-operator-crd/monitoring.coreos.com_podmonitors.yaml ;} \
		|| kubectl apply --kubeconfig $(E2E_KUBECONFIG) -f ./yamls/monitoring.coreos.com_podmonitors.yaml
	{ timeout 10 kubectl apply --timeout 10s --kubeconfig $(E2E_KUBECONFIG) -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/example/prometheus-operator-crd/monitoring.coreos.com_prometheusrules.yaml ;} \
		|| kubectl apply --kubeconfig $(E2E_KUBECONFIG) -f ./yamls/monitoring.coreos.com_prometheusrules.yaml
	{ timeout 10 kubectl apply --timeout 10s --kubeconfig $(E2E_KUBECONFIG) -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/example/prometheus-operator-crd/monitoring.coreos.com_probes.yaml  ;} \
		|| kubectl apply --kubeconfig $(E2E_KUBECONFIG) -f ./yamls/monitoring.coreos.com_probes.yaml
	{ timeout 10 kubectl apply --timeout 10s --kubeconfig $(E2E_KUBECONFIG) -f https://raw.githubusercontent.com/grafana-operator/grafana-operator/master/deploy/manifests/latest/crds.yaml  ;} \
		|| kubectl apply --kubeconfig $(E2E_KUBECONFIG) -f ./yamls/grafanadashboards.yaml


.PHONY: setup_kurise
setup_kurise:
	@echo "add openkruise charts repository..." ; \
		[ -z "$(HTTP_PROXY)" ] || export https_proxy=$(HTTP_PROXY) ; \
			helm repo add openkruise https://openkruise.github.io/charts/ ; \
			IMAGE_LIST=` helm template kruise openkruise/kruise --set manager.image.repository=$(E2E_OPENKRUISE_IMAGE) | grep " image: " | tr -d '"'| awk '{print $$2}' | uniq ` ; \
			for IMAGE in $${IMAGE_LIST}; do \
				docker pull $${IMAGE} ; \
				kind load docker-image $${IMAGE} --name $(E2E_CLUSTER_NAME);	\
			done; \
			helm upgrade --install kruise openkruise/kruise --wait --timeout 20m --debug --set manager.image.repository=$(E2E_OPENKRUISE_IMAGE) \
			    --kubeconfig $(E2E_KUBECONFIG) || { KIND_CLUSTER_NAME=$(E2E_CLUSTER_NAME) ./scripts/debugEnv.sh $(E2E_KUBECONFIG) "detail" "$(E2E_LOG_FILE)" ; exit 1 ; } ; \

.PHONY: setup_spiderpool
setup_spiderpool:
		if [ "$(USE_TLS_METHOD)" == "certmanager" ] ; then \
			echo "USE_TLS_METHOD $(USE_TLS_METHOD) " ; \
			$(QUIET) IMAGE_CERT_MANAGER="$(IMAGE_CERT_MANAGER_NAME)" scripts/install-cert-manager.sh "$(E2E_CLUSTER_NAME)" "$(CERT_MANAGER_ISSUER_NAME)" ; \
			HELM_OPTION="--set spiderpoolController.tls.certmanager.issuerName=$(CERT_MANAGER_ISSUER_NAME) --set spiderpoolController.tls.method=certmanager" ; \
		elif [ "$(USE_TLS_METHOD)" == "provided" ] ; then \
			echo "USE_TLS_METHOD $(USE_TLS_METHOD) " ; \
			$(ROOT_DIR)/tools/cert/generateCert.sh "$(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/tls" ; \
			CA=`cat $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/tls/ca.crt  | base64 -w0 | tr -d '\n' ` ; \
			SERVER_CERT=` cat $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/tls/server.crt | base64 -w0 | tr -d '\n' ` ; \
			SERVER_KEY=` cat $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/tls/server.key | base64 -w0 | tr -d '\n' ` ; \
			HELM_OPTION="--set spiderpoolController.tls.provided.tlsCert=\"$${SERVER_CERT}\" --set spiderpoolController.tls.provided.tlsKey=\"$${SERVER_KEY}\" --set spiderpoolController.tls.provided.tlsCa=\"$${CA}\" --set spiderpoolController.tls.method=provided" ; \
		elif [ "$(USE_TLS_METHOD)" == "auto" ] ; then \
			echo "USE_TLS_METHOD $(USE_TLS_METHOD) " ; \
			HELM_OPTION=" --set spiderpoolController.tls.method=auto " ; \
		else \
			echo "unknown USE_TLS_METHOD $(USE_TLS_METHOD) " ; \
			exit 1 ; \
		fi ; \
		if [ "$(E2E_SPIDERPOOL_ENABLE_COORDINATOR)" == "true" ] ; then \
		    HELM_OPTION+=" --set coordinator.enabled=true " ; \
		else \
		  	HELM_OPTION+=" --set coordinator.enabled=false " ; \
		fi ; \
		HELM_OPTION+=" --set multus.multusCNI.install=true " ; \
		HELM_OPTION+=" --set multus.multusCNI.uninstall=true " ; \
		HELM_OPTION+=" --set multus.multusCNI.image.registry= " ; \
		HELM_OPTION+=" --set multus.multusCNI.image.repository=$(E2E_MULTUS_IMAGE_NAME) " ; \
		if [ "$(INSTALL_OVERLAY_CNI)" == "true" ]; then \
		    HELM_OPTION+=" --set multus.multusCNI.defaultCniCRName= " ; \
		else \
		    HELM_OPTION+=" --set multus.multusCNI.defaultCniCRName=$(MULTUS_DEFAULT_CNI_VLAN0) " ; \
		fi ; \
		if [ "$(E2E_SPIDERPOOL_ENABLE_MULTUSCONFIG)" == "true" ] ; then \
			HELM_OPTION+=" --set multus.enableMultusConfig=true " ; \
		else \
			HELM_OPTION+=" --set multus.enableMultusConfig=false " ; \
		fi ; \
		if [ "$(E2E_SPIDERPOOL_ENABLE_SUBNET)" == "true" ] ; then \
		    HELM_OPTION+=" --set ipam.enableSpiderSubnet=true " ; \
		else \
		    HELM_OPTION+=" --set ipam.enableSpiderSubnet=false " ; \
		fi ; \
		if [ "$(INSTALL_SRIOV)" == "true" ] ; then \
			HELM_OPTION+=" --set sriov.install=true " ; \
		else \
			HELM_OPTION+=" --set sriov.install=false " ; \
		fi ; \
		if [ "$(INSTALL_RDMA)" == "true" ] ; then \
		    HELM_OPTION+=" --set rdma.rdmaSharedDevicePlugin.install=true " ; \
			HELM_OPTION+=" --set plugins.installRdmaCNI=true " ; \
		else \
		    HELM_OPTION+=" --set rdma.rdmaSharedDevicePlugin.install=false " ; \
			HELM_OPTION+=" --set plugins.installRdmaCNI=false " ; \
		fi ; \
		HELM_OPTION+=" --set spiderpoolAgent.debug.logLevel=debug --set spiderpoolController.debug.logLevel=debug  " ; \
		HELM_OPTION+=" --set ipam.gc.gcAll.intervalInSecond=30 " ; \
		ipv4_regex="^([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\/[0-9]+)$$"; \
		ipv6_regex="^([a-fA-F0-9:]+\/[0-9]+)$$"; \
		subnet0=$$(docker network inspect kind -f {{\(index\ $$.IPAM.Config\ 0\).Subnet}}) ; \
		gateway0=$$(docker network inspect kind -f {{\(index\ $$.IPAM.Config\ 0\).Gateway}}) ; \
		subnet1=$$(docker network inspect kind -f {{\(index\ $$.IPAM.Config\ 1\).Subnet}}) ; \
		gateway1=$$(docker network inspect kind -f {{\(index\ $$.IPAM.Config\ 1\).Gateway}}) ; \
		if [[ $${subnet0} =~ $${ipv4_regex} ]]; then \
			ipv4_subnet=$${subnet0}; \
			ipv4_gateway=$${gateway0}; \
		elif [[ $${subnet0} =~ $${ipv6_regex} ]]; then \
			ipv6_subnet=$${subnet0}; \
			ipv6_gateway=$${gateway0}; \
		fi; \
		if [[ $${subnet1} =~ $${ipv4_regex} ]]; then \
			ipv4_subnet=$${subnet1}; \
			ipv4_gateway=$${gateway1}; \
		elif [[ $${subnet1} =~ $${ipv6_regex} ]]; then \
			ipv6_subnet=$${subnet1}; \
			ipv6_gateway=$${gateway1}; \
		fi;  \
		echo "docker network kind IPv4 subnet $${ipv4_subnet}"; \
		echo "docker network kind IPv4 gateway $${ipv4_gateway}"; \
		echo "docker network kind IPv6 subnet $${ipv6_subnet}"; \
		echo "docker network kind IPv6 gateway $${ipv6_gateway}"; \
		if [ "$(E2E_IP_FAMILY)" == "ipv4" ] ; then \
			ipv4_prefix=$$(echo $${ipv4_subnet} | awk '{split($$0,a,".");printf "%s.%s",a[1],a[2]}'); \
			ipv4_ip_range=$${ipv4_prefix}.40.2-$${ipv4_prefix}.40.254 ; \
			HELM_OPTION+=" --set clusterDefaultPool.installIPv4IPPool=true --set clusterDefaultPool.installIPv6IPPool=false" ; \
			HELM_OPTION+=" --set clusterDefaultPool.ipv4IPPoolName=default-v4-ippool" ; \
			HELM_OPTION+=" --set clusterDefaultPool.ipv4Subnet=$${ipv4_subnet}" ; \
			HELM_OPTION+=" --set clusterDefaultPool.ipv4Gateway=$${ipv4_gateway}" ; \
			HELM_OPTION+=" --set clusterDefaultPool.ipv4IPRanges={$${ipv4_ip_range}}" ; \
			HELM_OPTION+=" --set ipam.enableIPv4=true --set ipam.enableIPv6=false" ; \
		elif [ "$(E2E_IP_FAMILY)" == "ipv6" ] ; then \
			ipv6_ip_range=$${ipv6_subnet%::*}:f::2-$${ipv6_subnet%::*}:f::fe ; \
			HELM_OPTION+=" --set clusterDefaultPool.installIPv4IPPool=false --set clusterDefaultPool.installIPv6IPPool=true" ; \
			HELM_OPTION+=" --set clusterDefaultPool.ipv6IPPoolName=default-v6-ippool" ; \
			HELM_OPTION+=" --set clusterDefaultPool.ipv6Subnet=$${ipv6_subnet}" ; \
			HELM_OPTION+=" --set clusterDefaultPool.ipv6Gateway=$${ipv6_gateway}" ; \
			HELM_OPTION+=" --set clusterDefaultPool.ipv6IPRanges={$${ipv6_ip_range}}" ; \
			HELM_OPTION+=" --set ipam.enableIPv4=false --set ipam.enableIPv6=true" ; \
		else \
			ipv4_prefix=$$(echo $${ipv4_subnet} | awk '{split($$0,a,".");printf "%s.%s",a[1],a[2]}') ; \
			ipv4_ip_range=$${ipv4_prefix}.40.2-$${ipv4_prefix}.40.254 ; \
			ipv6_ip_range=$${ipv6_subnet%::*}:f::2-$${ipv6_subnet%::*}:f::fe ; \
			HELM_OPTION+=" --set clusterDefaultPool.installIPv4IPPool=true --set clusterDefaultPool.installIPv6IPPool=true" ; \
			HELM_OPTION+=" --set clusterDefaultPool.ipv4IPPoolName=default-v4-ippool --set clusterDefaultPool.ipv6IPPoolName=default-v6-ippool" ; \
			HELM_OPTION+=" --set clusterDefaultPool.ipv4Subnet=$${ipv4_subnet} --set clusterDefaultPool.ipv6Subnet=$${ipv6_subnet}" ; \
			HELM_OPTION+=" --set clusterDefaultPool.ipv4Gateway=$${ipv4_gateway} --set clusterDefaultPool.ipv6Gateway=$${ipv6_gateway} " ; \
			HELM_OPTION+=" --set clusterDefaultPool.ipv4IPRanges={$${ipv4_ip_range}} --set clusterDefaultPool.ipv6IPRanges={$${ipv6_ip_range}}" ; \
			HELM_OPTION+=" --set ipam.enableIPv4=true --set ipam.enableIPv6=true" ; \
		fi ; \
		HELM_OPTION+=" --set spiderpoolAgent.prometheus.enabled=true --set spiderpoolController.prometheus.enabled=true " ; \
		HELM_OPTION+=" --set spiderpoolAgent.prometheus.enabledDebugMetric=true --set spiderpoolController.prometheus.enabledDebugMetric=true " ; \
		if [ -n "$(PYROSCOPE_LOCAL_PORT)" ] ; then \
        			docker stop $(PYROSCOPE_CONTAINER_NAME) &>/dev/null || true ; \
        			docker rm $(PYROSCOPE_CONTAINER_NAME) &>/dev/null || true ; \
        			ServerAddress=$$(docker network inspect kind -f {{\(index\ $$.IPAM.Config\ 0\).Gateway}}) ; \
        			echo "setup pyroscope on $${ServerAddress}:$(PYROSCOPE_LOCAL_PORT)" ; \
        			docker run -d --name $(PYROSCOPE_CONTAINER_NAME) -p $(PYROSCOPE_LOCAL_PORT):4040 $(IMAGE_PYROSCOPE_NAME) server ; \
        			echo "set env to spiderpool " ; \
        			HELM_OPTION+=" --set spiderpoolController.extraEnv[0].name=SPIDERPOOL_PYROSCOPE_PUSH_SERVER_ADDRESS  --set spiderpoolController.extraEnv[0].value=http://$${ServerAddress}:$(PYROSCOPE_LOCAL_PORT) " ; \
        			HELM_OPTION+=" --set spiderpoolAgent.extraEnv[0].name=SPIDERPOOL_PYROSCOPE_PUSH_SERVER_ADDRESS  --set spiderpoolAgent.extraEnv[0].value=http://$${ServerAddress}:$(PYROSCOPE_LOCAL_PORT) " ; \
        			echo "finish setuping pyroscope " ; \
          	 	fi ; \
		HELM_OPTION+=" --set spiderpoolController.replicas=2 " ; \
		HELM_OPTION+=" --set spiderpoolAgent.prometheus.serviceMonitor.install=true --set spiderpoolAgent.prometheus.prometheusRule.install=true --set spiderpoolAgent.prometheus.grafanaDashboard.install=true " ; \
		HELM_OPTION+=" --set spiderpoolController.prometheus.serviceMonitor.install=true --set spiderpoolController.prometheus.prometheusRule.install=true --set spiderpoolController.prometheus.grafanaDashboard.install=true " ; \
		HELM_OPTION+=" --set plugins.installCNI=true " ; \
		HELM_OPTION+=" --set plugins.installOvsCNI=true " ; \
		HELM_OPTION+=" 	--set spiderpoolAgent.image.registry="" \
		--set spiderpoolAgent.image.repository=$(SPIDERPOOL_AGENT_IMAGE_NAME) \
		--set spiderpoolAgent.image.tag=$(E2E_SPIDERPOOL_TAG) \
		--set spiderpoolController.image.registry="" \
		--set spiderpoolController.image.repository=$(SPIDERPOOL_CONTROLLER_IMAGE_NAME) \
		--set spiderpoolController.image.tag=$(E2E_SPIDERPOOL_TAG) \
		--set spiderpoolInit.image.registry="" \
		--set spiderpoolInit.image.repository=$(SPIDERPOOL_CONTROLLER_IMAGE_NAME) \
		--set spiderpoolInit.image.tag=$(E2E_SPIDERPOOL_TAG) \
		--set rdma.rdmaSharedDevicePlugin.image.registry=$(E2E_RDMA_DP_IMAGE_REPO) \
		--set sriov.image.registry=$(E2E_SRIOV_IMAGE_REPO)   " \
		ALL_IMAGES=`helm template $(RELEASE_NAME) $(ROOT_DIR)/charts/spiderpool $${HELM_OPTION} | grep ' image: ' | tr -d '"' | awk -F 'image: ' '{print $$2}' | sort | uniq | tr '\n' ' '` ; \
		echo "ALL_IMAGES: $${ALL_IMAGES} " ; \
		for IMAGE in $${ALL_IMAGES}; do \
				if ! grep "$${IMAGE}" <<< `docker images | awk '{printf("%s:%s\n",$$1,$$2)}'`; then \
						echo "==> $${IMAGE} no found, pulling...." ; \
						docker pull $${IMAGE} ; \
				fi ; \
				kind load docker-image $${IMAGE}  --name $(E2E_CLUSTER_NAME); \
		done ; \
		echo "setup spiderpool with image $(SPIDERPOOL_AGENT_IMAGE_NAME):$(E2E_SPIDERPOOL_TAG) and $(SPIDERPOOL_CONTROLLER_IMAGE_NAME):$(E2E_SPIDERPOOL_TAG) " ; \
		set -x ; \
		helm upgrade --install $(RELEASE_NAME) $(ROOT_DIR)/charts/spiderpool --wait --debug \
			-n $(RELEASE_NAMESPACE)  \
			$${HELM_OPTION}  \
			$(E2E_HELM_ADDITIONAL_OPTIONS) \
			--kubeconfig $(E2E_KUBECONFIG)  || { KIND_CLUSTER_NAME=$(E2E_CLUSTER_NAME) ./scripts/debugEnv.sh $(E2E_KUBECONFIG) "detail" "$(E2E_LOG_FILE)" ; exit 1 ; } ; \
		if [ "$(INSTALL_SRIOV)" == "true" ] ; then \
		    echo "label node for sriov operator " ; \
		    kubectl --kubeconfig $(E2E_KUBECONFIG) get node | sed '1d' | awk '{print $$1}' | xargs -n 1 -i kubectl --kubeconfig $(E2E_KUBECONFIG) label node {}  node-role.kubernetes.io/worker="" ; \
		fi ; \
		exit 0

.PHONY: clean_spiderpool_crd
clean_spiderpool_crd:
	@echo -e "\033[35m [clean spiderpool crd] \033[0m"
	@-$(QUIET) export KUBECONFIG=$(E2E_KUBECONFIG); $(ROOT_DIR)/tools/scripts/cleanCRD.sh
	@ALL_CRD=`kubectl get crd -n $(RELEASE_NAMESPACE) --kubeconfig $(E2E_KUBECONFIG)  | awk '{print $$1 } ' ` ; FAIL="false" ; \
	for CRD in $$ALL_CRD ; do \
		echo $${CRD} | grep -q -E 'spiderpool.spidernet.io|sriovnetwork.openshift.io' && FAIL="true" && echo "error, failed to clean spiderpool crd: $${CRD}" ; \
	    if [ "$$FAIL" == "true" ] ; then \
           echo -e "\033[35m [===== describe crd: $${CRD} =====] \033[0m" ; \
		   kubectl describe crd $${CRD} --kubeconfig $(E2E_KUBECONFIG) ; \
		   echo -e "\033[35m [===== get crd: $${CRD} -ojson =====] \033[0m" ; \
           kubectl get crd $${CRD} -ojson --kubeconfig $(E2E_KUBECONFIG) ; \
		fi ; \
	done ; \
	[ "$$FAIL" == "true" ] && echo "spiderpool crd cleanup failed" && exit 1 ; \
	echo -e "\033[35m [done] all spiderpool crd cleaned up !!! \033[0m"

.PHONY: uninstall_spiderpool
uninstall_spiderpool:
	@echo -e "\033[35m [helm uninstall spiderpool] \033[0m"
	helm uninstall $(RELEASE_NAME) --wait --debug -n $(RELEASE_NAMESPACE) \
	     --kubeconfig $(E2E_KUBECONFIG)  || { KIND_CLUSTER_NAME=$(E2E_CLUSTER_NAME) ./scripts/debugEnv.sh $(E2E_KUBECONFIG) "detail"   ; exit 1 ; } ; \
	echo "Multus config has been cleanup successfully." ; \
	for ((i=0; i<100; i++)); do \
    	if ! kubectl --kubeconfig=$(E2E_KUBECONFIG) get all --all-namespaces | grep -q "spiderpool" ; then \
			echo "All Spiderpool Pod successfully cleared." ; \
			exit 0; \
		fi ;\
		echo "found spiderpool pod, waiting..." ; \
		sleep 1 ; \
	done; \
	make clean_spiderpool_crd ; \
	echo "error: found spiderpool resources" ; exit 1 ;
    @echo "check cni conf removed from nodes: $(POST_UNINSTALL_CHECK_CNI_CONF) " ; \
		NODE_LIST=` docker ps | egrep "kindest/node.* $(E2E_CLUSTER_NAME)-(control|worker)" | awk '{print $$1 }' ` ; \
		[ -n "$$NODE_LIST" ] || { echo "error, failed to find any kind nodes, please setup kind cluster $(E2E_CLUSTER_NAME) first" ; exit 1 ; } ; \
		for NODE in $${NODE_LIST} ; do \
			ALL_CONF=`docker exec $${NODE} ls /etc/cni/net.d ` || { echo "error, failed to get cni conf /etc/cni/net.d on node $${NODE} " ; exit 1 ; }; \
			ALL_CONF=` echo "$${ALL_CONF}" | tr '\n' ' ' ` ; \
			echo "all cni conf on node $${NODE}: $${ALL_CONF} " ; \
			for ITEM in $(POST_UNINSTALL_CHECK_CNI_CONF) ; do \
			     grep " $${ITEM} " <<< " $${ALL_CONF} " &>/dev/null && { echo "error, found $${ITEM} under /etc/cni/net.d on node $${NODE} " ; exit 1 ; } ; \
			done ; \
		done

.PHONY: upgrade_spiderpool
upgrade_spiderpool:
	@echo -e "\033[35m [upgrade spiderpool] \033[0m"
	@make helm_upgrade_spiderpool

.PHONY: helm_upgrade_spiderpool
helm_upgrade_spiderpool:
	@echo -e "\033[35m [helm upgrade spiderpool] \033[0m"
	kubectl delete po -n $(RELEASE_NAMESPACE) spiderpool-init --kubeconfig $(E2E_KUBECONFIG) || true ;\
	HELM_OPTION="";\
	HELM_OPTION+=" --set spiderpoolController.replicas=1 " ; \
	if [ "$(INSTALL_OVERLAY_CNI)" == "true" ]; then \
		HELM_OPTION+=" --set multus.multusCNI.defaultCniCRName= " ; \
	else \
		HELM_OPTION+=" --set multus.multusCNI.defaultCniCRName=$(MULTUS_DEFAULT_CNI_VLAN0) " ; \
	fi ; \
	HELM_OPTION+=" 	--set spiderpoolAgent.image.registry="" \
	--set spiderpoolAgent.image.repository=$(SPIDERPOOL_AGENT_IMAGE_NAME) \
	--set spiderpoolAgent.image.tag=$(E2E_SPIDERPOOL_TAG) \
	--set spiderpoolController.image.registry="" \
	--set spiderpoolController.image.repository=$(SPIDERPOOL_CONTROLLER_IMAGE_NAME) \
	--set spiderpoolController.image.tag=$(E2E_SPIDERPOOL_TAG) \
	--set spiderpoolInit.image.registry="" \
	--set spiderpoolInit.image.repository=$(SPIDERPOOL_CONTROLLER_IMAGE_NAME) \
	--set spiderpoolInit.image.tag=$(E2E_SPIDERPOOL_TAG) \
	--set multus.multusCNI.uninstall=false " \
	ALL_IMAGES=`helm template $(RELEASE_NAME) $(ROOT_DIR)/charts/spiderpool $${HELM_OPTION} | grep ' image: ' | tr -d '"' | awk -F 'image: ' '{print $$2}' | sort | uniq | tr '\n' ' '` ; \
	echo "ALL_IMAGES: $${ALL_IMAGES} " ; \
	for IMAGE in $${ALL_IMAGES}; do \
			if ! grep "$${IMAGE}" <<< `docker images | awk '{printf("%s:%s\n",$$1,$$2)}'`; then \
					echo "==> $${IMAGE} no found, pulling...." ; \
					docker pull $${IMAGE} ; \
			fi ; \
			kind load docker-image $${IMAGE}  --name $(E2E_CLUSTER_NAME); \
	done ; \
	echo "upgrade spiderpool with image $(SPIDERPOOL_AGENT_IMAGE_NAME):$(E2E_SPIDERPOOL_TAG) and $(SPIDERPOOL_CONTROLLER_IMAGE_NAME):$(E2E_SPIDERPOOL_TAG) " ; \
	set -x ; \
	helm --kubeconfig $(E2E_KUBECONFIG) upgrade $(RELEASE_NAME) $(ROOT_DIR)/charts/spiderpool \
	    $${HELM_OPTION} \
		-n $(RELEASE_NAMESPACE) --debug --reuse-values ; \
    cd $(ROOT_DIR)/charts/spiderpool/crds ; \
	ls | grep '\.yaml$$' | xargs -I {} kubectl apply -f {} --kubeconfig $(E2E_KUBECONFIG) ; \
	kubectl wait --for=condition=ready -l app.kubernetes.io/instance=spiderpool --timeout=300s pod -n $(RELEASE_NAMESPACE) --kubeconfig $(E2E_KUBECONFIG) || true; \
	kubectl scale deploy -n $(RELEASE_NAMESPACE) -l app.kubernetes.io/component=spiderpool-controller --replicas=2 --kubeconfig $(E2E_KUBECONFIG); \
	kubectl wait --for=condition=ready -l app.kubernetes.io/component=spiderpool-controller --timeout=300s pod -n $(RELEASE_NAMESPACE) --kubeconfig $(E2E_KUBECONFIG) || true; \
	helm --kubeconfig $(E2E_KUBECONFIG) list -A ; \

.PHONY: clean
clean:
	@rm -rf $(CLUSTER_DIR)
	-@  kind get clusters | xargs -n1  kind delete cluster --name
	-@ docker stop $(PYROSCOPE_CONTAINER_NAME) &>/dev/null
	-@ docker rm $(PYROSCOPE_CONTAINER_NAME) &>/dev/null
	-@ docker stop $(VLAN_GATEWAY_CONTAINER) &>/dev/null
	-@ docker rm $(VLAN_GATEWAY_CONTAINER) &>/dev/null

.PHONY: net-tools
net-tools:
	@NODE_LIST=` docker ps | egrep "kindest/node.* $(E2E_CLUSTER_NAME)-(control|worker)" | awk '{print $$1 }' ` ; \
	[ -n "$$NODE_LIST" ] || { echo "error, failed to find any kind nodes, please setup kind cluster $(E2E_CLUSTER_NAME) first" ; exit 1 ; } ; \
	for NODE in $${NODE_LIST} ; do \
	  docker cp scripts/install-net-tools.sh $${NODE}:/home/ ; \
	  docker exec $${NODE} chmod +x /home/install-net-tools.sh ; \
	  docker exec $${NODE} ./home/install-net-tools.sh ; \
	done

.PHONY: check_env
check_env:
	$(QUIET) [ -n "$(E2E_CLUSTER_NAME)" ] || { echo "error, miss E2E_CLUSTER_NAME " ; false ; }
	$(QUIET) ( [ "$(E2E_IP_FAMILY)" == "ipv4" ] || [ "$(E2E_IP_FAMILY)" == "ipv6" ]  || [ "$(E2E_IP_FAMILY)" == "dual" ] ) \
			|| { echo "error, E2E_IP_FAMILY=$(E2E_IP_FAMILY) must be ipv4/ipv6/dual" ;  exit 1 ; }


.PHONY: prepare
prepare:
	@echo -e "\033[35m [Step 2] Check The Tools For Ready: \033[0m"
	$(QUEIT) JUST_CLI_CHECK=true scripts/install-tools.sh
	$(QUEIT) mkdir -p $(DOWNLOAD_DIR)
	$(QUEIT) IMAGE_LIST="" ; \
		 [ "$(USE_TLS_METHOD)" == "certmanager" ] && IMAGE_LIST+=" $(IMAGE_CERT_MANAGER_NAME) " ; \
		 [ -n "$(PYROSCOPE_LOCAL_PORT)" ] && IMAGE_LIST+=" $(IMAGE_PYROSCOPE_NAME) " ; \
		 IMAGE_LIST+=" $(TEST_IMAGE_NAME) " ; \
		 IMAGE_LIST+=" $(E2E_VLAN_GATEWAY_IMAGE) " ; \
		 ARCH=$(ARCH) IMAGE_LIST="$${IMAGE_LIST}" \
		 	CNI_PACKAGE_VERSION=$(CNI_PACKAGE_VERSION)  scripts/prepare.sh $(DOWNLOAD_DIR)

#============ e2e ====================
.PHONY: e2e_test
e2e_test:
	@echo -e "\033[35m Run e2e test on the cluster $(E2E_CLUSTER_NAME) \033[0m "
	@ echo -e "\033[35m [E2E] Run E2E with ginkgo label=$(E2E_GINKGO_LABELS) , timeout=$(E2E_TIMEOUT) GINKGO_OPTION=$(GINKGO_OPTION) \033[0m"
	NODE_LIST=` docker ps | egrep "kindest/node.* $(E2E_CLUSTER_NAME)-(control|worker)" | awk '{print $$NF }' ` ; \
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
		export RELEASE_NAMESPACE=$(RELEASE_NAMESPACE) ; \
		export INSTALL_OVERLAY_CNI=$(INSTALL_OVERLAY_CNI) ; \
		export E2E_SPIDERSUBNET_ENABLED=$(E2E_SPIDERPOOL_ENABLE_SUBNET) ; \
		rm -f $(E2E_LOG_FILE) || true  ; \
		echo "=========== before test `date` ===========" >> $(E2E_LOG_FILE) ; \
		./scripts/debugEnv.sh $(E2E_KUBECONFIG) "system" "$(E2E_LOG_FILE)" ; \
		RESULT=0 ; \
		$(ROOT_DIR)/tools/scripts/ginkgo.sh \
			--race --timeout=$(E2E_TIMEOUT) --output-interceptor-mode=none --poll-progress-after=120s --poll-progress-interval=30s \
			--json-report e2ereport.json --output-dir $(ROOT_DIR) --procs $(E2E_GINKGO_PROCS) -gcflags=-l \
			--label-filter="$(E2E_GINKGO_LABELS)" -randomize-suites -randomize-all  -vv --fail-fast  $(GINKGO_OPTION) \
			-r e2e/*  || RESULT=1  ; \
		echo "=========== after test `date` ===========" >> $(E2E_LOG_FILE) ; \
		./scripts/debugEnv.sh $(E2E_KUBECONFIG) "system" "$(E2E_LOG_FILE)" ; \
		KIND_CLUSTER_NAME=$(E2E_CLUSTER_NAME) ./scripts/debugEnv.sh $(E2E_KUBECONFIG) "detail" "$(E2E_LOG_FILE)" ; \
		./scripts/debugEnv.sh $(E2E_KUBECONFIG) "error" "$(E2E_LOG_FILE)" || { echo "error, found error log !!!" ; RESULT=1 ; } ; \
		if (($${RESULT} != 0)) ; then \
		   echo "failed to run e2e test"  ; \
		   exit 1 ; \
		fi ; \
		echo "" ; \
		echo "============================================" ; \
		echo "succeeded to run all test" ; \
		echo "output report to e2ereport.json" ; \
		echo "output env log to $(E2E_LOG_FILE) "

.PHONY: usage
usage:
	@echo "usage:"
	@echo -e "  \033[35m make prepare \033[0m:       --- Check some required tools is exist like docker/helm.etc and download cni-plugins"
	@echo -e "  \033[35m make init \033[0m:          --- Setup a kind cluster, Such as: kind-init E2E_CLUSTER_NAME=spider,More config refer to Makefile.defs(e2e-kind-config)"
	@echo -e "  \033[35m make e2e-test \033[0m:      --- Ginkgo test,Such as: make e2e-test, More config refer to Makefile.defs(e2e-kind-config)"
	@echo -e "  \033[35m make clean \033[0m:    --- Clean kind cluster and some config file, Such as: make clean E2E_CLUSTER_NAME=spider"
	@echo -e "  \033[35m make e2e \033[0m:           --- prepare -> kind-init -> e2e-test "
