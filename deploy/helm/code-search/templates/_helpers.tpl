{{/*
Expand the name of the chart.
*/}}
{{- define "code-search.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "code-search.fullname" -}}
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
{{- define "code-search.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "code-search.labels" -}}
helm.sh/chart: {{ include "code-search.chart" . }}
{{ include "code-search.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "code-search.selectorLabels" -}}
app.kubernetes.io/name: {{ include "code-search.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "code-search.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "code-search.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Get the PostgreSQL host
*/}}
{{- define "code-search.postgresql.host" -}}
{{- .Values.postgresql.host }}
{{- end }}

{{/*
Get the PostgreSQL port
*/}}
{{- define "code-search.postgresql.port" -}}
{{- .Values.postgresql.port }}
{{- end }}

{{/*
Get the Redis URL
*/}}
{{- define "code-search.redisUrl" -}}
{{- printf "%s:%d" .Values.redis.host (int (default 6379 .Values.redis.port)) }}
{{- end }}

{{/*
Image helper
*/}}
{{- define "code-search.image" -}}
{{- $registry := .global.imageRegistry | default "" -}}
{{- $repository := .image.repository -}}
{{- $tag := .image.tag | default .appVersion -}}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- else }}
{{- printf "%s:%s" $repository $tag }}
{{- end }}
{{- end }}

{{/*
Zoekt shard count (same as indexer since zoekt is a sidecar)
*/}}
{{- define "code-search.zoekt.replicas" -}}
{{- if .Values.sharding.enabled }}
{{- .Values.sharding.replicas }}
{{- else }}
{{- 1 }}
{{- end }}
{{- end }}

{{/*
Indexer shard count
*/}}
{{- define "code-search.indexer.replicas" -}}
{{- if .Values.sharding.enabled }}
{{- .Values.sharding.replicas }}
{{- else }}
{{- .Values.indexer.replicaCount }}
{{- end }}
{{- end }}

{{/*
Generate comma-separated list of zoekt shard URLs for sharded deployments
e.g., "http://code-search-indexer-0.code-search-zoekt-headless:6070,http://code-search-indexer-1.code-search-zoekt-headless:6070"
*/}}
{{- define "code-search.zoektShardUrls" -}}
{{- $fullName := include "code-search.fullname" . -}}
{{- $headlessSvc := printf "%s-zoekt-headless" $fullName -}}
{{- $port := .Values.zoekt.service.port -}}
{{- $replicas := int .Values.sharding.replicas -}}
{{- $urls := list -}}
{{- range $i := until $replicas -}}
{{- $urls = append $urls (printf "http://%s-indexer-%d.%s:%d" $fullName $i $headlessSvc $port) -}}
{{- end -}}
{{- join "," $urls -}}
{{- end -}}
