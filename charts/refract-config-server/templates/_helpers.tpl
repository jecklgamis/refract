{{/*
Chart name and version label, per Helm's standard label conventions.
*/}}
{{- define "refract-config-server.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "refract-config-server.fullname" -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "refract-config-server.selectorLabels" -}}
app.kubernetes.io/name: {{ include "refract-config-server.fullname" . }}
app.kubernetes.io/component: config-server
{{- end }}

{{- define "refract-config-server.labels" -}}
helm.sh/chart: {{ include "refract-config-server.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{ include "refract-config-server.selectorLabels" . }}
{{- end }}
