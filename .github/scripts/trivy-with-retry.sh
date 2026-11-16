#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/oci-retry.sh"
: "${OCI_TRIVY_BINARY:?Expected the original Trivy binary path}"
oci_retry "$OCI_TRIVY_BINARY" "$@"
