#!/usr/bin/env bash
# Syncs generated Kubebuilder manifests (CRDs and the manager ClusterRole)
# into the Helm chart under dist/chart/, wrapping them with the conditionals
# expected by the helm/v2-alpha plugin (.Values.crd.enable, .Values.crd.keep,
# .Values.rbac.enable). Run this after `make manifests` so the packaged chart
# stays in sync with config/crd/bases/ and config/rbac/role.yaml.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CRD_SRC_DIR="${ROOT_DIR}/config/crd/bases"
CRD_DST_DIR="${ROOT_DIR}/dist/chart/templates/crd"
ROLE_SRC="${ROOT_DIR}/config/rbac/role.yaml"
ROLE_DST="${ROOT_DIR}/dist/chart/templates/rbac/role.yaml"

mkdir -p "${CRD_DST_DIR}"

# Remove stale CRD templates so renamed/removed CRDs do not linger in the chart.
find "${CRD_DST_DIR}" -mindepth 1 -maxdepth 1 -name '*.yaml' -delete

for src in "${CRD_SRC_DIR}"/*.yaml; do
  [ -e "${src}" ] || continue
  base="$(basename "${src}")"
  dst="${CRD_DST_DIR}/${base}"
  awk '
    BEGIN { print "{{- if .Values.crd.enable }}" }
    /^    controller-gen\.kubebuilder\.io\/version:/ {
      print
      print "    {{- if .Values.crd.keep }}"
      print "    \"helm.sh/resource-policy\": keep"
      print "    {{- end }}"
      print "  labels:"
      print "    {{- include \"chart.labels\" . | nindent 4 }}"
      next
    }
    { print }
    END { print "{{- end }}" }
  ' "${src}" > "${dst}"
done

# Manager ClusterRole — wrap rules from config/rbac/role.yaml with the chart
# header (labels + rbac.enable conditional). The role name must match the one
# referenced by the chart's role_binding.yaml.
{
  echo '{{- if .Values.rbac.enable }}'
  echo 'apiVersion: rbac.authorization.k8s.io/v1'
  echo 'kind: ClusterRole'
  echo 'metadata:'
  echo '  labels:'
  echo '    {{- include "chart.labels" . | nindent 4 }}'
  echo '  name: genkit-operator-manager-role'
  # Copy everything from the source role starting at the `rules:` line.
  awk '/^rules:/{flag=1} flag' "${ROLE_SRC}"
  echo '{{- end -}}'
} > "${ROLE_DST}"

echo "Helm chart synced:"
echo "  CRDs   -> ${CRD_DST_DIR}"
echo "  Role   -> ${ROLE_DST}"
