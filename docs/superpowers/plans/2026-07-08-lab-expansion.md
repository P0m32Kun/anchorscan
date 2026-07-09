# AnchorScan Lab Expansion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand the local lab to cover MariaDB, SSH, SMB, and an unknown TCP listener while keeping one simple compose-based startup flow for manual AnchorScan testing.

**Architecture:** Extend the existing root-level `docker-compose.lab.yml` instead of creating a second stack or a lab control plane. Reuse Docker Hub images for standard services, use one tiny Python listener for the unknown TCP case, and document the real-container-IP workflow in the existing README and lab docs.

**Tech Stack:** Docker Compose, Docker Hub service images, Python one-liner listener inside `python:3.12-alpine`, Markdown docs, existing repo lab docs.

## Global Constraints

- Keep a single compose file at `/Users/kun/DEV/new-Anchor/docker-compose.lab.yml`.
- Keep existing `tomcat` and `redis` lab services unchanged unless a compose-level change is required for consistency.
- Add only these new lab services: `mariadb`, `ssh`, `samba`, and `unknown-tcp`.
- Prefer real container IP testing over `127.0.0.1` host-port testing in all new docs.
- Document the macOS requirement for `docker-mac-net-connect` or equivalent before telling the operator to scan container IPs.
- Do not add a Web UI, helper daemon, bootstrap script, or separate compose stack.
- Use the fewest files and dependencies possible; add no new project dependency.
- Keep docs action-oriented: show exact commands the operator should run next.

---

## File Structure

- Modify `/Users/kun/DEV/new-Anchor/docker-compose.lab.yml`: add four services, stable container names, and fallback host-port mappings that avoid common local conflicts.
- Modify `/Users/kun/DEV/new-Anchor/README.md`: add a short "Lab Startup" section with startup, IP discovery, and scan commands.
- Modify `/Users/kun/DEV/new-Anchor/docs/testing-lab-checklist.md`: align recommended targets and sample scan commands with the expanded lab.
- Modify `/Users/kun/DEV/new-Anchor/docs/troubleshooting-lab.md`: add real-IP access troubleshooting and unknown-service expectations.

### Task 1: Expand the Docker lab stack

**Files:**
- Modify: `/Users/kun/DEV/new-Anchor/docker-compose.lab.yml`

**Interfaces:**
- Consumes: existing `tomcat` and `redis` service definitions in `/Users/kun/DEV/new-Anchor/docker-compose.lab.yml`
- Produces: six runnable lab containers with stable names:
  - `anchorscan-lab-tomcat`
  - `anchorscan-lab-redis`
  - `anchorscan-lab-mariadb`
  - `anchorscan-lab-ssh`
  - `anchorscan-lab-samba`
  - `anchorscan-lab-unknown`

- [ ] **Step 1: Record the current compose baseline**

Run:

```bash
docker compose -f docker-compose.lab.yml config
```

Expected: PASS and only the existing `tomcat` and `redis` services appear in rendered output.

- [ ] **Step 2: Rewrite `docker-compose.lab.yml` with the expanded service set**

Update `/Users/kun/DEV/new-Anchor/docker-compose.lab.yml` to:

```yaml
services:
  tomcat:
    image: anchor-tomcat:latest
    container_name: anchorscan-lab-tomcat
    pull_policy: never
    ports:
      - "8080:8080"

  redis:
    image: redis:7-alpine
    container_name: anchorscan-lab-redis
    ports:
      - "6379:6379"

  mariadb:
    image: mariadb:11
    container_name: anchorscan-lab-mariadb
    environment:
      MARIADB_ROOT_PASSWORD: anchorscan
      MARIADB_DATABASE: anchorscan
    ports:
      - "13306:3306"

  ssh:
    image: atmoz/sftp:alpine
    container_name: anchorscan-lab-ssh
    command: lab:anchorscan:1001
    ports:
      - "10022:22"

  samba:
    image: dperson/samba
    container_name: anchorscan-lab-samba
    command:
      - "-u"
      - "lab;anchorscan"
      - "-s"
      - "public;/share;yes;no;no;lab"
    ports:
      - "1445:445"

  unknown-tcp:
    image: python:3.12-alpine
    container_name: anchorscan-lab-unknown
    command:
      - "python"
      - "-c"
      - |
        import socket
        s = socket.socket()
        s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        s.bind(("0.0.0.0", 9099))
        s.listen()
        while True:
            conn, _ = s.accept()
            conn.sendall(b"anchorscan-unknown\r\n")
            conn.close()
    ports:
      - "19099:9099"
```

