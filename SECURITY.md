# Security policy

## Supported versions

The latest stable v1 release receives security fixes. Older releases and the
`main` branch are unsupported; upgrade before reporting unless the issue is a
regression under active development.

| Version | Supported |
| --- | --- |
| Latest stable v1 release | Yes |
| Older releases | No |
| `main` | No |

## Reporting a vulnerability

Do not disclose a suspected vulnerability in a public issue. Use the
[private vulnerability reporting form](https://github.com/faustbrian/go-clock/security/advisories/new).

Do not include secrets, production timestamps, callback payloads, customer
data, or raw production errors in a public report or initial contact request.

The package has no production network, filesystem, cgo, unsafe, or runtime
patching surface. Resource exhaustion, callback isolation, and process-global
clock mutation are part of the threat model in
[docs/security-model.md](docs/security-model.md).
