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
return the multus image
*/}}
{{- define "spiderpool.multus.image" -}}
{{- $registryName := .Values.multus.multusCNI.image.registry -}}
{{- $repositoryName := .Values.multus.multusCNI.image.repository -}}
{{- if .Values.global.imageRegistryOverride }}
    {{- printf "%s/%s" .Values.global.imageRegistryOverride $repositoryName -}}
{{ else if $registryName }}
    {{- printf "%s/%s" $registryName $repositoryName -}}
{{- else -}}
    {{- printf "%s" $repositoryName -}}
{{- end -}}
{{- if .Values.multus.multusCNI.image.digest }}
    {{- print "@" .Values.multus.multusCNI.image.digest -}}
{{- else if .Values.multus.multusCNI.image.tag -}}
    {{- printf ":%s" .Values.multus.multusCNI.image.tag -}}
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

{{/*
insight labels
*/}}
{{- define "insight.labels" -}}
operator.insight.io/managed-by: insight
{{- end}}

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