{{- define "to_millicores" -}}
  {{- $value := toString . -}}
  {{- if hasSuffix "m" $value -}}
    {{ trimSuffix "m" $value }}
  {{- else -}}
    {{ mulf $value 1000 }}
  {{- end -}}
{{- end -}}


{{- /*
When migrating another legacy Deployment whose API is provided by core-legacy-api,
change metadata.name in templates/deployment.yaml to a new name, for example
core-legacy-api-v2. After the config-server migration, the existing core-legacy-api
Deployment already serves the config-server API. Reusing its name for this pre-hook
would delete the serving Deployment before creating its replacement, making that
API unavailable. A new name lets the pre-hook deploy the replacement while the old
Deployment continues serving traffic. With pruning enabled, Argo removes the old
Deployment during Sync if it is a normal tracked resource, not a retained hook.
Use PruneLast=true to defer deletion until the Service update and other resources
have synced and become healthy. Old hook resources require explicit cleanup.
*/ -}}
{{- define "core-legacy-api.configServerExists" -}}
{{- if lookup "apps/v1" "Deployment" .Release.Namespace "config-server" -}}
true
{{- end -}}
{{- end -}}
