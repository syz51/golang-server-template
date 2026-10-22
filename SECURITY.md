# Security policy

The latest published template revision is the maintenance target. Generated projects are independent copies and do not receive fixes automatically. Consumers should monitor upstream changes, update their dependencies, and review applicable fixes. Older template releases have no separate backport commitment.

## Reporting a vulnerability

Use [GitHub private vulnerability reporting](https://github.com/syz51/golang-server-template/security/advisories/new) when it is enabled. If the private form is unavailable, open a public issue asking the maintainer for a private reporting channel **without publishing vulnerability details or exploit code**. Response times are best effort; no response-time SLA is promised.

Include affected revisions, the relevant configuration, a minimal reproduction, impact, and any proposed fix. Remove credentials and personal information. Coordinate public disclosure with the maintainer.

## Deployment scope

The user API is an unauthenticated in-memory demonstration. Do not deploy it for real personal data without application-specific authentication, authorization, persistence, and operational controls. `/health` is liveness only and does not prove dependency readiness.

CORS is disabled by default; configure explicit origins only when needed. Terminate TLS through an appropriate deployment setup, protect management/debug access, provide secrets at runtime, and align ingress limits and stop grace periods with the server configuration. Never expose Delve outside the development machine.

The distroless non-root image reduces included components but does not guarantee freedom from vulnerabilities. CI scans reachable Go dependencies; assess base images and deployment infrastructure separately.

When adopting this template, replace this policy's upstream reporting links with your application's reporting channel and support policy.
