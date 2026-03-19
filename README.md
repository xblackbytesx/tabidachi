# Tabidachi 旅立ち

**tab·i·da·chi** | /tɑːbɪˈdɑːtʃi/ | *noun* | Japanese

**1.** The act of setting off on a journey; departure for a trip.
   *"the quiet thrill of tabidachi as the platform slips away behind you"*

**2.** (figurative) A fresh start; the moment one commits to going somewhere new.

— from **旅** (*tabi*, journey, travel) + **立ち** (*tachi*, rising up, setting forth)

---

Tabidachi is a self-hosted travel itinerary manager. Build and organise trips with legs, days, and events; add cover photos; export/import JSON; and generate AI-assisted itinerary prompts. A JSON API with personal access tokens lets external clients (e.g. a native mobile app) read your trips.

---

## Features

- **Trip builder** — structured legs → days → events (activities, transit, accommodation)
- **Cover images** — auto-fetched from Pexels / Unsplash / Wikipedia, or set manually
- **AI prompt wizards** — convert existing booking docs into Tabidachi JSON, or plan a new trip from scratch
- **Import / Export** — round-trip via a documented JSON schema
- **JSON API** — read-only `/api/v1/` endpoints secured with personal access tokens (PATs)
- **Single-user or multi-account** — each account only sees its own trips
- **Dark theme** — built with Shoelace web components on an OLED-black palette

---

## Screenshots

<table>
  <tr>
    <td><img src="docs/screenshots/03%20-%20Trips%20-%20Tabidachi.png" width="400" alt="Trip dashboard"></td>
    <td><img src="docs/screenshots/04%20-%20Japan%20Itinerary%20-%20Tabidachi.png" width="400" alt="Trip view with cover image and itinerary"></td>
  </tr>
  <tr>
    <td><img src="docs/screenshots/08%20-%20Edit%20Japan%20Itinerary%20-%20Tabidachi.png" width="400" alt="Itinerary builder with days and events"></td>
    <td><img src="docs/screenshots/01%20-%20New%20Trip%20-%20Tabidachi.png" width="400" alt="New trip creation methods"></td>
  </tr>
  <tr>
    <td><img src="docs/screenshots/11%20-%20Plan%20with%20AI%20-%20Tabidachi.png" width="400" alt="Plan a new trip with AI"></td>
    <td><img src="docs/screenshots/10%20-%20Convert%20Existing%20Itinerary%20-%20Tabidachi.png" width="400" alt="Convert existing itinerary with AI"></td>
  </tr>
</table>

[See all screenshots →](docs/screenshots/)

---

## Tech stack

| Layer | Technology |
|---|---|
| Language | Go |
| Web framework | Echo |
| Templating | templ (SSR) |
| Frontend | HTMX + Shoelace |
| Database | PostgreSQL |
| Auth | gorilla/sessions + gorilla/csrf |
| Migrations | golang-migrate |

---

## Quick start (development)

