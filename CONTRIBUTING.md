# Contributing to KubeAssist Operator

Thank you for your interest in contributing! This document explains how to get started.

---

## Code of Conduct

This project follows the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/main/code-of-conduct.md).
Be respectful and constructive in all interactions.

---

## Ways to Contribute

- **Bug reports** — open a GitHub Issue with steps to reproduce
- **Feature requests** — open a GitHub Issue describing the use case
- **Code** — submit a Pull Request (see workflow below)
- **Documentation** — improvements to README, docs/, or code comments
- **Tests** — add or improve controller tests

---

## Development Setup

### Prerequisites

- Go 1.22+
- `kubectl` connected to a test cluster (or `kind` / `minikube` locally)
- `controller-gen` for regenerating CRD manifests (optional)

```bash
# Install controller-gen
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

# Install operator-sdk (for bundle validation)
# https://sdk.operatorframework.io/docs/installation/
```

### Running locally

```bash
git clone https://github.com/kubeassist-operator/kubeassist-operator.git
cd kubeassist-operator/operator

# Install CRD into your current cluster
kubectl apply -f config/crd/bases/

# Run the controller locally (uses your kubeconfig)
go run ./cmd/ --leader-elect=false
```

### Regenerating manifests after API changes

```bash
# Regenerate deepcopy functions
controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Regenerate CRD YAML
controller-gen crd paths="./..." output:crd:artifacts:config=config/crd/bases
```

---

## Pull Request Workflow

1. **Fork** the repository
2. **Create a branch** — `git checkout -b fix/my-fix` or `feat/my-feature`
3. **Make your changes** — keep commits small and focused
4. **Add tests** if you changed controller logic
5. **Ensure it builds** — `go build ./...` and `go vet ./...`
6. **Open a PR** against the `main` branch
7. **Respond to review feedback**

### Commit message format

```
<type>: <short summary>

<optional body>
```

Types: `fix`, `feat`, `docs`, `chore`, `test`, `refactor`

Examples:
- `fix: correctly populate status.url for Ingress-based agents`
- `feat: add spec.serviceType field for NodePort support`
- `docs: add watsonx example to README`

---

## Testing

```bash
cd operator

# Run unit tests
go test ./...

# Run with verbose output
go test -v ./internal/controller/...
```

---

## Questions?

Open a GitHub Issue or start a Discussion in the repository.
