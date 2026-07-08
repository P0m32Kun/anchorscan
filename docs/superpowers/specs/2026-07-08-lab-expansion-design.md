# AnchorScan Lab Expansion Design

Date: 2026-07-08

## Goal

Expand the local testing lab so manual validation covers more non-web and mixed-service cases without turning the lab into a separate product.

The lab should remain easy to start with one compose file, but provide enough service variety to validate:

- MySQL / MariaDB fingerprinting and routing
- SSH fingerprinting
- SMB fingerprinting and NSE routing
- unknown-service stability
- mixed-host report grouping

The operator will scan container real IPs rather than `127.0.0.1` port mappings.

## Scope

Included:

- extend `docker-compose.lab.yml`
- keep existing `Tomcat` and `Redis`
- add `MariaDB`, `OpenSSH`, `Samba`, and one lightweight unknown TCP service
- document startup, shutdown, and real-IP discovery
- document macOS-specific note for `docker-mac-net-connect`
- update manual lab checklist and troubleshooting docs

Not included:

- a Web UI for lab lifecycle management
- exploit frameworks or exploit confirmation labs
- a separate compose stack for "extra" services
- automatic lab bootstrap scripts unless the compose file proves insufficient

## User Experience

The operator should need only one startup command:

```bash
docker compose -f docker-compose.lab.yml up -d
```

The operator should then:

1. confirm containers are healthy with `docker ps`
2. retrieve each container IP with `docker inspect`
3. run AnchorScan against one service IP or several service IPs
4. stop the lab with:

```bash
docker compose -f docker-compose.lab.yml down
```

The docs should provide ready-to-run examples for:

- one service at a time
- a mixed-host style validation using several targets in one run

## Lab Layout

Keep a single compose file at the repo root.

Services:

- `tomcat`
  - existing web target on `8080`
- `redis`
  - existing non-web target on `6379`
- `mariadb`
  - new target on `3306`
  - simple root password for operator testing
- `openssh`
  - new target on `22`
  - fixed test account and password
- `samba`
  - new target on `445`
  - fixed test account preferred over guest mode for more stable behavior
- `unknown-tcp`
  - lightweight listener on `9099`
  - intentionally weak or custom banner so AnchorScan can prove it does not misroute or crash

All services remain reachable through published host ports for convenience, but manual validation guidance will prefer container real IPs.

## Service Choices

### MariaDB

Use an official or common public MariaDB image from Docker Hub.

Why:

- low setup cost
- stable `3306` exposure
- useful for `mysql` / `mariadb` normalization checks

This lab is for fingerprinting and routing validation, not database exploitation.

### OpenSSH

Use a common public OpenSSH server image from Docker Hub.

Why:

- very common baseline service
- stable `ssh` fingerprint output
- easy manual connection test when needed

### Samba

Use a public Samba image from Docker Hub.

Why:

- exposes `445`
- covers `smb` alias normalization
- helps validate NSE routing on non-web services

Prefer a fixed credential over anonymous guest access unless the selected image makes guest mode dramatically simpler and still stable.

### Unknown TCP

Use a tiny image and a simple listener command instead of building a custom application image.

Why:

- keeps the lab small
- avoids introducing a new code-maintained service
- still exercises the "unknown service should not crash or become web" path

If possible, use a listener that accepts TCP connections and emits a short custom string. If the selected tool cannot do that cleanly, a silent listener is acceptable.

## Documentation Changes

### README

Add a short "Lab Startup" section that covers:

- starting the compose stack
- listing running containers
- retrieving container IPs
- scanning by real IP

This section should stay short and action-oriented.

### testing-lab-checklist

Update the recommended targets and commands so the lab checklist matches the expanded compose file.

Add explicit examples for:

- Tomcat
- Redis
- MariaDB
- SSH
- SMB
- unknown TCP
- one mixed run

### troubleshooting-lab

Add or expand notes for:

- container real IP access from macOS
- what to do if a container is up but not reachable by IP
- what to expect from weak/unknown fingerprints

## Networking Notes

Real container IP access is the default validation path for this lab design.

Document these assumptions clearly:

- Linux hosts can usually reach bridge container IPs directly
- macOS hosts commonly need `docker-mac-net-connect` or equivalent
- published host ports remain available as a fallback, but they are not the preferred validation path for this manual lab pass

## Testing

This change is mostly infrastructure and documentation, so keep verification small:

- compose file parses successfully
- if Docker is available, the stack starts
- container IP retrieval steps in the docs match real container names

Do not add broad automated E2E coverage as part of this design step.

## Risks

### Image drift

Public images can change behavior over time.

Mitigation:

- prefer widely used images
- keep docs focused on exposed ports and expected fingerprint families, not fragile version strings

### macOS networking friction

Container real-IP testing is less turnkey on macOS.

Mitigation:

- document the requirement up front
- preserve host-port publishing as fallback

### Overweight lab

Adding too many services would slow startup and distract from the scanner itself.

Mitigation:

- add only four services now
- keep one compose file
- avoid custom-built vulnerable stacks

## Implementation Notes

Follow the existing repo style:

- smallest working compose expansion
- no new control-plane code
- no speculative scripts
- documentation should tell the operator exactly what to run next
