{{/* Expand the chart name. */}}
{{- define "proxy-vpn.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* Create a fully qualified app name. */}}
{{- define "proxy-vpn.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "proxy-vpn.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/name: {{ include "proxy-vpn.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "proxy-vpn.selectorLabels" -}}
app.kubernetes.io/name: {{ include "proxy-vpn.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/* image builder — per-BIN tag fallback */}}
{{- define "proxy-vpn.image" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag -}}
{{- if .override -}}
{{- .override -}}
{{- else -}}
{{- printf "%s/%s-%s:%s" .Values.image.registry .Values.image.repository .bin $tag -}}
{{- end -}}
{{- end -}}

{{- define "proxy-vpn.envSecretName" -}}
{{- if .Values.externalSecret.enabled -}}
{{- .Values.externalSecret.name -}}
{{- else -}}
{{- printf "%s-env" (include "proxy-vpn.fullname" .) -}}
{{- end -}}
{{- end -}}
