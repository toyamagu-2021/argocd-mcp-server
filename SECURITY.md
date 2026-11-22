# Security policy

## Supported versions

Security fixes are applied to the default branch. Release tags follow
the versions published on the GitHub Releases page.

## Reporting a vulnerability

Please report security issues privately. Do not open a public GitHub
issue for a suspected vulnerability.

Use [GitHub private vulnerability reporting](https://github.com/toyamagu-2021/argocd-mcp-server/security/advisories/new)
for this repository. If that form is unavailable, email the repository
owner through the address on their GitHub profile.

Include the affected version or commit, the component, and a short
description of the impact. Please allow time for a fix before any
public disclosure.

## Dependencies

This project uses Go modules and GitHub Actions. Dependency updates are
opened by Dependabot. Reachable Go vulnerabilities are reported by the
govulncheck workflow.
