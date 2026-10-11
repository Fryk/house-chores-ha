# Obowiązki — household chores for Home Assistant

🇬🇧 English · [🇵🇱 Polski](README.pl.md)

A Home Assistant app (formerly add-on) for sharing household chores. Reminders arrive as actionable notifications from the Home Assistant Companion app on Android, and the UI opens inside Home Assistant through ingress, with no extra app or login.

> **Status:** early development (milestone M0, skeleton). The app installs, sends a test notification with action buttons and receives the taps. Chores, schedules and statistics come in the next milestones.

## Features (planned)

- Chores with a due date, weekly chores, ad-hoc chores ("needs doing" / "done") and chores triggered by appliances (washer, dryer, dishwasher) based on entity state or power draw.
- Reminders only inside a notification window (10:00–20:00 by default), repeated until someone marks the chore done.
- Calendar and a comparison of who did what, with per-chore weights.
- Export of a month for any LLM and import of suggested weights.
- Polish and English UI.

## Installation

1. In Home Assistant go to **Settings → Apps → App store**, open the repositories dialog and add:
   ```
   https://github.com/Fryk/house-chores-ha
   ```
2. Install **Obowiązki** and start it. The first start builds the image on your device, which takes a few minutes.
3. Open the **Obowiązki** panel in the sidebar.

Details: [obowiazki/DOCS.md](obowiazki/DOCS.md).

## Development

The app is a single Go binary with SQLite (`modernc.org/sqlite`, no CGO) and server-rendered HTML. Without the `SUPERVISOR_TOKEN` environment variable it runs in **fake mode**: notifications go to the log and to the `/dev` page, where you can tap their buttons.

```sh
cd obowiazki
go test ./...
DATA_DIR=./data PORT=8080 go run ./cmd/obowiazki
# open http://localhost:8080 and http://localhost:8080/dev
```

Requires Go 1.26 or newer. The design specification is in [SPEC.md](SPEC.md) (in Polish).

## Repository layout

```text
repository.yaml   Home Assistant app repository metadata
obowiazki/        the app (folder name = slug)
  config.yaml     app configuration
  Dockerfile
  cmd/, internal/ Go code
  web/            embedded templates and static files
SPEC.md           specification (Polish)
```
