# Security Policy

## Supported Versions

| Version | Supported          |
|---------|--------------------|
| 1.x     | Yes                |
| < 1.0   | No                 |

## Reporting a Vulnerability

If you discover a security vulnerability in Mozza, please report it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, use GitHub's private vulnerability reporting — open a draft advisory at
**https://github.com/gshepptech/mozza/security/advisories/new** — and include:

1. A description of the vulnerability
2. Steps to reproduce the issue
3. The potential impact
4. Any suggested fixes (optional)

## Response Timeline

- **Acknowledgment**: Within 48 hours
- **Initial assessment**: Within 1 week
- **Fix timeline**: Depends on severity, typically within 2 weeks for critical issues

## Scope

The following are in scope:

- The Mozza binary and its dependencies
- The web dashboard
- The recipe parser and compiler
- Authentication and session management
- Data storage and encryption

The following are out of scope:

- Vulnerabilities in third-party Docker images deployed via Mozza
- Issues requiring physical access to the host machine
- Social engineering attacks

## Recognition

We appreciate security researchers who help keep Mozza safe. With your
permission, we will acknowledge your contribution in the release notes.
