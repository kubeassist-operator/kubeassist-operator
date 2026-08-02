# Changelog

All notable changes to the KubeAssist Operator are documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)
Versioning: [Semantic Versioning](https://semver.org/spec/v2.0.0.html)

---

## [0.1.0] — 2025-01-01

### Added
- Initial release of `kubeassist-operator`
- `KubeAssist` CRD (`kubeassist.github.io/v1alpha1`) for deploying autonomous AI agents on Kubernetes
- **Bring-your-own LLM** — `spec.apiKeySecretRef.key` defaults to `API_KEY`, compatible with any OpenAI-compatible provider (OpenAI, IBM watsonx, Anthropic, Mistral, Ollama, etc.)
- **Standard Kubernetes Ingress** support via `spec.ingress` — works on any cluster with any ingress controller
- **OpenShift Route** support — automatically created on OCP clusters, gracefully skipped on vanilla Kubernetes
- Scoped RBAC via `spec.targetNamespaces` — agent ServiceAccount gets `edit` access only to explicitly listed namespaces
- Ephemeral `/workspace` volume per agent pod (`spec.workspaceSize`, default: `5Gi`)
- Full lifecycle management: create, update, scale, delete with finalizer-based cleanup of ClusterRoleBindings
- Standard Kubernetes conditions (`Available`, `Progressing`, `Degraded`) on `status.conditions`
- `status.phase` (Pending / Running / Failed) and `status.url` populated automatically
- Health and readiness probes configured on the agent container
- `runAsNonRoot`, `seccompProfile: RuntimeDefault`, `allowPrivilegeEscalation: false` on the controller pod
- OLM bundle for OperatorHub submission (`alpha` channel)
- Apache 2.0 license
