{{- define "to_millicores" -}}
  {{- $value := toString . -}}
  {{- if hasSuffix "m" $value -}}
    {{ trimSuffix "m" $value }}
  {{- else -}}
    {{ mulf $value 1000 }}
  {{- end -}}
{{- end -}}

{{- define "core-legacy-api.consulAuth" -}}
  {{- $mode := .Values.CONSUL_AUTH_MODE | default .Values.M2M_AUTH_MODE | default "legacy" | trim -}}
  {{- $crdInstalled := .Capabilities.APIVersions.Has "netcracker.com/v1/ConsulACL" -}}
  {{- if eq $mode "legacy" -}}
consulAcl: false
consulTokenSecret: true
  {{- else if eq $mode "hybrid" -}}
consulAcl: {{ $crdInstalled }}
consulTokenSecret: true
  {{- else if eq $mode "k8s" -}}
    {{- if not $crdInstalled -}}
      {{- fail "Consul auth mode k8s needs the ConsulACL CRD (netcracker.com/v1) in the cluster: install Consul with its ACL configurator, or set CONSUL_AUTH_MODE to hybrid or legacy" -}}
    {{- end -}}
consulAcl: true
consulTokenSecret: false
  {{- else -}}
    {{- fail (printf "Unknown Consul auth mode '%s' from CONSUL_AUTH_MODE or M2M_AUTH_MODE: expected legacy, hybrid or k8s" $mode) -}}
  {{- end -}}
{{- end -}}
