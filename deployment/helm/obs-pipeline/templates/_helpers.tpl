{{/*
Render a fully-qualified image reference for an obs service.
Image names have the "obs-" prefix: obs-ingestor, obs-enricher, etc.
Usage: {{ include "obs.image" (dict "name" "ingestor" "root" .) }}
Renders: "obs-ingestor:latest" (no registry) or "registry/obs-ingestor:latest"
*/}}
{{- define "obs.image" -}}
{{- $fullName := printf "obs-%s" .name -}}
{{- if .root.Values.image.registry -}}
{{- printf "%s/%s:%s" .root.Values.image.registry $fullName .root.Values.image.tag -}}
{{- else -}}
{{- printf "%s:%s" $fullName .root.Values.image.tag -}}
{{- end -}}
{{- end }}

{{/*
Common labels applied to all resources.
*/}}
{{- define "obs.labels" -}}
app.kubernetes.io/managed-by: Helm
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end }}