## OperatorHub Submission Checklist

This document tracks progress toward the OperatorHub community operators submission.
Reference: https://github.com/k8s-operatorhub/community-operators/blob/main/docs/contributing-prerequisites.md

---

### Pre-submission checklist

- [x] Apache 2.0 LICENSE file present
- [x] API group uses owned/controlled domain — `kubeassist.github.io`
- [x] No IBM/OCP-specific branding in operator or CRD names
- [x] `spec.apiKeySecretRef.key` is configurable (defaults to `API_KEY`)
- [x] Standard Kubernetes Ingress supported (not just OpenShift Route)
- [x] OpenShift Route gracefully skipped on vanilla k8s (no API → no crash)
- [x] `runAsNonRoot: true` on operator pod
- [x] `seccompProfile: RuntimeDefault` on operator pod
- [x] `allowPrivilegeEscalation: false` on operator containers
- [x] Resource requests and limits set on operator container
- [x] CRD has `+kubebuilder:subresource:status`
- [x] Status conditions use standard `metav1.Condition` type
- [x] Finalizer cleans up cluster-scoped resources (ClusterRoleBindings)
- [x] OLM bundle structure: `bundle/manifests/` + `bundle/metadata/annotations.yaml`
- [x] CSV `installModes` supports all four modes
- [x] CSV has `alm-examples` with a valid CR sample
- [x] CSV `capabilities` set to `Basic Install`
- [x] CSV `maintainers` uses personal (non-IBM) email
- [x] CSV `provider.name` = `KubeAssist Community` (no IBM)
- [x] CSV `repository` points to public GitHub
- [x] Version starts at `v0.1.0` (no phantom `replaces:` chain)
- [x] CONTRIBUTING.md present
- [x] SECURITY.md present
- [x] CHANGELOG.md present
- [x] README.md with architecture diagram and quick start

### Still TODO before PR

- [ ] Build and push controller image to `ghcr.io/kubeassist-operator/controller:v0.1.0`
- [ ] Build and push agent image to `ghcr.io/kubeassist-operator/agent:latest`
- [ ] Run `operator-sdk bundle validate ./operator/bundle`
- [ ] Run operator scorecard against a live cluster
- [ ] Add the CRD YAML to `bundle/manifests/` (copy from `config/crd/bases/`)
- [ ] Register on OperatorHub CI: add `ci.yaml` to community-operators PR
- [ ] Open PR to `k8s-operatorhub/community-operators` under `operators/kubeassist-operator/0.1.0/`
