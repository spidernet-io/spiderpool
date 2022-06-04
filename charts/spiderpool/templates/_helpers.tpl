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
{{- default "spiderpool" .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Expand the name of spiderpoolController .
*/}}
{{- define "spiderpool.spiderpoolController.name" -}}
{{- default "spiderpoolcontroller" .Values.spiderpoolController.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Expand the name of spiderpoolAgent .
*/}}
{{- define "spiderpool.spiderpoolAgent.name" -}}
{{- default "spiderpoolagent" .Values.spiderpoolAgent.nameOverride | trunc 63 | trimSuffix "-" }}
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
app.kubernetes.io/component: {{ include "spiderpool.spiderpoolController.name" . }}
{{- end }}

{{/*
spiderpoolAgent Selector labels
*/}}
{{- define "spiderpool.spiderpoolAgent.selectorLabels" -}}
app.kubernetes.io/name: {{ include "spiderpool.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: {{ include "spiderpool.spiderpoolAgent.name" . }}
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

