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
{{- else -}}
    {{- $tag := default .Chart.AppVersion .Values.spiderpoolAgent.image.tag -}}
    {{- printf ":%s" $tag -}}
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
{{- else -}}
    {{- $tag := default .Chart.AppVersion .Values.spiderpoolController.image.tag -}}
    {{- printf ":%s" $tag -}}
{{- end -}}
{{- end -}}