- [ ] **Step 3: Verify the compose file parses after the edit**

Run:

```bash
docker compose -f docker-compose.lab.yml config
```

Expected: PASS and rendered output includes `mariadb`, `ssh`, `samba`, and `unknown-tcp`.

- [ ] **Step 4: Start the expanded lab once**

Run:

```bash
docker compose -f docker-compose.lab.yml up -d
```

Expected: PASS and `docker ps` shows the six `anchorscan-lab-*` containers.

- [ ] **Step 5: Verify real container IP lookup works with the chosen names**

Run:

```bash
docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' anchorscan-lab-mariadb
docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' anchorscan-lab-ssh
docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' anchorscan-lab-samba
docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' anchorscan-lab-unknown
```

Expected: each command prints a non-empty IPv4 address.

- [ ] **Step 6: Stop the lab cleanly**

Run:

```bash
docker compose -f docker-compose.lab.yml down
```

Expected: PASS and the lab containers are removed.

- [ ] **Step 7: Commit Task 1**

```bash
git add docker-compose.lab.yml
git commit -m "chore: expand local test lab services"
```

### Task 2: Add the short startup workflow to the README

**Files:**
- Modify: `/Users/kun/DEV/new-Anchor/README.md`

**Interfaces:**
- Consumes: container names and ports from Task 1
- Produces: a user-facing "Lab Startup" section with exact startup and scan commands

- [ ] **Step 1: Add a concise "Lab Startup" section to `README.md`**

Insert this section after the "Lab docs" section in `/Users/kun/DEV/new-Anchor/README.md`:

```md
## Lab Startup

Start the bundled local lab:

```bash
docker compose -f docker-compose.lab.yml up -d
docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
```

Get real container IPs:

```bash
docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' anchorscan-lab-tomcat
docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' anchorscan-lab-redis
docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' anchorscan-lab-mariadb
docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' anchorscan-lab-ssh
docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' anchorscan-lab-samba
docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' anchorscan-lab-unknown
```

On macOS, direct access to container bridge IPs usually requires `docker-mac-net-connect` or an equivalent route helper. If that is not available, use the published fallback host ports instead.

Example mixed scan using real container IPs:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target <TOMCAT_IP>,<REDIS_IP>,<MARIADB_IP>,<SSH_IP>,<SAMBA_IP>,<UNKNOWN_IP> \
  --ports 22,445,3306,6379,8080,9099 \
  --db data/scans.sqlite \
  --json reports/lab-mixed.json \
  --html reports/lab-mixed.html
```

Stop the lab:

