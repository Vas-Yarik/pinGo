<p align="center">
  <img src="images/logo.svg" alt="pinGo logo" width="420">
</p>

<p align="center">
  A fast, lightweight website health checker written in Go.
</p>

---

## About

**pinGo** is a single static binary that pings your websites, measures response time and tells you when something is down. No runtime, no database, no dependencies.

## Features

- Pluggable probe types: HTTP(S) and TCP out of the box, one interface to add more
- Three run modes: one-shot check, live TUI (btop-like), and daemon
- Native desktop notifications on state change (via `notify-send` on Ubuntu)
- Per-target latency history with uptime percentage and trend sparkline
- Concurrent checks with per-target timeouts

## Modes

| Command | What it does |
| --- | --- |
| `pingo` (or `pingo check`) | Pings all targets once, prints a table, exits `0` if all up, `1` if any down, `2` on error |
| `pingo watch` | Live TUI: real-time status, latency, uptime %, latency trend; `q` to quit |
| `pingo daemon` | Runs forever, sends an OS notification when a target goes down or recovers; built to be supervised by systemd |

All modes accept `-config <path>` (default `config.yaml`).

## Configuration

```yaml
interval: 30s

targets:
  - name: example            # any http(s) URL, probed with GET
    url: https://example.com
    timeout: 5s

  - name: postgres           # raw TCP connect probe
    type: tcp
    addr: db.internal:5432
    timeout: 3s
```

- `type` — `http` (default) or `tcp`
- `url` — required for http targets, absolute http(s) URL
- `addr` — required for tcp targets, `host:port`
- `timeout` — per target, defaults to 5s
- `interval` — polling period, defaults to 30s, minimum 1s

A target counts as **up** when the probe succeeds: any HTTP response (even 4xx/5xx) counts as up for http targets; an established TCP connection counts as up for tcp targets.

## Building and running

```sh
go build -o pingo ./cmd/pingo
./pingo -config config.example.yaml
./pingo watch -config config.example.yaml
./pingo daemon -config config.example.yaml
```

## Running as a service (Ubuntu)

Create `~/.config/systemd/user/pingo.service`:

```ini
[Unit]
Description=pinGo health checker

[Service]
ExecStart=%h/go/bin/pingo daemon -config %h/pingo/config.yaml
Restart=on-failure
Environment=DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/%U/bus

[Install]
WantedBy=default.target
```

Then:

```sh
systemctl --user daemon-reload
systemctl --user enable --now pingo
journalctl --user -u pingo -f
```

The `DBUS_SESSION_BUS_ADDRESS` line is what lets a user-session service deliver desktop notifications.
