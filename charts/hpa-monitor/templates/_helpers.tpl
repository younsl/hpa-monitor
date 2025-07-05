{{/*
Expand the name of the chart.
*/}}
{{- define "hpa-monitor.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "hpa-monitor.fullname" -}}
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

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "hpa-monitor.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "hpa-monitor.labels" -}}
helm.sh/chart: {{ include "hpa-monitor.chart" . }}
{{ include "hpa-monitor.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.extraLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "hpa-monitor.selectorLabels" -}}
app.kubernetes.io/name: {{ include "hpa-monitor.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "hpa-monitor.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "hpa-monitor.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the cluster role to use
*/}}
{{- define "hpa-monitor.clusterRoleName" -}}
{{- include "hpa-monitor.fullname" . }}
{{- end }}

{{/*
Create the name of the cluster role binding to use
*/}}
{{- define "hpa-monitor.clusterRoleBindingName" -}}
{{- include "hpa-monitor.fullname" . }}
{{- end }}

{{/*
Common annotations
*/}}
{{- define "hpa-monitor.annotations" -}}
{{- with .Values.extraAnnotations }}
{{ toYaml . }}
{{- end }}
{{- end }}