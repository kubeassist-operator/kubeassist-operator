#!/usr/bin/env bash
# =============================================================================
# install.sh — Install KubeAssist Operator
#
# Works on both vanilla Kubernetes and OpenShift.
#
# Usage:
#   bash install.sh [--namespace kubeassist-operator-system]
#
# Prerequisites:
#   - kubectl or oc on PATH, logged in with cluster-admin
# =============================================================================
set -euo pipefail

OPERATOR_NS="${OPERATOR_NS:-kubeassist-operator-system}"
BRANCH="${BRANCH:-feature/community-release}"
BASE="https://raw.githubusercontent.com/kubeassist-operator/kubeassist-operator/${BRANCH}/operator"

# Colours
GREEN='\033[0;32m'; CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'
log()  { echo -e "${CYAN}[INFO]${RESET}  $*"; }
ok()   { echo -e "${GREEN}[OK]${RESET}    $*"; }
step() { echo -e "\n${BOLD}━━━  $*  ━━━${RESET}"; }

# Use oc if available, else kubectl
CMD="kubectl"
command -v oc &>/dev/null && CMD="oc"
log "Using: $CMD"

# ── Step 1: CRD ───────────────────────────────────────────────────────────────
step "Step 1 — Install CRD"
$CMD apply -f "${BASE}/config/crd/bases/kubeassist.github.io_kubeassists.yaml"
ok "CRD installed"

# ── Step 2: Namespace ─────────────────────────────────────────────────────────
step "Step 2 — Operator namespace"
$CMD create namespace "${OPERATOR_NS}" --dry-run=client -o yaml | $CMD apply -f -
ok "Namespace '${OPERATOR_NS}' ready"

# ── Step 3: RBAC ──────────────────────────────────────────────────────────────
step "Step 3 — RBAC"
$CMD apply -f "${BASE}/config/rbac/service_account.yaml"
$CMD apply -f "${BASE}/config/rbac/role.yaml"
$CMD apply -f "${BASE}/config/rbac/role_binding.yaml"
$CMD apply -f "${BASE}/config/rbac/leader_election_role.yaml"
ok "RBAC applied"

# ── Step 4: Operator deployment ───────────────────────────────────────────────
step "Step 4 — Operator deployment"
$CMD apply -f "${BASE}/config/manager/manager.yaml"
log "Waiting for operator pod to be ready..."
$CMD rollout status deployment/kubeassist-operator-controller-manager \
  -n "${OPERATOR_NS}" --timeout=120s
ok "Operator is running"

echo ""
echo -e "${GREEN}✔  KubeAssist Operator installed successfully${RESET}"
echo ""
echo "Next steps:"
echo "  1. Create a namespace for your agent:"
echo "       $CMD create namespace kubeassist"
echo ""
echo "  2. Create the API key secret:"
echo "       $CMD create secret generic my-llm-key \\"
echo "         --from-literal=API_KEY=sk-... \\"
echo "         -n kubeassist"
echo ""
echo "  3. Apply a KubeAssist CR:"
echo "       $CMD apply -f ${BASE}/config/samples/kubeassist_v1alpha1_kubeassist.yaml"
echo ""
echo "  4. Watch it come up:"
echo "       $CMD get kubeassist -n kubeassist -w"
