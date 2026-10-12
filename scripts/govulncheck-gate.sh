#!/usr/bin/env bash
# Runs govulncheck and fails on any called vulnerability not listed in ACCEPTED_VULNS.
# govulncheck has no native suppression and exits 0 in JSON mode, so this script is the gate.
set -euo pipefail

: "${GOVULNCHECK:?path to govulncheck binary required}"
: "${GO_VERSION:?expected Go version (e.g. 1.25.14) required}"

# Go 1.25 is out of support; these stdlib vulns are fixed only in go1.26.9.
# Delete this list once the stack can move to Go >= 1.26.9.
ACCEPTED_VULNS='[
  "GO-2026-6603",
  "GO-2026-6605",
  "GO-2026-6607",
  "GO-2026-6608",
  "GO-2026-6610",
  "GO-2026-6611",
  "GO-2026-6612",
  "GO-2026-6613",
  "GO-2026-6617"
]'

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