The development stack uses [air](https://github.com/air-verse/air) for live reload — every `.go` or `.templ` save rebuilds and restarts the server automatically. No local Go toolchain is required; everything runs inside Docker.

**Prerequisites:** Docker (with Compose support).

```bash
git clone https://github.com/xblackbytesx/tabidachi.git
cd tabidachi
make dev
```

The first build pulls base images, installs npm dependencies (Shoelace + HTMX), and starts the app. Once you see `starting server addr=:8080` in the logs, open:

```
http://localhost:8080
```

Register an account and you're in. The database persists in a named Docker volume (`tabidachi_pgdata_dev`) between restarts.

All dev values are hard-coded in `docker/docker-compose-dev.yml` — nothing to configure. To get higher-quality cover images, optionally set `PEXELS_API_KEY` and/or `UNSPLASH_ACCESS_KEY` in that file.

### Useful commands

```bash
make dev      # start (or restart) the dev stack
make down     # stop all containers
make reset    # full teardown + clean restart (wipes the dev database)
make logs     # follow app logs
```

---

## Self-hosting (production)

### 1. Prerequisites

- A Linux host with Docker
- A reverse proxy (Traefik, Caddy, nginx, …) on a Docker network named `proxy`
- Two host directories for persistent data (database + uploads)

The production Compose file (`docker/docker-compose.yml`) joins an **external** Docker network called `proxy` so your reverse proxy can reach the app container. If you use a different network name, edit the `networks` section in `docker/docker-compose.yml`.

### 2. Create host directories

```bash
# Adjust the base path to wherever you keep Docker data
mkdir -p /opt/docker/tabidachi/database
mkdir -p /opt/docker/tabidachi/uploads
```

### 3. Create the environment file

Copy `.env.example` to `.env` next to the Compose file and fill in real values. At minimum you need `DOCKER_ROOT`, `DB_PASSWORD`, `SESSION_SECRET`, `CSRF_AUTH_KEY`, and `APP_BASE_URL`. See the [environment variable reference](#environment-variable-reference) below for the full list.

```bash
cp .env.example .env
# Generate secrets:
openssl rand -base64 32   # run twice — once for SESSION_SECRET, once for CSRF_AUTH_KEY
```

### 4. Start the stack

```bash
make up
# or directly:
docker compose -f docker/docker-compose.yml up --build -d
```

The app container builds from source on first run, runs database migrations automatically, then listens on port `8080`. Point your reverse proxy at the `tabidachi-app` container on that port.

### 5. Verify

```bash
curl https://tabidachi.example.com/health
# {"status":"ok"}
```

---

## Updating

```bash
git pull
docker compose -f docker/docker-compose.yml up --build -d
```

The app runs database migrations automatically on every startup, so schema upgrades are handled for you. The database and uploads volumes are not touched by a rebuild.

---

## Cover images

Tabidachi automatically fetches a cover photo for each trip and leg on creation. It tries providers in order until one succeeds:

1. **Pexels** — set `PEXELS_API_KEY` (free tier is sufficient)
2. **Unsplash** — set `UNSPLASH_ACCESS_KEY` (free tier is sufficient)
3. **Wikipedia** — no key required; always available as a fallback

Cover images will always be found for any destination. Configuring Pexels or Unsplash simply gives higher-quality results from curated photo libraries.

Images are downloaded once and stored locally under `data/uploads/` inside the container (mapped to `DOCKER_ROOT/tabidachi/uploads` on the host). They are served directly by the app at `/uploads/`.

You can also set cover images manually from the trip editor — search results are proxied through the same download-and-store pipeline so the app never exposes external URLs directly to browsers.

---

## JSON API

Tabidachi exposes a small read-only REST API intended for external clients such as a native mobile app.

### Authentication

All API requests require a **personal access token** (PAT) in the `Authorization` header:

```
Authorization: Bearer tbd_<token>
```

Tokens are generated in **Settings → Personal Access Tokens**. The raw token is shown only once at creation time. Tokens are stored as SHA-256 hashes in the database — a compromised database does not expose usable tokens.

### Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/trips` | List all trips (summary) |
| `GET` | `/api/v1/trips/:id` | Full trip detail including itinerary data |

**Example**

```bash
curl -H "Authorization: Bearer tbd_yourtoken" \
     https://tabidachi.example.com/api/v1/trips
```

The `coverImageUrl` field in responses is always an absolute URL, safe to use directly in a mobile app.

---

## Data model overview

Trips are stored as a PostgreSQL row with a JSONB `data` column. The Tabidachi schema is versioned (`schemaVersion: "1.0"`). The top-level structure:

```
Trip
└── Leg[]            (destination, dates, accommodation)
    └── Day[]        (date, type, notes)
        └── Event[]  (activity | transit | accommodation)
```

You can export any trip to its raw JSON from the trip view page and re-import it (or an LLM-generated JSON) via **New Trip → Import JSON**.

---

## Environment variable reference

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | Yes | — | PostgreSQL connection string |
| `SESSION_SECRET` | Yes | — | Session encryption key (min 32 chars) |
| `CSRF_AUTH_KEY` | Yes | — | CSRF signing key (min 32 chars) |
| `APP_BASE_URL` | No | `http://localhost:8080` | Public URL; used to make API image URLs absolute |
| `PORT` | No | `8080` | Port the app listens on |
| `SECURE_COOKIES` | No | `true` | Set `false` for HTTP-only local development |
| `ALLOW_REGISTRATION` | No | `true` | Set `false` to prevent new user sign-ups |
| `PEXELS_API_KEY` | No | — | Enable Pexels cover image search |
| `UNSPLASH_ACCESS_KEY` | No | — | Enable Unsplash cover image search |

---

## Project structure

```
tabidachi/
├── cmd/tabidachi/     # main entry point, route registration
├── docker/            # Dockerfiles, Compose files, air config
├── internal/
│   ├── auth/          # session management, password hashing
│   ├── config/        # environment variable loading
│   ├── db/            # connection pool, migration runner
│   │   └── migrations/
│   ├── domain/        # core types (Trip, Leg, Day, Event, APIToken…)
│   ├── handler/       # HTTP handlers (trip, builder, image, api, settings…)
│   ├── images/        # cover image fetching (Pexels, Unsplash, Wikipedia)
│   ├── middleware/    # auth, API token auth, request logger
│   └── repository/   # database access (TripStore, UserStore, TokenStore)
└── web/
    ├── static/        # CSS, JS, vendor assets (Shoelace, HTMX)
    └── templates/     # templ components (layouts, pages)
```

---

## License

GPL-2.0
