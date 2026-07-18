# @sk-task helm-chart#T1.1: Named templates for chart-wide labels and naming

{{- define "maskchain.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "maskchain.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "maskchain.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "maskchain.labels" -}}
helm.sh/chart: {{ include "maskchain.chart" . }}
{{ include "maskchain.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "maskchain.selectorLabels" -}}
app.kubernetes.io/name: {{ include "maskchain.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "maskchain.component.name" -}}
{{ printf "%s-%s" (include "maskchain.fullname" .) .component | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "maskchain.component.labels" -}}
{{ include "maskchain.labels" . }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{- define "maskchain.component.selectorLabels" -}}
{{ include "maskchain.selectorLabels" . }}
app.kubernetes.io/component: {{ .component }}
{{- end }}
