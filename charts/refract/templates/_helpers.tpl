{{/*
Chart name and version label, per Helm's standard label conventions.
*/}}
{{- define "refract.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "refract.fullname" -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "refract.selectorLabels" -}}
app.kubernetes.io/name: {{ include "refract.fullname" . }}
app.kubernetes.io/component: gateway
{{- end }}

{{- define "refract.labels" -}}
helm.sh/chart: {{ include "refract.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{ include "refract.selectorLabels" . }}
{{- end }}
