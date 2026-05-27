# LAN Access Design

## Goal

Make the ATOL Go Server web UI reachable from a Mac or phone on the same local network while keeping it private to the LAN.

## Current State

The server already listens on `:8080` by default through `HTTP_ADDR`, and `docker-compose.yml` already publishes `8080:8080`. No application routing change is required for basic LAN access.

## Approach

Use the Windows machine as the LAN host:

- Run `atol-server` through Docker Compose on Windows.
- Keep the container published on TCP port `8080`.
- Open inbound TCP `8080` in Windows Firewall for the `Private` network profile only.
- Use the Windows machine's LAN IPv4 address from Mac and phone browsers: `http://<windows-lan-ip>:8080/`.
- Prefer a DHCP reservation in the router so the Windows LAN IP stays stable.

## Security Boundary

This design is LAN-only. It does not add public internet access, tunneling, router port forwarding, or a reverse proxy. The firewall rule should be bound to the `Private` profile so the UI is not exposed when Windows is on an untrusted network.

## Google OAuth Note

The web UI works from LAN by IP. If Google authorization is used from a Mac or phone, Google Cloud must also allow the LAN callback URI:

```text
http://<windows-lan-ip>:8080/oauth/google/callback
```

The existing `http://localhost:8080/oauth/google/callback` callback remains valid when authorizing directly on the Windows machine.

## Verification

On Windows:

```powershell
docker compose ps atol-server
Invoke-WebRequest http://localhost:8080/ -UseBasicParsing
```

From Mac or phone on the same Wi-Fi/LAN:

```text
http://<windows-lan-ip>:8080/
```

If LAN access fails while localhost works, the likely causes are Windows Firewall, the Windows network profile being `Public`, the wrong Windows IPv4 address, or client isolation enabled on the Wi-Fi router.
