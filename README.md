# DevNAT

**Expose any local service to the internet over a single outbound `:443` tunnel — no port forwarding, no firewall changes, no public IP.**

DevNAT is a small, self-hostable reverse-tunnel built for developers who need to
show work to a client *now*. You run a service locally, point DevNAT at it, and
get a public HTTPS URL backed by your own relay. The agent dials **out** on port
443, so it works behind NAT, CGNAT, dynamic IPs and corporate firewalls.

```
  Client browser            Relay (public, your VPS)        Agent (your machine)
  https://demo.example.com  *.example.com :443              devnat http 8080
        │                          │                              │
        │  GET /                   │   (outbound tunnel already    │
        │─────────────────────────▶│    opened by the agent)       │
        │                          │── stream over the tunnel ────▶│
        │                          │                              │── localhost:8080
        │                          │◀──── response ────────────────│
        │◀─────────────────────────│                              │
```

> Why not pure peer-to-peer (WebRTC / hole punching)? A normal client browser
> can only open `https://` to a name that resolves to a **public** IP. So a thin
> public relay is required. DevNAT keeps that relay minimal — it only multiplexes
> and forwards; your service and its data stay entirely on your machine.

---

## Features

- **One outbound connection on 443** — traverses NAT/CGNAT without opening ports.
- **Automatic HTTPS** — per-subdomain Let's Encrypt certificates issued on demand
  (TLS-ALPN-01), so you only need a wildcard DNS record, not DNS-API credentials.
- **Single static binary** — same binary is both `agent` and `relay`.
- **Local request inspector** — built-in dashboard at `http://127.0.0.1:4040`.
- **Self-host or shared** — run your own relay, or point everyone at one instance.
- **Token auth** — gate who can open tunnels on your relay.

---

## Quickstart

### 1. Run the relay (once, on a public host)

Point DNS at the relay host:

```
*.example.com   A   <relay-ip>
example.com     A   <relay-ip>
```

Then start it:

```bash
devnat relay --domain example.com --email you@example.com --token "$(openssl rand -hex 24)"
```

Or with Docker:

```bash
cp .env.example .env   # fill in DEVNAT_DOMAIN, DEVNAT_EMAIL, DEVNAT_TOKEN
docker compose up -d --build
```

### 2. Expose a local service (on the developer's machine)

```bash
devnat http 8080 --relay wss://example.com --token <same-token>
#  tunnel up   https://a1b2c3d4e5.example.com  ->  http://127.0.0.1:8080
#  dashboard   http://127.0.0.1:4040
```

Pick a friendly subdomain for a client demo:

```bash
devnat http 3000 --subdomain acme-demo --relay wss://example.com --token <token>
#  tunnel up   https://acme-demo.example.com  ->  http://127.0.0.1:3000
```

Send the URL to your client. Stop the tunnel with `Ctrl-C`.

---

## Try it locally (no domain, no TLS)

Most browsers resolve `*.localhost` to `127.0.0.1`, so you can test the whole
flow on one machine:

```bash
# terminal 1 — relay in dev mode
devnat relay --dev --domain localhost --addr :8000

# terminal 2 — something to expose
python3 -m http.server 8080

# terminal 3 — the tunnel
devnat http 8080 --relay ws://localhost:8000

# then open the printed http://<sub>.localhost:8000 URL
```

---

## Configuration

Every flag has an environment-variable equivalent.

### Relay

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--domain` | `DEVNAT_DOMAIN` | — (required) | Public base domain. |
| `--addr` | `DEVNAT_ADDR` | `:443` | Listen address. |
| `--email` | `DEVNAT_EMAIL` | — | ACME contact email for TLS. |
| `--token` | `DEVNAT_TOKEN` | empty | Shared secret; empty = open relay. |
| `--dev` | — | `false` | Plain HTTP, no TLS (local testing). |
| — | `DEVNAT_CERT_DIR` | platform default | Where certificates are stored. |

### Agent

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--relay` | `DEVNAT_RELAY` | `wss://devnat.example.com` | Relay WebSocket URL. |
| `--token` | `DEVNAT_TOKEN` | empty | Auth token for the relay. |
| `--subdomain` | — | random | Requested subdomain. |
| `--dashboard` | — | `127.0.0.1:4040` | Local inspector address (empty to disable). |

---

## Build from source

```bash
make build         # -> ./devnat
make run-relay-dev # build + run a local dev relay
```

Requires Go 1.26+.

---

## Deployment notes

- The relay wants to own `:443`. If the host already runs Nginx for other sites,
  either give the relay its own host/IP, or front it with an Nginx `stream {}`
  block doing SNI passthrough to the relay container.
- With `network_mode: host`, container port mapping is ignored; the relay binds
  `:443` directly on the host (the compose file adds `NET_BIND_SERVICE`).
- Certificates persist in the `certs` volume — don't wipe it between deploys or
  you'll re-issue on every restart and may hit Let's Encrypt rate limits.

## Security

- Always set `--token` on a public relay; otherwise anyone can open tunnels.
- Tunnelled traffic terminates TLS at the relay and travels to your machine over
  the encrypted tunnel. The relay sees decrypted HTTP (it has to route by host),
  so run relays you trust.
- Never commit your `.env`.

## Roadmap

- [ ] Password-protected links (`--password`) for client demos
- [ ] Expiring tunnels (`--expires 2h`)
- [ ] Raw TCP tunnels (databases, SSH), not just HTTP
- [ ] Custom domains (`CNAME` to the relay)
- [ ] Per-token reserved subdomains + a multi-user registry

## License

MIT © Alexandre Alan
