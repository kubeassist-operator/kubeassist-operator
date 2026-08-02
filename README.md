# KubeAssist Operator

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22-blue)](go.mod)
[![OperatorHub](https://img.shields.io/badge/OperatorHub-alpha-orange)](https://operatorhub.io)

Deploy and manage autonomous AI agents on **any Kubernetes cluster**.

Works with any OpenAI-compatible LLM provider — OpenAI, IBM watsonx, Anthropic Claude, Mistral, Llama via Ollama, and more. Each `KubeAssist` custom resource provisions a fully isolated, production-ready agent pod with scoped RBAC, health probes, persistent workspace, and an HTTP endpoint.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  Kubernetes Cluster                                             │
│                                                                 │
│  ┌──────────────────────────┐                                   │
│  │  kubeassist-operator     │  watches KubeAssist CRs           │
│  │  (controller-manager)    │─────────────────────────┐         │
│  └──────────────────────────┘                         │         │
│                                                       ▼         │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  KubeAssist CR  (namespace: my-namespace)               │    │
│  │                                                         │    │
│  │  spec:                                                  │    │
│  │    apiKeySecretRef:                                      │    │
│  │      name: my-llm-key   ──► Secret (API_KEY)            │    │
│  │    image: my-agent:v1                                   │    │
│  │    ingress:                                             │    │
│  │      enabled: true                                      │    │
│  │      host: agent.example.com                            │    │
│  │                                                         │    │
│  │  Owned resources:                                       │    │
│  │    Deployment ──► Pod (agent container)                 │    │
│  │    Service    ──► ClusterIP :8080                       │    │
│  │    Ingress    ──► agent.example.com  (vanilla k8s)      │    │
│  │    Route      ──► agent-ns.apps.cluster (OpenShift)     │    │
│  │    ConfigMap  ──► agent_config.yaml                     │    │
│  │    ServiceAccount + ClusterRoleBinding (view)           │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

---

## Features

- **Bring your own LLM** — any OpenAI-compatible API endpoint
- **Configurable secret key name** — works with `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `API_KEY`, or any custom key
- **Standard Kubernetes Ingress** — works on any cluster (nginx, traefik, ALB, etc.)
- **OpenShift Route** — automatically created on OCP with edge TLS termination
- **Scoped RBAC** — agent only gets edit access to namespaces you explicitly list
- **Health probes** — readiness and liveness probes included out of the box
- **Ephemeral workspace** — configurable `/workspace` volume per pod
- **Full Lifecycle** — create, update, and delete are all reconciled cleanly

---

## Quick Start

### Prerequisites

- Kubernetes 1.25+ or OpenShift 4.10+
- `kubectl` / `oc` CLI
- An API key for any OpenAI-compatible LLM provider

### 1. Install the CRD and operator

```bash
# Install CRD
kubectl apply -f https://raw.githubusercontent.com/kubeassist-operator/kubeassist-operator/feature/community-release/operator/config/crd/bases/kubeassist.github.io_kubeassists.yaml

# Install operator
kubectl apply -f https://raw.githubusercontent.com/kubeassist-operator/kubeassist-operator/feature/community-release/operator/config/manager/manager.yaml
```

### 2. Create a namespace and API key Secret

```bash
kubectl create namespace kubeassist

kubectl create secret generic my-llm-key \
  --from-literal=API_KEY=sk-...  \
  -n kubeassist
```

> Change `API_KEY` to match your provider:
> - OpenAI: `OPENAI_API_KEY`
> - Anthropic: `ANTHROPIC_API_KEY`
> - IBM watsonx: `WATSONX_APIKEY`

### 3. Apply a KubeAssist CR

**Vanilla Kubernetes (with Ingress):**

```yaml
apiVersion: kubeassist.github.io/v1alpha1
kind: KubeAssist
metadata:
  name: my-agent
  namespace: kubeassist
spec:
  apiKeySecretRef:
    name: my-llm-key
    key: API_KEY
  image: ghcr.io/kubeassist-operator/agent:latest
  replicas: 1
  ingress:
    enabled: true
    host: my-agent.example.com
    ingressClassName: nginx
    tlsSecretName: my-agent-tls   # optional
```

**OpenShift (Route auto-created):**

```yaml
apiVersion: kubeassist.github.io/v1alpha1
kind: KubeAssist
metadata:
  name: my-agent
  namespace: kubeassist
spec:
  apiKeySecretRef:
    name: my-llm-key
    key: API_KEY
  image: ghcr.io/kubeassist-operator/agent:latest
  replicas: 1
  routeTimeout: 600s
```

### 4. Check status

```bash
kubectl get kubeassist my-agent -n kubeassist
# NAME       PHASE     URL                                       AGE
# my-agent   Running   https://my-agent.example.com              2m

kubectl get kubeassist my-agent -n kubeassist -o jsonpath='{.status.url}'
```

---

## CRD Reference

### `spec` fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `apiKeySecretRef.name` | string | — | **Required.** Name of the Secret containing the LLM API key |
| `apiKeySecretRef.key` | string | `API_KEY` | Key inside the Secret to inject as `AGENT_API_KEY` env var |
| `image` | string | `ghcr.io/kubeassist-operator/agent:latest` | Agent container image |
| `imagePullSecretName` | string | — | Pull secret name for private registries |
| `mode` | string | `default` | Agent operating mode (passed as `AGENT_MODE`) |
| `replicas` | int | `1` | Number of agent pods |
| `workspaceSize` | string | `5Gi` | Ephemeral workspace volume size |
| `resources` | ResourceRequirements | — | CPU/memory requests and limits |
| `targetNamespaces` | []string | — | Namespaces where agent gets `edit` RBAC |
| `ingress.enabled` | bool | `false` | Create a standard Kubernetes Ingress |
| `ingress.host` | string | — | Hostname for the Ingress rule |
| `ingress.ingressClassName` | string | — | IngressClass (e.g. `nginx`, `traefik`) |
| `ingress.annotations` | map | — | Extra Ingress annotations |
| `ingress.tlsSecretName` | string | — | TLS Secret for HTTPS |
| `routeTimeout` | string | `600s` | OpenShift Route haproxy timeout |

### `status` fields

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | `Pending`, `Running`, or `Failed` |
| `url` | string | Public HTTPS endpoint |
| `conditions` | []Condition | Standard Kubernetes conditions |
| `observedGeneration` | int | Last reconciled generation |

---

## Building from Source

```bash
git clone https://github.com/kubeassist-operator/kubeassist-operator.git
cd kubeassist-operator/operator

# Download dependencies
go mod download

# Build the manager binary
go build -o bin/manager ./cmd/

# Run locally against a cluster (kubeconfig required)
go run ./cmd/ --leader-elect=false
```

---

## Contributing

We welcome contributions! Please read [CONTRIBUTING.md](CONTRIBUTING.md) before submitting a PR.

---

## License

Apache License 2.0 — see [LICENSE](LICENSE).
