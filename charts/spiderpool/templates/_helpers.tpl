{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "spiderpool.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Expand the name of spiderpool .
*/}}
{{- define "spiderpool.name" -}}
{{- default "spiderpool" .Values.global.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "spiderpool.spiderpoolController.labels" -}}
helm.sh/chart: {{ include "spiderpool.chart" . }}
{{ include "spiderpool.spiderpoolController.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "spiderpool.spiderpoolInit.labels" -}}
helm.sh/chart: {{ include "spiderpool.chart" . }}
{{ include "spiderpool.spiderpoolInit.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
spiderpoolAgent Common labels
*/}}
{{- define "spiderpool.spiderpoolAgent.labels" -}}
helm.sh/chart: {{ include "spiderpool.chart" . }}
{{ include "spiderpool.spiderpoolAgent.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}


{{/*
spiderpoolController Selector labels
*/}}
{{- define "spiderpool.spiderpoolController.selectorLabels" -}}
app.kubernetes.io/name: {{ include "spiderpool.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: {{ .Values.spiderpoolController.name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
spiderpoolAgent Selector labels
*/}}
{{- define "spiderpool.spiderpoolAgent.selectorLabels" -}}
app.kubernetes.io/name: {{ include "spiderpool.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: {{ .Values.spiderpoolAgent.name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
spiderpoolInit Selector labels
*/}}
{{- define "spiderpool.spiderpoolInit.selectorLabels" -}}
app.kubernetes.io/name: {{ include "spiderpool.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: {{ .Values.spiderpoolInit.name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/* vim: set filetype=mustache: */}}
{{/*
Renders a value that contains template.
Usage:
{{ include "tplvalues.render" ( dict "value" .Values.path.to.the.Value "context" $) }}
*/}}
{{- define "tplvalues.render" -}}
    {{- if typeIs "string" .value }}
        {{- tpl .value .context }}
    {{- else }}
        {{- tpl (.value | toYaml) .context }}
    {{- end }}
{{- end -}}

{{/*
Return the appropriate apiVersion for poddisruptionbudget.
*/}}
{{- define "capabilities.policy.apiVersion" -}}
{{- if semverCompare "<1.21-0" .Capabilities.KubeVersion.Version -}}
{{- print "policy/v1beta1" -}}
{{- else -}}
{{- print "policy/v1" -}}
{{- end -}}
{{- end -}}

{{/*
Return the appropriate apiVersion for deployment.
*/}}
{{- define "capabilities.deployment.apiVersion" -}}
{{- if semverCompare "<1.14-0" .Capabilities.KubeVersion.Version -}}
{{- print "extensions/v1beta1" -}}
{{- else -}}
{{- print "apps/v1" -}}
{{- end -}}
{{- end -}}

{{/*
Return the appropriate apiVersion for RBAC resources.
*/}}
{{- define "capabilities.rbac.apiVersion" -}}
{{- if semverCompare "<1.17-0" .Capabilities.KubeVersion.Version -}}
{{- print "rbac.authorization.k8s.io/v1beta1" -}}
{{- else -}}
{{- print "rbac.authorization.k8s.io/v1" -}}
{{- end -}}
{{- end -}}

{{/*
return the spiderpoolAgent image
*/}}
{{- define "spiderpool.spiderpoolAgent.image" -}}
{{- $registryName := .Values.spiderpoolAgent.image.registry -}}
{{- $repositoryName := .Values.spiderpoolAgent.image.repository -}}
{{- if .Values.global.imageRegistryOverride }}
    {{- printf "%s/%s" .Values.global.imageRegistryOverride $repositoryName -}}
{{ else if $registryName }}
    {{- printf "%s/%s" $registryName $repositoryName -}}
{{- else -}}
    {{- printf "%s" $repositoryName -}}
{{- end -}}
{{- if .Values.spiderpoolAgent.image.digest }}
    {{- print "@" .Values.spiderpoolAgent.image.digest -}}
{{- else if .Values.spiderpoolAgent.image.tag -}}
    {{- printf ":%s" .Values.spiderpoolAgent.image.tag -}}
{{- else -}}
    {{- printf ":v%s" .Chart.AppVersion -}}
{{- end -}}
{{- end -}}

{{/*
return the spiderpoolController image
*/}}
{{- define "spiderpool.spiderpoolController.image" -}}
{{- $registryName := .Values.spiderpoolController.image.registry -}}
{{- $repositoryName := .Values.spiderpoolController.image.repository -}}
{{- if .Values.global.imageRegistryOverride }}
    {{- printf "%s/%s" .Values.global.imageRegistryOverride $repositoryName -}}
{{ else if $registryName }}
    {{- printf "%s/%s" $registryName $repositoryName -}}
{{- else -}}
    {{- printf "%s" $repositoryName -}}
{{- end -}}
{{- if .Values.spiderpoolController.image.digest }}
    {{- print "@" .Values.spiderpoolController.image.digest -}}
{{- else if .Values.spiderpoolController.image.tag -}}
    {{- printf ":%s" .Values.spiderpoolController.image.tag -}}
{{- else -}}
    {{- printf ":v%s" .Chart.AppVersion -}}
{{- end -}}
{{- end -}}

{{/*
return the spiderpoolInit image
*/}}
{{- define "spiderpool.spiderpoolInit.image" -}}
{{- $registryName := .Values.spiderpoolInit.image.registry -}}
{{- $repositoryName := .Values.spiderpoolInit.image.repository -}}
{{- if .Values.global.imageRegistryOverride }}
    {{- printf "%s/%s" .Values.global.imageRegistryOverride $repositoryName -}}
{{ else if $registryName }}
    {{- printf "%s/%s" $registryName $repositoryName -}}
{{- else -}}
    {{- printf "%s" $repositoryName -}}
{{- end -}}
{{- if .Values.spiderpoolInit.image.digest }}
    {{- print "@" .Values.spiderpoolInit.image.digest -}}
{{- else if .Values.spiderpoolInit.image.tag -}}
    {{- printf ":%s" .Values.spiderpoolInit.image.tag -}}
{{- else -}}
    {{- printf ":v%s" .Chart.AppVersion -}}
{{- end -}}
{{- end -}}

{{/*
generate the CA cert
*/}}
{{- define "generate-ca-certs" }}
    {{- $ca := genCA "spidernet.io" (.Values.spiderpoolController.tls.auto.caExpiration | int) -}}
    {{- $_ := set . "ca" $ca -}}
{{- end }}

#=================== multus =====================

{{/*
return the multus image
*/}}
{{- define "spiderpool.multus.image" -}}
{{- $registryName := .Values.multus.multusCNI.image.registry -}}
{{- $repositoryName := .Values.multus.multusCNI.image.repository -}}
{{- if .Values.global.imageRegistryOverride }}
    {{- printf "%s/%s" .Values.global.imageRegistryOverride $repositoryName -}}
{{- else if $registryName }}
    {{- printf "%s/%s" $registryName $repositoryName -}}
{{- else -}}
    {{- printf "%s" $repositoryName -}}
{{- end -}}
{{- if .Values.multus.multusCNI.image.digest }}
    {{- print "@" .Values.multus.multusCNI.image.digest -}}
{{- else if .Values.multus.multusCNI.image.tag -}}
    {{- printf ":%s" .Values.multus.multusCNI.image.tag -}}
{{- end -}}
{{- end -}}

{{/*
spiderpool multus Common labels
*/}}
{{- define "spiderpool.multus.labels" -}}
helm.sh/chart: {{ include "spiderpool.chart" . }}
{{ include "spiderpool.multus.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
tier: node
app: multus
{{- end }}

{{/*
spiderpool multus Selector labels
*/}}
{{- define "spiderpool.multus.selectorLabels" -}}
app.kubernetes.io/name: {{ include "spiderpool.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: {{ .Values.multus.multusCNI.name | trunc 63 | trimSuffix "-" }}
name: multus
{{- end }}

#=================== plugins =====================

{{/*
return the plugins image
*/}}
{{- define "plugins.image" -}}
{{- $registryName := .Values.plugins.image.registry -}}
{{- $repositoryName := .Values.plugins.image.repository -}}
{{- if .Values.global.imageRegistryOverride }}
    {{- printf "%s/%s" .Values.global.imageRegistryOverride $repositoryName -}}
{{ else if $registryName }}
    {{- printf "%s/%s" $registryName $repositoryName -}}
{{- else -}}
    {{- printf "%s" $repositoryName -}}
{{- end -}}
{{- if .Values.plugins.image.digest }}
    {{- print "@" .Values.plugins.image.digest -}}
{{- else if .Values.plugins.image.tag -}}
    {{- printf ":%s" .Values.plugins.image.tag -}}
{{- else -}}
    {{- printf ":v%s" .Chart.AppVersion -}}
{{- end -}}
{{- end -}}

{{/*
return the rdma shared device plugin
*/}}
{{- define "rdmashareddp.image" -}}
{{- $registryName := .Values.rdma.rdmaSharedDevicePlugin.image.registry -}}
{{- $repositoryName := .Values.rdma.rdmaSharedDevicePlugin.image.repository -}}
{{- if .Values.global.imageRegistryOverride }}
    {{- printf "%s/%s" .Values.global.imageRegistryOverride $repositoryName -}}
{{ else if $registryName }}
    {{- printf "%s/%s" $registryName $repositoryName -}}
{{- else -}}
    {{- printf "%s" $repositoryName -}}
{{- end -}}
{{- if .Values.rdma.rdmaSharedDevicePlugin.image.digest }}
    {{- print "@" .Values.rdma.rdmaSharedDevicePlugin.image.digest -}}
{{- else if .Values.rdma.rdmaSharedDevicePlugin.image.tag -}}
    {{- printf ":%s" .Values.rdma.rdmaSharedDevicePlugin.image.tag -}}
{{- else -}}
    {{- printf ":v%s" .Chart.AppVersion -}}
{{- end -}}
{{- end -}}

{{/*
spiderpool rdma shared device plugin Common labels
*/}}
{{- define "spiderpool.rdmashareddp.labels" -}}
helm.sh/chart: {{ include "spiderpool.chart" . }}
{{ include "spiderpool.rdmashareddp.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
tier: node
app: rdma-shared-device=plugin
{{- end }}

{{/*
spiderpool rdma shared device plugin Selector labels
*/}}
{{- define "spiderpool.rdmashareddp.selectorLabels" -}}
app.kubernetes.io/name: {{ include "spiderpool.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: {{ .Values.rdma.rdmaSharedDevicePlugin.name | trunc 63 | trimSuffix "-" }}
name: multus
{{- end }}

#=================== sriov =====================

{{/*
Common labels
*/}}
{{- define "sriov.operator.labels" -}}
helm.sh/chart: {{ include "spiderpool.chart" . }}
{{ include "sriov.operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}


{{/*
Selector labels
*/}}
{{- define "sriov.operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "spiderpool.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: {{ .Values.sriov.name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
return the sriov network operator image
*/}}
{{- define "sriov.operator.image" -}}
{{- $registryName := .Values.sriov.image.registry -}}
{{- $repositoryName := .Values.sriov.image.operator.repository -}}
{{- if .Values.global.imageRegistryOverride }}
    {{- printf "%s/%s" .Values.global.imageRegistryOverride $repositoryName -}}
{{ else if $registryName }}
    {{- printf "%s/%s" $registryName $repositoryName -}}
{{- else -}}
    {{- printf "%s" $repositoryName -}}
{{- end -}}
{{- if .Values.sriov.image.operator.tag -}}
    {{- printf ":%s" .Values.sriov.image.operator.tag -}}
{{- else -}}
    {{- printf ":%s" "latest" -}}
{{- end -}}
{{- end -}}

{{/*
return the sriov cni image
*/}}
{{- define "sriov.sriovCni.image" -}}
{{- $registryName := .Values.sriov.image.registry -}}
{{- $repositoryName := .Values.sriov.image.sriovCni.repository -}}
{{- if .Values.global.imageRegistryOverride }}
    {{- printf "%s/%s" .Values.global.imageRegistryOverride $repositoryName -}}
{{ else if $registryName }}
    {{- printf "%s/%s" $registryName $repositoryName -}}
{{- else -}}
    {{- printf "%s" $repositoryName -}}
{{- end -}}
{{- if .Values.sriov.image.sriovCni.tag -}}
    {{- printf ":%s" .Values.sriov.image.sriovCni.tag -}}
{{- else -}}
    {{- printf ":%s" "latest" -}}
{{- end -}}
{{- end -}}

{{/*
return the sriov ibSriovCni image
*/}}
{{- define "sriov.ibSriovCni.image" -}}
{{- $registryName := .Values.sriov.image.registry -}}
{{- $repositoryName := .Values.sriov.image.ibSriovCni.repository -}}
{{- if .Values.global.imageRegistryOverride }}
    {{- printf "%s/%s" .Values.global.imageRegistryOverride $repositoryName -}}
{{ else if $registryName }}
    {{- printf "%s/%s" $registryName $repositoryName -}}
{{- else -}}
    {{- printf "%s" $repositoryName -}}
{{- end -}}
{{- if .Values.sriov.image.ibSriovCni.tag -}}
    {{- printf ":%s" .Values.sriov.image.ibSriovCni.tag -}}
{{- else -}}
    {{- printf ":%s" "latest" -}}
{{- end -}}
{{- end -}}

{{/*
return the sriov sriovDevicePlugin image
*/}}
{{- define "sriov.sriovDevicePlugin.image" -}}
{{- $registryName := .Values.sriov.image.registry -}}
{{- $repositoryName := .Values.sriov.image.sriovDevicePlugin.repository -}}
{{- if .Values.global.imageRegistryOverride }}
    {{- printf "%s/%s" .Values.global.imageRegistryOverride $repositoryName -}}
{{ else if $registryName }}
    {{- printf "%s/%s" $registryName $repositoryName -}}
{{- else -}}
    {{- printf "%s" $repositoryName -}}
{{- end -}}
{{- if .Values.sriov.image.sriovDevicePlugin.tag -}}
    {{- printf ":%s" .Values.sriov.image.sriovDevicePlugin.tag -}}
{{- else -}}
    {{- printf ":%s" "latest" -}}
{{- end -}}
{{- end -}}

{{/*
return the sriov resourcesInjector image
*/}}
{{- define "sriov.resourcesInjector.image" -}}
{{- $registryName := .Values.sriov.image.registry -}}
{{- $repositoryName := .Values.sriov.image.resourcesInjector.repository -}}
{{- if .Values.global.imageRegistryOverride }}
    {{- printf "%s/%s" .Values.global.imageRegistryOverride $repositoryName -}}
{{ else if $registryName }}
    {{- printf "%s/%s" $registryName $repositoryName -}}
{{- else -}}
    {{- printf "%s" $repositoryName -}}
{{- end -}}
{{- if .Values.sriov.image.resourcesInjector.tag -}}
    {{- printf ":%s" .Values.sriov.image.resourcesInjector.tag -}}
{{- else -}}
    {{- printf ":%s" "latest" -}}
{{- end -}}
{{- end -}}

{{/*
return the sriov sriovConfigDaemon image
*/}}
{{- define "sriov.sriovConfigDaemon.image" -}}
{{- $registryName := .Values.sriov.image.registry -}}
{{- $repositoryName := .Values.sriov.image.sriovConfigDaemon.repository -}}
{{- if .Values.global.imageRegistryOverride }}
    {{- printf "%s/%s" .Values.global.imageRegistryOverride $repositoryName -}}
{{ else if $registryName }}
    {{- printf "%s/%s" $registryName $repositoryName -}}
{{- else -}}
    {{- printf "%s" $repositoryName -}}
{{- end -}}
{{- if .Values.sriov.image.sriovConfigDaemon.tag -}}
    {{- printf ":%s" .Values.sriov.image.sriovConfigDaemon.tag -}}
{{- else -}}
    {{- printf ":%s" "latest" -}}
{{- end -}}
{{- end -}}

{{/*
return the sriov webhook image
*/}}
{{- define "sriov.webhook.image" -}}
{{- $registryName := .Values.sriov.image.registry -}}
{{- $repositoryName := .Values.sriov.image.webhook.repository -}}
{{- if .Values.global.imageRegistryOverride }}
    {{- printf "%s/%s" .Values.global.imageRegistryOverride $repositoryName -}}
{{ else if $registryName }}
    {{- printf "%s/%s" $registryName $repositoryName -}}
{{- else -}}
    {{- printf "%s" $repositoryName -}}
{{- end -}}
{{- if .Values.sriov.image.webhook.tag -}}
    {{- printf ":%s" .Values.sriov.image.webhook.tag -}}
{{- else -}}
    {{- printf ":%s" "latest" -}}
{{- end -}}
{{- end -}}

#========================================
