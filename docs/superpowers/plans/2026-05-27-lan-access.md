# LAN Access Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Document the exact Windows LAN setup needed to open the ATOL Go Server UI from a Mac or phone.

**Architecture:** The server remains unchanged because it already binds to `:8080` and Docker Compose already publishes `8080:8080`. The implementation adds operator-facing documentation for Windows Firewall, network profile, LAN IP discovery, and client verification.

**Tech Stack:** Go HTTP server, Docker Compose, Windows PowerShell, Windows Firewall.

---

### Task 1: Capture The LAN Design

**Files:**
- Create: `docs/superpowers/specs/2026-05-27-lan-access-design.md`

- [x] **Step 1: Write the design spec**

Document that LAN access uses the existing `HTTP_ADDR=:8080` and `8080:8080` Docker port mapping, with Windows Firewall allowing inbound TCP `8080` on the `Private` profile only.

- [x] **Step 2: Verify the spec has no placeholders**

Run:

```bash
rg -n 'T''BD|TO''DO|fill[[:space:]]in|lat''er' docs/superpowers/specs/2026-05-27-lan-access-design.md
```

Expected: no matches.

### Task 2: Add Windows LAN Checklist

**Files:**
- Modify: `WINDOWS_DOCKER_CHECKLIST.md`

- [x] **Step 1: Add a LAN section after local server verification**

Add a section that shows:

```powershell
Get-NetConnectionProfile
Get-NetIPAddress -AddressFamily IPv4 | Where-Object {
    $_.IPAddress -notlike "169.254.*" -and
    $_.IPAddress -ne "127.0.0.1"
} | Select-Object InterfaceAlias,IPAddress
```

- [x] **Step 2: Add the firewall command**

Add an idempotent firewall rule:

```powershell
if (-not (Get-NetFirewallRule -DisplayName "ATOL Go Server 8080 LAN" -ErrorAction SilentlyContinue)) {
    New-NetFirewallRule `
        -DisplayName "ATOL Go Server 8080 LAN" `
        -Direction Inbound `
        -Action Allow `
        -Protocol TCP `
        -LocalPort 8080 `
        -Profile Private
}
```

- [x] **Step 3: Add client verification**

Document that Mac and phone browsers should open:

```text
http://<Windows IPv4>:8080/
```

Also include likely failure causes: Windows network profile is `Public`, firewall rule is missing, wrong IP address, Windows and client are on different networks, or Wi-Fi client isolation is enabled.

- [x] **Step 4: Add Google OAuth callback note**

Document that Google authorization from another device requires adding:

```text
http://<Windows IPv4>:8080/oauth/google/callback
```

to the Google Cloud OAuth client.

### Task 3: Verify Documentation

**Files:**
- Read: `WINDOWS_DOCKER_CHECKLIST.md`
- Read: `docs/superpowers/specs/2026-05-27-lan-access-design.md`
- Read: `docs/superpowers/plans/2026-05-27-lan-access.md`

- [x] **Step 1: Check changed files**

Run:

```bash
git diff -- WINDOWS_DOCKER_CHECKLIST.md docs/superpowers/specs/2026-05-27-lan-access-design.md docs/superpowers/plans/2026-05-27-lan-access.md
```

Expected: the diff contains only LAN access documentation and no application code changes.

- [x] **Step 2: Search for placeholders**

Run:

```bash
rg -n 'T''BD|TO''DO|fill[[:space:]]in|lat''er' WINDOWS_DOCKER_CHECKLIST.md docs/superpowers/specs/2026-05-27-lan-access-design.md docs/superpowers/plans/2026-05-27-lan-access.md
```

Expected: no matches.
