# Testing ERPNext

## Prerequisites
- Docker and Docker Compose installed
- Port 8080 available

## Running the App (Docker Compose)

ERPNext runs via `frappe_docker` with 11 services:

```bash
# Clone frappe_docker (if not already present)
git clone https://github.com/frappe/frappe_docker.git /home/ubuntu/frappe_docker

# Start all services
cd /home/ubuntu/frappe_docker
docker compose -f pwd.yml up -d
```

### Services
| Service | Role |
|---|---|
| db (MariaDB 10.6) | Database |
| redis-cache | Caching layer |
| redis-queue | Job queue broker |
| backend (Gunicorn) | Python/Frappe API server |
| frontend (Nginx) | Reverse proxy on :8080 |
| websocket (Node.js) | Real-time Socket.IO server |
| queue-short | Background worker (short jobs) |
| queue-long | Background worker (long jobs) |
| scheduler | Cron/scheduled job runner |
| configurator | One-shot site config (exits after run) |
| create-site | One-shot site + app install (exits after run) |

### Startup Time
- Containers start in ~5 seconds
- Site creation (`create-site` service) takes ~60 seconds to install frappe + erpnext
- Monitor with: `docker compose -f pwd.yml logs create-site --follow`

## Default Credentials
- URL: http://localhost:8080
- Username: `Administrator`
- Password: `admin`

## Setup Wizard
After first login, the setup wizard has 3 steps:
1. **Language/Country/Timezone/Currency** — e.g., English, United States, America/New_York, USD
2. **Account Setup** — Full Name, Email, Password (must be "Strong" strength)
3. **Organization** — Company Name, Abbreviation (auto-filled), Chart of Accounts, Financial Year

After completing the wizard, the desk loads at `/desk` with all ERPNext modules.

## Verifying Components

### Backend API
```bash
# Check scheduler status (requires auth session)
curl http://localhost:8080/api/method/frappe.utils.scheduler.get_scheduler_status
# Expected: {"message":{"status":"active"}}
```

### Workers
```bash
# Check worker logs
docker compose -f pwd.yml logs queue-short --tail 10
docker compose -f pwd.yml logs queue-long --tail 10
# Look for "Job OK" messages
```

### Websocket
```bash
docker compose -f pwd.yml logs websocket --tail 5
# Should show: "Realtime service listening on: ws://0.0.0.0:9000"
```

### All Services Status
```bash
docker compose -f pwd.yml ps -a
# All services should be "Up" except configurator and create-site ("Exited (0)")
```

## Key UI Pages to Verify
- Desk home: `/desk` — shows all module icons
- Accounting: `/desk/invoicing` — P&L chart, financial summaries
- Stock: `/desk/stock` — Stock Value chart, warehouse count
- Search bar (Ctrl+K) — triggers backend API calls, verifies full-stack connectivity

## Stopping the App
```bash
docker compose -f pwd.yml down        # Stop and remove containers
docker compose -f pwd.yml down -v     # Also remove volumes (full reset)
```

## Troubleshooting
- If the site doesn't load after startup, check `create-site` logs — it may still be running
- The `background-jobs` page URL may not exist in all versions; verify workers via docker logs instead
- The app requires Python >=3.14 for local (non-Docker) development, which may not be available in standard pyenv — use Docker instead

## Devin Secrets Needed
None required for basic testing with Docker setup (uses default admin/admin credentials).
