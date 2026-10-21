#!/usr/bin/env bash
# Runs govulncheck and fails on any called vulnerability not listed in ACCEPTED_VULNS.
# govulncheck has no native suppression and exits 0 in JSON mode, so this script is the gate.
set -euo pipefail

: "${GOVULNCHECK:?path to govulncheck binary required}"
: "${GO_VERSION:?expected Go version (e.g. 1.27.3) required}"

ACCEPTED_VULNS='[]'

report=$("$GOVULNCHECK" -format json ./...)

# A mis-detected toolchain yields an empty stdlib version and zero findings, i.e. a silent pass.
scanned_go=$(jq -r 'select(.SBOM) | .SBOM.go_version' <<<"$report")
if [[ "$scanned_go" != "go${GO_VERSION}" ]]; then
  echo "✗ govulncheck scanned stdlib '${scanned_go}', expected go${GO_VERSION}" >&2
  exit 1
fi

called_vulns() {
  jq -r 'select(.finding) | .finding | select(.trace[0].function != null) | .osv' <<<"$report" | sort -u
}

unaccepted=$(called_vulns | jq -R -r --argjson accepted "$ACCEPTED_VULNS" 'select(. as $id | $accepted | index($id) | not)')
if [[ -n "$unaccepted" ]]; then
  echo "✗ govulncheck found vulnerabilities not in ACCEPTED_VULNS:" >&2
  echo "$unaccepted" >&2
  echo "Run '${GOVULNCHECK} ./...' for call traces." >&2
  exit 1
fi

accepted_hits=$(called_vulns | wc -l | tr -d ' ')
echo "✓ govulncheck (go${GO_VERSION}): no new vulnerabilities; ${accepted_hits} accepted vulnerabilities still present"
