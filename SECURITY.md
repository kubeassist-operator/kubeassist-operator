# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.1.x   | ✅ Yes    |

## Reporting a Vulnerability

**Do not open a public GitHub Issue for security vulnerabilities.**

Please report security vulnerabilities by emailing:
**rudraprakash.r@gmail.com**

Include:
- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Potential impact

You will receive a response within 5 business days.
We will work with you to understand and address the issue before any public disclosure.

## Security Design Notes

- The operator runs as a non-root user (`runAsNonRoot: true`)
- The controller pod uses `seccompProfile: RuntimeDefault`
- `allowPrivilegeEscalation: false` is set on the controller container
- The agent's LLM API key is injected via a Kubernetes Secret reference —
  it is never stored in the CR spec or logged
- RBAC is scoped: the agent ServiceAccount only gets `edit` access to
  namespaces explicitly listed in `spec.targetNamespaces`