```bash
docker compose -f docker-compose.lab.yml down
```
```

- [ ] **Step 2: Verify the README section is readable and uses the right names**

Run:

```bash
sed -n '/## Lab Startup/,$p' README.md
```

Expected: the section shows the six `anchorscan-lab-*` names, the macOS note, and the mixed scan example.

- [ ] **Step 3: Commit Task 2**

```bash
git add README.md
git commit -m "docs: add lab startup instructions"
```

### Task 3: Align manual test and troubleshooting docs with the new lab

**Files:**
- Modify: `/Users/kun/DEV/new-Anchor/docs/testing-lab-checklist.md`
- Modify: `/Users/kun/DEV/new-Anchor/docs/troubleshooting-lab.md`

**Interfaces:**
- Consumes: Task 1 service names and ports, Task 2 real-IP workflow
- Produces: manual validation and troubleshooting docs that match the expanded lab

- [ ] **Step 1: Update the bundled lab example in `docs/testing-lab-checklist.md`**

Replace the small bundled-lab example command in `/Users/kun/DEV/new-Anchor/docs/testing-lab-checklist.md` with:

```md
For the bundled Docker lab, prefer real container IPs and the exact service ports under test:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target <TOMCAT_IP>,<REDIS_IP>,<MARIADB_IP>,<SSH_IP>,<SAMBA_IP>,<UNKNOWN_IP> \
  --ports 22,445,3306,6379,8080,9099 \
  --db data/scans.sqlite \
  --json reports/lab.json \
  --html reports/lab.html
```
```

- [ ] **Step 2: Add explicit per-service manual commands to `docs/testing-lab-checklist.md`**

Add these examples under the existing service case headings in `/Users/kun/DEV/new-Anchor/docs/testing-lab-checklist.md`:

```md
### 3. MySQL / MariaDB

Command:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target <MARIADB_IP> \
  --ports 3306 \
  --db data/scans.sqlite \
  --json reports/mariadb.json \
  --html reports/mariadb.html
```

### 4. SMB

Command:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target <SAMBA_IP> \
  --ports 445 \
  --db data/scans.sqlite \
  --json reports/samba.json \
  --html reports/samba.html
```

### 5. SSH

Command:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target <SSH_IP> \
  --ports 22 \
  --db data/scans.sqlite \
  --json reports/ssh.json \
  --html reports/ssh.html
```

### 6. Unknown Service

Command:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target <UNKNOWN_IP> \
  --ports 9099 \
  --db data/scans.sqlite \
  --json reports/unknown.json \
  --html reports/unknown.html
```
```

- [ ] **Step 3: Extend `docs/troubleshooting-lab.md` for real-IP access**

Add these sections to `/Users/kun/DEV/new-Anchor/docs/troubleshooting-lab.md`:

```md
## 12. Container is up but its real IP is unreachable

Check:

- the container has a bridge IP from `docker inspect`
- you are scanning the container IP, not the published host port
- on macOS, `docker-mac-net-connect` or an equivalent route helper is running

Quick checks:

```bash
docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' anchorscan-lab-mariadb
docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' anchorscan-lab-samba
```

If these commands return IPs but the host still cannot connect, treat it as a Docker networking issue first, not an AnchorScan bug.

## 13. Unknown TCP service is detected weakly or not labeled

That is acceptable.

Expected behavior:

- the open port is still discovered
- weak or unknown fingerprint data does not crash the run
- the service is not forced into the Web path
- JSON and HTML reports are still generated

If the run crashes or the service is misrouted into `httpx`, that is a real bug.
```

- [ ] **Step 4: Verify the doc set matches the new lab**

Run:

```bash
rg -n 'anchorscan-lab-(tomcat|redis|mariadb|ssh|samba|unknown)|9099|10022|13306|1445|docker-mac-net-connect' README.md docs/testing-lab-checklist.md docs/troubleshooting-lab.md
```

Expected: matches appear in the README startup section and the updated lab docs.

- [ ] **Step 5: Commit Task 3**

```bash
git add docs/testing-lab-checklist.md docs/troubleshooting-lab.md
git commit -m "docs: align lab guides with expanded services"
```

## Self-Review

- Spec coverage:
  - single compose expansion: Task 1
  - real-IP startup flow: Task 2
  - README update: Task 2
  - testing checklist update: Task 3
  - troubleshooting update: Task 3
  - macOS note: Tasks 2 and 3
- Placeholder scan: no `TBD`, `TODO`, or "similar to" shortcuts remain.
- Type and naming consistency: service names, container names, fallback host ports, and file paths are consistent across all tasks.
