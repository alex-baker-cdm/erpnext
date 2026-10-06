# API design & security standards baseline - alex-baker-cdm/erpnext (2026-10-06)

## Summary

| | |
|---|---|
| Repo / branch | `alex-baker-cdm/erpnext` @ `develop` (966e70aebd), fork of frappe/erpnext |
| Classification | provider |
| Surface | REST. Org-added code: Go HTTP service `go-erpnext/` (1 mounted route + catch-all). Upstream: Frappe `/api/method/<dotted.path>` (766 whitelisted functions in `erpnext/`, 6 `allow_guest`) and `/api/resource/<DocType>` |
| Spec present | **No** - no OpenAPI/proto/GraphQL in `go-erpnext/` or `erpnext/`; routing is a hand-maintained nginx template (`go-erpnext/nginx-go-routes.conf`) |
| Auth model | go-erpnext: **none** (no session/API-key validation code; depends on nginx `location =` routing while `docker-compose.override.yml` publishes `8001:8001`). Upstream Frappe: `sid` session cookie or `Authorization: token key:secret`, DocType/role permissions via `frappe.has_permission` |
| Runtime verified | **Yes** - Go server built from `develop` and run on :8001; Frappe stack (`/home/ubuntu/frappe_docker/pwd.yml`, frappe 16.25.0 / erpnext 16.26.1) on :8080; probes with curl + raw sockets; Flt parity computed against Python inside the backend container |
| Severity count | critical 0 · high 3 · medium 6 · low 4 |

**How it was run.** `cd go-erpnext && go build ./cmd/server && go vet ./... && go test ./...` (go1.22.2) - all green. Binary started with `GO_ERPNEXT_PORT=8001`. `govulncheck@v1.8.0 ./...` for area F. The Frappe stack was brought up with `docker compose -f pwd.yml up -d`; the pre-existing site `frontend` had a half-finished erpnext install (`App erpnext is not installed` on first probe), fixed with `bench --site frontend install-app erpnext --force` (environment only - no repo change). The setup wizard was not completed, which limits what upstream guest-write probes can prove (see Needs confirmation). The Go service is *not* part of the compose stack (`docker-compose.override.yml` is a reference file), so it was exercised directly on :8001 - which is also how it would be reachable in any deployment that copies that file.

**What was verified at runtime.** Unauthenticated access to the Go listener; every HTTP verb accepted on the ping route; malformed and 20 MB bodies; CRLF log injection via the path; slowloris (partial header held open >12 s); absence of security/CORS/request-id headers; Go vs Frappe error envelopes; `/healthz` absent; Go `frappe.Flt` vs Python `frappe.utils` rounding on 17 boundary values (8 differ from Frappe's default); Frappe guest 403 on `/api/resource`; dispatch of the two `allow_guest` write methods for Guest.

**Headline.** The Go service is an early scaffold (one route) whose shared primitives already carry the risks that matter for the migration: no auth hook (ERP-B-01), a rounding primitive that disagrees with Frappe on half-cent values (ERP-C-01), an EOL toolchain with 29 reachable stdlib CVEs (ERP-F-01), and no CI gate or contract (ERP-G-01, ERP-A-01). Fixing these before the first business route is mounted is cheap (mostly S effort).

## Route inventory

### go-erpnext (`cmd/server/main.go`)

| Method | Path | Handler | Auth | Notes |
|---|---|---|---|---|
| ANY | `/api/method/go_erpnext.ping` | `pingHandler` (main.go:37-41) | none | returns `{"message":"pong"}`; GET/POST/DELETE/OPTIONS all 200 |
| ANY | `/` (catch-all) | `catchAllHandler` (main.go:44-48) | none | `404 {"error":"not implemented in Go service"}` |

nginx (`nginx-go-routes.conf`) forwards only `location = /api/method/go_erpnext.ping` to upstream `go-erpnext:8001`. Outbound calls: none (pkg/db opens MariaDB using `site_config.json` but is not wired into the server).

### upstream Frappe/ERPNext (not fully audited - context only)

| Pattern | Auth | Notes |
|---|---|---|
| `/api/resource/<DocType>[/<name>]` | session / token; DocType permissions | Guest -> 403 PermissionError (verified) |
| `/api/method/<module.path.fn>` | session / token unless `allow_guest=True` | 766 `@frappe.whitelist` functions in `erpnext/`; 6 `allow_guest` (book_appointment: get_appointment_settings, get_timezones, get_appointment_slots, create_appointment; search_help.get_help_results_sections; templates.utils.send_message) |

## Findings by area

### A. Contract & design

| id | severity | title | file:lines | effort |
|---|---|---|---|---|
| ERP-A-01 | medium | go-erpnext: no API contract and no HTTP-method restriction on the only mounted route | `go-erpnext/cmd/server/main.go:36-48, 56-59` | S |
| ERP-A-02 | low | upstream: Frappe REST surface (/api/method, /api/resource) has no machine-readable spec to diff the Go port against | `erpnext/hooks.py:1-end (no spec artefact in repo)` | M |

### B. Authentication & authorization

| id | severity | title | file:lines | effort |
|---|---|---|---|---|
| ERP-B-01 | high | go-erpnext: no authentication or authorization layer; Go port published directly on host port 8001, bypassing nginx | `go-erpnext/cmd/server/main.go:56-66` | M |
| ERP-B-02 | low | upstream: two allow_guest write methods are reachable unauthenticated and the erpnext wrappers have no rate limit | `erpnext/www/book_appointment/index.py:94-113 (and erpnext/templates/utils.py:9-60)` | S |

### C. Input validation & money handling

| id | severity | title | file:lines | effort |
|---|---|---|---|---|
| ERP-C-01 | high | go-erpnext: frappe.Flt rounding diverges from Python frappe.utils.flt for half-cent values (money-correctness in taxmath, valuation, paymentterms) | `go-erpnext/pkg/frappe/utils.go:374-387` | M |
| ERP-C-02 | medium | go-erpnext: db.GetList concatenates ORDER BY unsanitized (latent SQL injection in the shared DB helper) | `go-erpnext/pkg/db/db.go:207-209` | S |
| ERP-C-03 | medium | go-erpnext: http.Server has no read/idle timeouts, header limit or body size limit (slowloris / resource exhaustion) | `go-erpnext/cmd/server/main.go:63-66` | S |

### D. Error handling

| id | severity | title | file:lines | effort |
|---|---|---|---|---|
| ERP-D-01 | medium | go-erpnext: error envelope differs from Frappe's and no correlation/request id is returned or logged | `go-erpnext/cmd/server/main.go:43-48` | S |

### E. Logging & observability

| id | severity | title | file:lines | effort |
|---|---|---|---|---|
| ERP-E-01 | medium | go-erpnext: unstructured printf logging with raw request path (log injection verified); no client IP, request id or level | `go-erpnext/cmd/server/main.go:14-23` | S |
| ERP-E-02 | low | go-erpnext: health endpoint is liveness only; no readiness, metrics or tracing hooks | `go-erpnext/cmd/server/main.go:36-41, 57` | S |

### F. Security hygiene

| id | severity | title | file:lines | effort |
|---|---|---|---|---|
| ERP-F-01 | high | go-erpnext: built on end-of-life Go 1.22; govulncheck finds 29 reachable stdlib vulnerabilities (net/http, database/sql) with fixes available | `go-erpnext/go.mod:3 (and Dockerfile:1)` | S |
| ERP-F-02 | low | go-erpnext: no security headers, CORS policy or rate limiting on the Go listener; container runs as root | `go-erpnext/cmd/server/main.go:37-41, 44-48 (and Dockerfile:9-12)` | S |

### G. Lifecycle

| id | severity | title | file:lines | effort |
|---|---|---|---|---|
| ERP-G-01 | medium | go-erpnext: no CI gate, changelog, version or rollback path for the Go service | `.github/workflows:(18 workflows, none reference go-erpnext)` | S |

## Detailed findings

### ERP-A-01 - go-erpnext: no API contract and no HTTP-method restriction on the only mounted route

**Severity:** medium · **Effort:** S · **File:** `go-erpnext/cmd/server/main.go:36-48, 56-59`

**Evidence**

```text
No OpenAPI/proto/GraphQL document exists anywhere in go-erpnext/ (only nginx-go-routes.conf, a hand-maintained nginx template with a commented-out 'Template for adding more Go routes'). The single route is registered with mux.HandleFunc without a method, so every verb returns 200:

### curl -s -i -X DELETE http://localhost:8001/api/method/go_erpnext.ping
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 19

{"message":"pong"}

### curl -s -i -X OPTIONS http://localhost:8001/api/method/go_erpnext.ping -H 'Origin: https://evil.example'
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 19

{"message":"pong"}

Malformed JSON body is also accepted (handler never reads the body):

### curl -s -i -X POST http://localhost:8001/api/method/go_erpnext.ping -H 'Content-Type: application/json' --data-binary 'not json {{{'
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 19

{"message":"pong"}

git log -p -- go-erpnext/cmd/server/main.go: one commit (2c1c964365, 2026-03-20), two HandleFunc lines ever; no version segment in the path and no versioning strategy documented.
```

**Remediation**

Add go-erpnext/openapi.yaml (OpenAPI 3.1) as the contract for every route the Go service takes over from Frappe, generate the server interface with oapi-codegen so the mounted routes cannot drift from the spec, and lint the spec with Spectral in CI. Use Go 1.22 method-qualified patterns (mux.HandleFunc("GET /api/method/go_erpnext.ping", ...)) so non-matching verbs get 405 with an Allow header. Decide the version strategy up front (path /api/v2/... matching Frappe's own v2 convention, or a header) and record it in the spec's info.version.

### ERP-A-02 - upstream: Frappe REST surface (/api/method, /api/resource) has no machine-readable spec to diff the Go port against

**Severity:** low · **Effort:** M · **File:** `erpnext/hooks.py:1-end (no spec artefact in repo)`

**Evidence**

```text
The surface the Go service is replacing route-by-route is Frappe's generic /api/resource/<DocType> CRUD and /api/method/<dotted.python.path> RPC (766 @frappe.whitelist functions in erpnext/ per AST scan). Neither frappe nor this repo ships an OpenAPI document, so there is nothing to diff a Go implementation against; parity today rests on JSON fixtures extracted by go-erpnext/scripts/extract_fixtures.py. Frappe's envelope observed at runtime:

### curl -s -m 20 -w '\nHTTP %{http_code}\n' 'http://localhost:8080/api/resource/Customer?limit_page_length=2'
{"exc_type":"PermissionError","_server_messages":"[\"{\\\"message\\\":\\\"User <strong>Guest</strong> does not have doctype access via role permission for document <strong>DocType</strong>\\\",\\\"as_table\\\":false,\\\"title\\\":\\\"Message\\\"}\"]","_error_message":"No permission for DocType"}
HTTP 403
```

**Remediation**

Generate a per-route OpenAPI fragment for each Frappe method the Go service takes over (request params from the Python signature/type hints, response from a captured fixture) and keep it under go-erpnext/openapi/. Treat the fixture files in go-erpnext/testdata as contract tests and run them against both the Python and Go implementations in CI (consumer-driven contract pattern).

### ERP-B-01 - go-erpnext: no authentication or authorization layer; Go port published directly on host port 8001, bypassing nginx

**Severity:** high · **Effort:** M · **File:** `go-erpnext/cmd/server/main.go:56-66`

**Evidence**

```text
main.go contains no middleware that validates a Frappe `sid` cookie or `Authorization: token key:secret`; pkg/db has helpers but nothing reads tabUser.api_key/api_secret or the Redis session store. The only guard is nginx `location =` path routing (nginx-go-routes.conf:12-19), yet docker-compose.override.yml:14-15 publishes 8001:8001 to the host, so the service is reachable without nginx. Verified unauthenticated access and that credentials are ignored:

### curl -s -i http://localhost:8001/api/method/go_erpnext.ping
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 19

{"message":"pong"}

### curl -s -i http://localhost:8001/api/method/go_erpnext.ping -H 'X-Request-Id: abc-123' -H 'Authorization: token aaa:bbb'
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 19

{"message":"pong"}

Today only the ping route exists, so no data is exposed - this is rated high (not critical) because the gap is architectural: the first business route mounted here will be unauthenticated by default.
```

**Remediation**

Add an authn middleware applied to every route except liveness: accept `Authorization: token <api_key>:<api_secret>` and validate against `tabUser` (api_key, decrypted api_secret) via pkg/db, and accept the Frappe `sid` cookie by looking the session up in Frappe's Redis session store (or by calling /api/method/frappe.auth.get_logged_user on the backend). Propagate the resolved user into context and enforce DocType/row permission on every data route (equivalent of frappe.has_permission). Remove the `ports:` publish from docker-compose.override.yml (expose only on the compose network) or bind to 127.0.0.1, so nginx remains the only ingress.

### ERP-C-01 - go-erpnext: frappe.Flt rounding diverges from Python frappe.utils.flt for half-cent values (money-correctness in taxmath, valuation, paymentterms)

**Severity:** high · **Effort:** M · **File:** `go-erpnext/pkg/frappe/utils.go:374-387`

**Evidence**

```text
Flt rounds via strconv.FormatFloat(v,'f',precision) (correctly-rounded on the binary value) and ignores System Settings.rounding_method. Python frappe.utils.flt -> rounded() uses one of three methods (default "Banker's Rounding (legacy)": multiply, round to 8 dp, then round-half-up; "Banker's Rounding"; "Commercial Rounding"). Compared Go (go run, pkg/frappe.Flt) against the three Python implementations executed inside the frappe backend container (frappe 16.25.0):

value | Go frappe.Flt(v,2) | Python legacy (default) | Python Banker's | Python Commercial
2.675 | 2.67 | 2.68 | 2.68 | 2.68 <-- differs from default (legacy)
1.005 | 1 | 1.01 | 1.0 | 1.01 <-- differs from default (legacy)
0.125 | 0.12 | 0.13 | 0.12 | 0.13 <-- differs from default (legacy)
0.375 | 0.38 | 0.38 | 0.38 | 0.38
2.5 | 2.5 | 2.5 | 2.5 | 2.5
3.5 | 3.5 | 3.5 | 3.5 | 3.5
1.15 | 1.15 | 1.15 | 1.15 | 1.15
100.005 | 100 | 100.01 | 100.0 | 100.01 <-- differs from default (legacy)
1234.5675 | 1234.57 | 1234.57 | 1234.57 | 1234.57
0.045 | 0.04 | 0.05 | 0.04 | 0.05 <-- differs from default (legacy)
1.0049999999 | 1 | 1.0 | 1.0 | 1.0
19.995 | 20 | 20.0 | 20.0 | 20.0
2.345 | 2.35 | 2.35 | 2.34 | 2.35
8.325 | 8.32 | 8.33 | 8.32 | 8.33 <-- differs from default (legacy)
5.015 | 5.01 | 5.02 | 5.02 | 5.02 <-- differs from default (legacy)
1.235 | 1.24 | 1.24 | 1.24 | 1.24
2.3449999999999998 | 2.34 | 2.35 | 2.34 | 2.35 <-- differs from default (legacy)

8 of 17 probe values differ from Frappe's default method (e.g. 2.675 -> Go 2.67 vs Python 2.68; 1.005 -> 1.00 vs 1.01; 100.005 -> 100.00 vs 100.01) and Go matches none of the three methods consistently (5.015 -> Go 5.01, every Python method 5.02). Flt is the rounding primitive for taxmath.SetInCompanyCurrency/CalculateItemValues (taxmath.go:88,130-169), valuation.RoundOffIfNearZero/getTotalStockAndValue (valuation.go:32-35,282-285) and paymentterms.GetPaymentTermDetails (paymentterms.go:344-345). Existing fixtures (testdata/*.json) pass because they contain no half-unit boundary cases. Not reachable over HTTP yet (no route mounted); would be critical once any pricing/tax/valuation route is served by Go.
```

**Remediation**

Reimplement Flt/rounded on an exact decimal type (shopspring/decimal or cockroachdb/apd) with the three Frappe strategies (_bankers_rounding_legacy, _bankers_rounding, _round_away_from_zero) selected by System Settings.rounding_method read through pkg/db, and keep float64 only at the JSON boundary. Extend scripts/extract_fixtures.py to emit half-unit boundary cases (x.xx5, 0.045, 100.005, negative values) for every rounding method and add a rapid property test asserting Go == Python on those fixtures before any money route is mounted.

### ERP-C-02 - go-erpnext: db.GetList concatenates ORDER BY unsanitized (latent SQL injection in the shared DB helper)

**Severity:** medium · **Effort:** S · **File:** `go-erpnext/pkg/db/db.go:207-209`

**Evidence**

```text
GetList validates doctype, field and filter identifiers with validIdentifier (db.go:19-28) and binds filter values with `?`, but `opts.OrderBy` is appended verbatim: `query += " ORDER BY " + opts.OrderBy`. Any future handler that maps a request parameter (Frappe's `order_by`) onto ListOptions.OrderBy would allow `name; DROP ...`/UNION injection. Code-verified only: no mounted route calls GetList today, so there is no runtime transcript.
```

**Remediation**

Parse OrderBy into (field, direction) and validate the field with sanitizeIdentifier and the direction against {ASC, DESC} before formatting; alternatively accept a typed `[]OrderBy{Field string; Desc bool}` so callers cannot pass raw SQL. Add a unit test that rejects `name; DROP TABLE x` and keep the Go `db` package behind an allow-list of fields per DocType (DoctypeSchema.GetFieldNames already provides it).

### ERP-C-03 - go-erpnext: http.Server has no read/idle timeouts, header limit or body size limit (slowloris / resource exhaustion)

**Severity:** medium · **Effort:** S · **File:** `go-erpnext/cmd/server/main.go:63-66`

**Evidence**

```text
srv := &http.Server{Addr, Handler} sets none of ReadHeaderTimeout, ReadTimeout, WriteTimeout, IdleTimeout or MaxHeaderBytes, and no handler wraps r.Body in http.MaxBytesReader. Verified: a raw TCP client that sends a partial request header and then one byte every 1.5 s is never disconnected -

python3 raw-socket probe: sent 'GET /api/method/go_erpnext.ping HTTP/1.1\r\nHost: x\r\nX-Slow: ' then 1 byte every 1.5s -> "partial-header connection still open after 12.0 s: True" (server never closed the connection)

A 20 MB POST body is accepted with 200 (handler ignores the body, so curl reported sent=0; a future body-reading handler would buffer it unbounded):

### head -c 20000000 /dev/zero | tr '\0' 'A' > big.bin; curl -s -o /dev/null -w 'oversized 20MB body -> HTTP %{http_code} sent=%{size_upload} time=%{time_total}s\n' -X POST http://localhost:8001/api/method/go_erpnext.ping --data-binary @big.bin
oversized 20MB body -> HTTP 200 sent=0 time=0.000463s
```

**Remediation**

Set ReadHeaderTimeout: 5s, ReadTimeout: 15s, WriteTimeout: 30s, IdleTimeout: 60s, MaxHeaderBytes: 1<<16 on http.Server, and add a middleware that does r.Body = http.MaxBytesReader(w, r.Body, 1<<20) (returning 413) before any JSON decoding. Reject unexpected Content-Types with 415 and decode with json.Decoder.DisallowUnknownFields into per-route request structs validated by go-playground/validator.

### ERP-D-01 - go-erpnext: error envelope differs from Frappe's and no correlation/request id is returned or logged

**Severity:** medium · **Effort:** S · **File:** `go-erpnext/cmd/server/main.go:43-48`

**Evidence**

```text
Go errors are `{"error": "..."}` while the Frappe backend the Go service sits beside returns `{"exc_type", "_server_messages", "_error_message"}` (and `{"message": ...}` on success, which Go does match). Clients of /api/method/* therefore see two error shapes depending on which backend nginx routed to. No X-Request-ID is accepted, generated or echoed (the one sent was ignored) and log lines carry no id to correlate with a client report.

Go:
### curl -s -i http://localhost:8001/api/method/erpnext.stock.valuation.get_fifo_rate
HTTP/1.1 404 Not Found
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 42

{"error":"not implemented in Go service"}

Frappe (same site, unauthenticated):
### curl -s -m 20 -w '\nHTTP %{http_code}\n' 'http://localhost:8080/api/resource/Customer?limit_page_length=2'
{"exc_type":"PermissionError","_server_messages":"[\"{\\\"message\\\":\\\"User <strong>Guest</strong> does not have doctype access via role permission for document <strong>DocType</strong>\\\",\\\"as_table\\\":false,\\\"title\\\":\\\"Message\\\"}\"]","_error_message":"No permission for DocType"}
HTTP 403

Request id ignored:
### curl -s -i http://localhost:8001/api/method/go_erpnext.ping -H 'X-Request-Id: abc-123' -H 'Authorization: token aaa:bbb'
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 19

{"message":"pong"}
```

**Remediation**

Pick one envelope for the whole org and implement it as a single writeError(w, r, status, code, msg) helper: either mirror Frappe's shape exactly (so a route moving from Python to Go is invisible to clients) or adopt RFC 9457 application/problem+json {type,title,status,detail,instance} and have Frappe map to it at the nginx edge. Add a request-id middleware (reuse incoming X-Request-ID, else uuid.NewString()), echo it in the response header, put it in `instance`/`request_id` on errors and in every log line.

### ERP-E-01 - go-erpnext: unstructured printf logging with raw request path (log injection verified); no client IP, request id or level

**Severity:** medium · **Effort:** S · **File:** `go-erpnext/cmd/server/main.go:14-23`

**Evidence**

```text
loggingMiddleware calls log.Printf("%s %s %d %s", r.Method, r.URL.Path, status, duration). The decoded path is written verbatim, so CR/LF in the URL forges log lines:

### curl -s -i -H 'X-Forwarded-For: 1.2.3.4' 'http://localhost:8001/foo%0d%0aInjected:%20header'
HTTP/1.1 404 Not Found
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 42

{"error":"not implemented in Go service"}

server.log then contains two lines for one request:
2026/10/06 08:06:08 GET /foo
Injected: header 404 8.283µs

There is no level field, so 5xx cannot be alerted on without regex-parsing the status column; X-Real-IP/X-Forwarded-For set by nginx are not logged; request bodies/tokens are not logged (good).
```

**Remediation**

Switch to log/slog with slog.NewJSONHandler, logging method, path via strconv.Quote (or r.URL.EscapedPath()), status, duration_ms, bytes, remote_ip (X-Real-IP), user (once ERP-B-01 exists) and request_id; use slog.LevelError for status >= 500 and LevelWarn for 4xx so a log-based alert can key on `level`. Never log Authorization headers or bodies.

### ERP-E-02 - go-erpnext: health endpoint is liveness only; no readiness, metrics or tracing hooks

**Severity:** low · **Effort:** S · **File:** `go-erpnext/cmd/server/main.go:36-41, 57`

**Evidence**

```text
/api/method/go_erpnext.ping returns {"message":"pong"} unconditionally; pkg/db.New (which does conn.Ping) is never wired into the server, so readiness of the MariaDB dependency is not reported. Conventional probe paths are not served and there is no /metrics:

### curl -s -i 'http://localhost:8001/healthz'
HTTP/1.1 404 Not Found
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 42

{"error":"not implemented in Go service"}
```

**Remediation**

Expose /livez (process up) and /readyz (db.PingContext with 1s timeout, returning 503 with the failing dependency) and point the Docker HEALTHCHECK / compose healthcheck at /readyz. Add prometheus/client_golang promhttp on /metrics (request count/latency by route and status) and wrap the mux with go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp so traces propagate from nginx/Frappe.

### ERP-F-01 - go-erpnext: built on end-of-life Go 1.22; govulncheck finds 29 reachable stdlib vulnerabilities (net/http, database/sql) with fixes available

**Severity:** high · **Effort:** S · **File:** `go-erpnext/go.mod:3 (and Dockerfile:1)`

**Evidence**

```text
govulncheck@v1.8.0 against go1.22.2: 'Your code is affected by 29 vulnerabilities from the Go standard library' (all traced from server.main -> http.Server.ListenAndServe or db.New -> sql.Open). Examples: GO-2026-6089 net/http (fixed go1.25.13), GO-2026-5039 net/textproto (fixed 1.25.11), GO-2025-3563 net/http/internal (fixed 1.23.8), GO-2025-3849 database/sql (fixed 1.23.12), GO-2024-2824 net (fixed 1.22.3). Module-level: filippo.io/edwards25519 v1.1.0 -> GO-2026-4503 (fixed 1.1.1), not called by this code. Dockerfile builds FROM golang:1.22-alpine; Go 1.22 reached end-of-life Aug 2025 so no toolchain in that line receives the 1.23+/1.24+/1.25+ fixes.

Full output in audit appendix (govulncheck ./... run on 2026-10-06).
```

**Remediation**

Bump go.mod to `go 1.25` with a `toolchain go1.25.13` directive, change Dockerfile to `FROM golang:1.25-alpine` (and pin by digest), run `go get -u filippo.io/edwards25519@v1.1.1 && go mod tidy`, and add `govulncheck ./...` to the CI job proposed in ERP-G-01 so the baseline stays clean. Re-run govulncheck after the bump to confirm 0 reachable findings.

### ERP-F-02 - go-erpnext: no security headers, CORS policy or rate limiting on the Go listener; container runs as root

**Severity:** low · **Effort:** S · **File:** `go-erpnext/cmd/server/main.go:37-41, 44-48 (and Dockerfile:9-12)`

**Evidence**

```text
Every response carries only Content-Type, Date and Content-Length - no X-Content-Type-Options, Strict-Transport-Security, Cache-Control or Content-Security-Policy, and no rate limiter exists (Frappe's @rate_limit is Python-only). Dockerfile final stage has no USER instruction, so the binary runs as root in alpine:3.19. Relevant because ERP-B-01 shows the listener is reachable without nginx:

### curl -s -i http://localhost:8001/api/method/go_erpnext.ping
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 19

{"message":"pong"}
```

**Remediation**

Add a small headers middleware (X-Content-Type-Options: nosniff, Cache-Control: no-store, Strict-Transport-Security when behind TLS, X-Frame-Options: DENY) and an explicit CORS policy (deny by default; rs/cors with the Frappe site origin if browser calls are expected). Add per-IP/per-user rate limiting with golang.org/x/time/rate keyed on X-Real-IP/user, matching Frappe's limits. In the Dockerfile add `USER 65532:65532` (or use gcr.io/distroless/static) and a HEALTHCHECK.

### ERP-G-01 - go-erpnext: no CI gate, changelog, version or rollback path for the Go service

**Severity:** medium · **Effort:** S · **File:** `.github/workflows:(18 workflows, none reference go-erpnext)`

**Evidence**

```text
`grep -ril 'go-erpnext\|golang\|setup-go' .github/workflows` returns nothing: none of backport, docker-release, linters, server-tests-* etc. build, vet, test or scan go-erpnext. go-erpnext has no CHANGELOG, no version constant, no deprecation markers, and the Dockerfile image is untagged. docker-compose.override.yml is a 'reference' file not consumed by /home/ubuntu/frappe_docker/pwd.yml, and nginx-go-routes.conf is not included by the frontend image, so there is no defined deploy or rollback procedure (verified: the local frappe_docker stack runs without the Go service; build/vet/test only pass when run by hand).
```

**Remediation**

Add .github/workflows/go-erpnext.yml (actions/setup-go from go.mod, `go build ./...`, `go vet ./...`, `go test -race ./...`, govulncheck, staticcheck, Spectral lint of openapi.yaml, docker build) on paths: go-erpnext/**. Version the image with semver tags produced from a CHANGELOG.md/Release Please, and document rollback as flipping the nginx `location =` block back to the Frappe upstream (a one-line revert) so route-by-route cutover is reversible.

### ERP-B-02 - upstream: two allow_guest write methods are reachable unauthenticated and the erpnext wrappers have no rate limit

**Severity:** low · **Effort:** S · **File:** `erpnext/www/book_appointment/index.py:94-113 (and erpnext/templates/utils.py:9-60)`

**Evidence**

```text
AST scan of erpnext/ finds 6 @frappe.whitelist(allow_guest=True) methods; two perform writes with ignore_permissions=True: create_appointment (Appointment.insert from caller-supplied JSON `contact`) and send_message (Lead + Opportunity + Communication inserts). create_appointment has no @rate_limit; erpnext's send_message wrapper has none either (only the inner frappe.www.contact.send_message is limited to 1000/hour). Verified they are dispatched for Guest (no PermissionError, unlike /api/resource which returns 403):

### curl -s -m 30 -L -w '\nHTTP %{http_code}\n' -X POST 'http://localhost:8080/api/method/erpnext.www.book_appointment.index.create_appointment' -H 'Accept: application/json' -H 'X-Requested-With: XMLHttpRequest' -H 'Content-Type: application/json' -d '{"date":"2026-10-20","time":"10:00:00","tz":"UTC","contact":"{\"name\":\"Audit Guest\",\"email\":\"audit-guest@example.com\",\"number\":\"123\"}"}' | sed 's/<[^>]*>//g' | grep -v '^\s*$' | head -20
{"exc_type":"Redirect"}
HTTP 301

### curl -s -m 30 -w '\nHTTP %{http_code}\n' -X POST 'http://localhost:8080/api/method/erpnext.templates.utils.send_message' -H 'Accept: application/json' -H 'Content-Type: application/json' -d '{"sender":"audit-guest3@example.com","message":"hello from audit 3","subject":"Audit probe 3"}'
{}
HTTP 200

The write side-effects could NOT be confirmed on this site (fresh install, setup wizard not completed, Contact Us Settings disabled): create_appointment raised a Redirect and send_message returned 200 with zero Lead/Opportunity/Communication rows afterwards - see needs_confirmation.
```

**Remediation**

These are public web-form endpoints by design; the org standard should still require every allow_guest write to carry @frappe.rate_limit (key on IP, e.g. limit=5/hour for create_appointment), a captcha/honeypot, strict pydantic-style validation of the `contact` JSON (max lengths, email format via validate_email_address) and to be disabled by default via settings. Track upstream (frappe/erpnext) rather than patch the fork.

## Positives

- go-erpnext/pkg/db: identifiers (doctype, field, filter field) are allow-listed with ^[a-zA-Z0-9_ ]+$ and all filter values / names are bound with `?` placeholders (db.go:19-28, 105, 153, 201) - a good reference for safe dynamic SQL against Frappe tables.
- go-erpnext: money/tax/valuation logic is pure (no I/O) and covered by property-based tests (pgregory.net/rapid) plus JSON fixtures extracted from the Python implementation (scripts/extract_fixtures.py, testdata/*.json); `go build`, `go vet ./...` and `go test ./...` all pass - the fixture-parity harness is a reusable pattern for other ports.
- go-erpnext/cmd/server: graceful shutdown on SIGINT/SIGTERM with a 10 s drain (main.go:68-90) and a logging middleware that captures status and latency for every request.
- go-erpnext: deny-by-default routing - unknown paths return a JSON 404 from the Go service instead of falling through (main.go:43-48, 59), so a misrouted nginx location cannot silently proxy to the wrong backend.
- go-erpnext: no credentials in the repo - DB credentials are read at runtime from Frappe's site_config.json (db.go:45-67); multi-stage Dockerfile with CGO_ENABLED=0 produces a minimal static binary.
- upstream/Frappe: /api/resource is deny-by-default for Guest (403 PermissionError with a consistent {exc_type,_server_messages,_error_message} envelope) and whitelisted methods must opt in to allow_guest - only 6 of 766 do.

## Needs confirmation

- upstream: whether the two allow_guest write methods actually persist rows when the site is fully configured. On this fresh site create_appointment -> 301 {"exc_type":"Redirect"} and send_message -> 200 {} with 0 Lead/Opportunity/Communication rows afterwards (Contact Us Settings is_disabled=1, setup wizard not run). Re-test on a configured site with Appointment Booking Settings agents and Contact Us enabled.
- upstream: 106 of 766 @frappe.whitelist functions match the heuristic 'performs a write (.insert/.save/.submit/db.set_value/raw UPDATE) with no has_permission/check_permission in the function body'. Most rely on Document.save()/insert() implicit permission checks and are probably fine; the 15 sampled below use frappe.db.set_value or save(ignore_permissions=True), which bypass those checks, and should be verified with a non-System-Manager user: erpnext/www/book_appointment/index.py:95 create_appointment (allow_guest=True; Appointment.insert(ignore_permissions=True)); erpnext/templates/utils.py:10 send_message (allow_guest=True; Lead/Opportunity/Communication insert(ignore_permissions=True); no @rate_limit on the erpnext wrapper); erpnext/support/doctype/issue/issue.py:225 set_status (frappe.db.set_value bypasses Issue write permission); erpnext/support/doctype/issue/issue.py:219 set_multiple_status (frappe.db.set_value); erpnext/accounts/doctype/process_payment_reconciliation/process_payment_reconciliation.py:132 pause_job_for_doc (frappe.db.set_value); erpnext/stock/doctype/serial_and_batch_bundle/serial_and_batch_bundle.py:2197 update_serial_or_batch (frappe.db.set_value + save(ignore_permissions=True)); erpnext/manufacturing/doctype/workstation/workstation.py:211 start_job (Job Card save(ignore_permissions=True)); erpnext/manufacturing/doctype/workstation/workstation.py:219 complete_job (save(ignore_permissions=True) + submit); erpnext/accounts/doctype/pos_profile/pos_profile.py:322 set_default_profile (raw UPDATE via frappe.db.sql; scoped to session user); erpnext/crm/doctype/opportunity/opportunity.py:500 set_multiple_status; erpnext/projects/doctype/task/task.py:365 set_multiple_status; erpnext/crm/frappe_crm_api.py:154 create_customer (insert(ignore_permissions=True)); erpnext/accounts/utils.py:441 add_ac (honours caller-supplied ignore_permissions flag from form_dict); erpnext/stock/doctype/bin/bin.py:40 recalculate_qty (self.save()); erpnext/buying/doctype/purchase_order/purchase_order.py:783 make_purchase_invoice_from_portal (has an explicit Portal User check - likely OK)
- upstream: remaining 91 heuristic matches not sampled (file:line:function): erpnext/accounts/doctype/account/account.py:425:convert_group_to_ledger, erpnext/accounts/doctype/account/account.py:436:convert_ledger_to_group, erpnext/accounts/doctype/account/account.py:518:update_account_number, erpnext/accounts/doctype/bank_clearance/bank_clearance.py:93:update_clearance_date, erpnext/accounts/doctype/bank_reconciliation_tool/bank_reconciliation_tool.py:112:update_bank_transaction, erpnext/accounts/doctype/bank_reconciliation_tool/bank_reconciliation_tool.py:142:create_journal_entry_bts, erpnext/accounts/doctype/bank_reconciliation_tool/bank_reconciliation_tool.py:301:create_payment_entry_bts, erpnext/accounts/doctype/bank_reconciliation_tool/bank_reconciliation_tool.py:479:reconcile_vouchers, erpnext/accounts/doctype/bank_transaction/bank_transaction.py:229:remove_payment_entries, erpnext/accounts/doctype/bank_transaction/bank_transaction_upload.py:38:create_bank_entries, erpnext/accounts/doctype/bisect_accounting_statements/bisect_accounting_statements.py:125:build_tree, erpnext/accounts/doctype/bisect_accounting_statements/bisect_accounting_statements.py:188:bisect_left, erpnext/accounts/doctype/bisect_accounting_statements/bisect_accounting_statements.py:202:bisect_right, erpnext/accounts/doctype/bisect_accounting_statements/bisect_accounting_statements.py:216:move_up, erpnext/accounts/doctype/budget/budget.py:848:revise_budget, erpnext/accounts/doctype/cheque_print_template/cheque_print_template.py:50:create_or_update_cheque_print_format, erpnext/accounts/doctype/cost_center/cost_center.py:59:convert_group_to_ledger, erpnext/accounts/doctype/cost_center/cost_center.py:70:convert_ledger_to_group, erpnext/accounts/doctype/party_link/party_link.py:70:create_party_link, erpnext/accounts/doctype/pos_invoice/pos_invoice.py:798:create_payment_request, erpnext/accounts/doctype/pos_invoice/pos_invoice.py:858:update_payments, erpnext/accounts/doctype/process_payment_reconciliation/process_payment_reconciliation.py:141:trigger_job_for_doc, erpnext/accounts/doctype/process_period_closing_voucher/process_period_closing_voucher.py:240:schedule_next_date, erpnext/accounts/doctype/process_period_closing_voucher/process_period_closing_voucher.py:93:start_pcv_processing, erpnext/accounts/doctype/repost_payment_ledger/repost_payment_ledger.py:24:start_payment_ledger_repost, erpnext/accounts/doctype/subscription/subscription.py:558:process, erpnext/accounts/doctype/subscription/subscription.py:689:cancel_subscription, erpnext/accounts/doctype/subscription/subscription.py:712:restart_subscription, erpnext/accounts/doctype/unreconcile_payment/unreconcile_payment.py:197:create_unreconcile_doc_for_selection, erpnext/accounts/utils.py:1548:update_cost_center, erpnext/accounts/utils.py:473:add_cc, erpnext/assets/doctype/location/location.py:233:add_node, erpnext/buying/doctype/request_for_quotation/request_for_quotation.py:477:create_supplier_quotation, erpnext/buying/doctype/supplier_scorecard/supplier_scorecard.py:201:make_all_scorecards, erpnext/buying/report/supplier_quotation_comparison/supplier_quotation_comparison.py:298:set_default_supplier, erpnext/controllers/accounts_controller.py:3004:repost_accounting_entries, erpnext/controllers/accounts_controller.py:4381:update_company_master_and_address, erpnext/controllers/stock_controller.py:2131:make_quality_inspections, erpnext/crm/doctype/lead/lead.py:491:make_lead_from_communication, erpnext/crm/doctype/lead/lead.py:540:add_lead_to_prospect, erpnext/crm/doctype/opportunity/opportunity.py:265:declare_enquiry_lost, erpnext/crm/doctype/opportunity/opportunity.py:530:make_opportunity_from_communication, erpnext/crm/frappe_crm_api.py:32:create_prospect_against_crm_deal, erpnext/crm/utils.py:245:add_note, erpnext/crm/utils.py:258:delete_note, erpnext/edi/doctype/code_list/code_list_import.py:12:import_genericode, erpnext/erpnext_integrations/doctype/plaid_settings/plaid_settings.py:54:add_institution, erpnext/erpnext_integrations/doctype/plaid_settings/plaid_settings.py:82:add_bank_accounts, erpnext/manufacturing/doctype/bom/bom.py:835:add_raw_materials, erpnext/manufacturing/doctype/bom_creator/bom_creator.py:175:add_boms, erpnext/manufacturing/doctype/bom_creator/bom_creator.py:418:add_item, erpnext/manufacturing/doctype/bom_creator/bom_creator.py:450:add_sub_assembly, erpnext/manufacturing/doctype/bom_creator/bom_creator.py:539:delete_node, erpnext/manufacturing/doctype/job_card/job_card.py:1445:complete_job_card, erpnext/manufacturing/doctype/job_card/job_card.py:1478:make_stock_entry_for_semi_fg_item, erpnext/manufacturing/doctype/master_production_schedule/master_production_schedule.py:312:fetch_materials_requests, erpnext/manufacturing/doctype/master_production_schedule/master_production_schedule.py:368:fetch_sales_orders, erpnext/manufacturing/doctype/master_production_schedule/master_production_schedule.py:46:get_actual_demand, erpnext/manufacturing/doctype/production_plan/production_plan.py:958:make_material_request, erpnext/manufacturing/doctype/work_order/work_order.py:2374:set_work_order_ops, erpnext/projects/doctype/project/project.py:557:create_duplicate_project, erpnext/projects/doctype/task/task.py:447:add_node, erpnext/projects/doctype/task/task.py:461:add_multiple_tasks, erpnext/quality_management/doctype/quality_procedure/quality_procedure.py:155:add_node, erpnext/selling/doctype/customer/customer.py:196:get_customer_group_details, erpnext/selling/doctype/quotation/quotation.py:261:declare_enquiry_lost, erpnext/selling/doctype/sales_order/sales_order.py:1596:make_purchase_order, erpnext/selling/doctype/sales_order/sales_order.py:1785:make_work_orders, erpnext/selling/doctype/sales_order/sales_order.py:887:create_delivery_schedule, erpnext/selling/page/point_of_sale/point_of_sale.py:338:create_opening_voucher, erpnext/selling/page/point_of_sale/point_of_sale.py:429:set_customer_info, erpnext/setup/doctype/company/company.py:961:add_node, erpnext/setup/doctype/department/department.py:100:add_node, erpnext/setup/doctype/employee/employee.py:305:deactivate_sales_person, erpnext/setup/doctype/employee/employee.py:313:create_user, erpnext/setup/doctype/transaction_deletion_record/transaction_deletion_record.py:378:generate_to_delete_list, erpnext/stock/doctype/batch/batch.py:308:split_batch, erpnext/stock/doctype/delivery_trip/delivery_trip.py:153:process_route, erpnext/stock/doctype/material_request/material_request.py:821:raise_work_orders, erpnext/stock/doctype/pick_list/pick_list.py:520:set_item_locations, erpnext/stock/doctype/stock_entry/stock_entry.py:3556:move_sample_to_retention_warehouse, erpnext/stock/doctype/stock_entry/stock_entry_utils.py:39:make_stock_entry, erpnext/stock/doctype/stock_reposting_settings/stock_reposting_settings.py:62:convert_to_item_wh_reposting, erpnext/stock/doctype/warehouse/warehouse.py:222:add_node, erpnext/stock/report/stock_and_account_value_comparison/stock_and_account_value_comparison.py:177:create_reposting_entries, erpnext/stock/report/stock_ledger_invariant_check/stock_ledger_invariant_check.py:299:create_reposting_entries, erpnext/support/doctype/issue/issue.py:121:split_issue, erpnext/support/doctype/issue/issue.py:275:make_issue_from_communication, erpnext/support/doctype/service_level_agreement/service_level_agreement.py:783:reset_service_level_agreement, erpnext/telephony/doctype/call_log/call_log.py:129:add_call_summary_and_call_type, erpnext/utilities/doctype/video/video.py:122:batch_update_youtube_data
- go-erpnext: whether any deployed environment actually includes nginx-go-routes.conf in the frontend image or uses docker-compose.override.yml - the local frappe_docker stack (/home/ubuntu/frappe_docker/pwd.yml) does not, so the Go service is not part of the running ERPNext today.
- go-erpnext: the orchestrator context listed pkg/reports, pkg/queries and pkg/dashboard; these directories do not exist on develop (966e70aebd) - confirm whether they live on an unmerged branch that should also be audited.
- go-erpnext: Frappe's `rounding_method` System Setting on the org's production sites (Banker's legacy vs Banker's vs Commercial) - determines which Python behaviour ERP-C-01 must match first.

## Appendix A - commands run and tool versions

```text
go version                      -> go1.22.2 linux/amd64
govulncheck -version            -> govulncheck@v1.8.0, DB https://vuln.go.dev
curl --version                  -> curl 7.81.0
docker compose version          -> v2.32.1
frappe / erpnext in container   -> 16.25.0 / 16.26.1

cd go-erpnext && go build ./cmd/server && go vet ./... && go test ./...
cd go-erpnext && govulncheck ./... > govulncheck.txt
go build -o /tmp/go-erpnext ./cmd/server && GO_ERPNEXT_PORT=8001 /tmp/go-erpnext
docker compose -f /home/ubuntu/frappe_docker/pwd.yml up -d
docker compose -f pwd.yml exec backend bench --site frontend install-app erpnext --force   # environment fix, not a repo change
curl probes: see transcripts below
python3 raw socket slowloris probe (partial header, 1 byte / 1.5 s, 12 s)
go run (temporary zz_audit_tmp/main.go calling pkg/frappe.Flt on 17 values; directory deleted afterwards, git status clean)
./env/bin/python -c "from frappe.utils.data import _bankers_rounding_legacy, _bankers_rounding, _round_away_from_zero ..."  (inside backend container)
git log -p -- go-erpnext/cmd/server/main.go ; grep -ril 'go-erpnext\|golang\|setup-go' .github/workflows
AST scan of erpnext/ for @frappe.whitelist(allow_guest=True) and write-without-permission-check heuristic (766 whitelisted, 6 allow_guest, 106 heuristic matches)
```

No application code, config or dependencies were modified. Temporary artefacts (built binary, probe scripts, transcript files) live outside the repo.

## Appendix B - Go service transcripts

```text
### curl -s -i http://localhost:8001/api/method/go_erpnext.ping
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 19

{"message":"pong"}


### curl -s -i -X POST http://localhost:8001/api/method/go_erpnext.ping -d '{"x":1}'
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 19

{"message":"pong"}


### curl -s -i -X DELETE http://localhost:8001/api/method/go_erpnext.ping
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 19

{"message":"pong"}


### curl -s -i -X OPTIONS http://localhost:8001/api/method/go_erpnext.ping -H 'Origin: https://evil.example'
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 19

{"message":"pong"}


### curl -s -i http://localhost:8001/api/method/go_erpnext.ping -H 'Origin: https://evil.example'
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 19

{"message":"pong"}


### curl -s -i http://localhost:8001/api/method/go_erpnext.ping/
HTTP/1.1 404 Not Found
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 42

{"error":"not implemented in Go service"}


### curl -s -i http://localhost:8001/api/method/erpnext.stock.valuation.get_fifo_rate
HTTP/1.1 404 Not Found
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 42

{"error":"not implemented in Go service"}


### curl -s -i http://localhost:8001/api/resource/Sales%20Invoice/ACC-SINV-0001
HTTP/1.1 404 Not Found
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 42

{"error":"not implemented in Go service"}


### curl -s -i 'http://localhost:8001/healthz'
HTTP/1.1 404 Not Found
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 42

{"error":"not implemented in Go service"}


### curl -s -i 'http://localhost:8001/../../etc/passwd'
HTTP/1.1 404 Not Found
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 42

{"error":"not implemented in Go service"}


### curl -s -i -X POST http://localhost:8001/api/method/go_erpnext.ping -H 'Content-Type: application/json' --data-binary 'not json {{{'
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 19

{"message":"pong"}


### head -c 20000000 /dev/zero | tr '\0' 'A' > big.bin; curl -s -o /dev/null -w 'oversized 20MB body -> HTTP %{http_code} sent=%{size_upload} time=%{time_total}s\n' -X POST http://localhost:8001/api/method/go_erpnext.ping --data-binary @big.bin
oversized 20MB body -> HTTP 200 sent=0 time=0.000463s


### curl -s -i http://localhost:8001/api/method/go_erpnext.ping -H 'X-Request-Id: abc-123' -H 'Authorization: token aaa:bbb'
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 19

{"message":"pong"}


### curl -s -i 'http://localhost:8001/api/method/go_erpnext.ping?x=%27%20OR%201=1'
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 19

{"message":"pong"}


### curl -s -i -H 'X-Forwarded-For: 1.2.3.4' 'http://localhost:8001/foo%0d%0aInjected:%20header'
HTTP/1.1 404 Not Found
Content-Type: application/json
Date: Tue, 06 Oct 2026 08:06:08 GMT
Content-Length: 42

{"error":"not implemented in Go service"}
```

## Appendix C - Frappe transcripts

```text
### curl -s -m 20 -w '\nHTTP %{http_code}\n' 'http://localhost:8080/api/resource/Customer?limit_page_length=2'
{"exc_type":"PermissionError","_server_messages":"[\"{\\\"message\\\":\\\"User <strong>Guest</strong> does not have doctype access via role permission for document <strong>DocType</strong>\\\",\\\"as_table\\\":false,\\\"title\\\":\\\"Message\\\"}\"]","_error_message":"No permission for DocType"}
HTTP 403


### curl -s -m 20 -w '\nHTTP %{http_code}\n' -X POST 'http://localhost:8080/api/method/erpnext.www.book_appointment.index.get_appointment_settings'
{"exc_type":"ValidationError","_server_messages":"[\"{\\\"message\\\":\\\"App erpnext is not installed\\\",\\\"as_table\\\":false,\\\"title\\\":\\\"Message\\\",\\\"indicator\\\":\\\"red\\\",\\\"raise_exception\\\":1,\\\"__frappe_exc_id\\\":\\\"8a0a9a45d7579eac0a32492cd7a0c618f8fbcbda7174848dd827d7c7\\\"}\",\"{\\\"message\\\":\\\"Failed to get method for command erpnext.www.book_appointment.index.get_appointment_settings with App erpnext is not installed\\\",\\\"as_table\\\":false,\\\"title\\\":\\\"Message\\\",\\\"indicator\\\":\\\"red\\\",\\\"raise_exception\\\":1,\\\"__frappe_exc_id\\\":\\\"6ae45fd3bef10d7e47d76333bd7425f8109cfa1821b62572d8ffabc9\\\"}\"]"}
HTTP 417


### curl -s -m 30 -w '\nHTTP %{http_code}\n' -X POST 'http://localhost:8080/api/method/erpnext.www.book_appointment.index.create_appointment' -H 'Content-Type: application/json' -d '{"date":"2026-10-20","time":"10:00:00","tz":"UTC","contact":"{\"name\":\"Audit Guest\",\"email\":\"audit-guest@example.com\",\"number\":\"123\"}"}'
{"exc_type":"ValidationError","_server_messages":"[\"{\\\"message\\\":\\\"App erpnext is not installed\\\",\\\"as_table\\\":false,\\\"title\\\":\\\"Message\\\",\\\"indicator\\\":\\\"red\\\",\\\"raise_exception\\\":1,\\\"__frappe_exc_id\\\":\\\"93549a76ea9afe1b8ca79f3be05cd17f6291bda3a104fb3033587488\\\"}\",\"{\\\"message\\\":\\\"Failed to get method for command erpnext.www.book_appointment.index.create_appointment with App erpnext is not installed\\\",\\\"as_table\\\":false,\\\"title\\\":\\\"Message\\\",\\\"indicator\\\":\\\"red\\\",\\\"raise_exception\\\":1,\\\"__frappe_exc_id\\\":\\\"aae377407fde21624a9726949bcdf25b65a2e909b37b45811f438b79\\\"}\"]"}
HTTP 417


### curl -s -m 30 -w '\nHTTP %{http_code}\n' -X POST 'http://localhost:8080/api/method/erpnext.templates.utils.send_message' -H 'Content-Type: application/json' -d '{"sender":"audit-guest2@example.com","message":"hello from audit","subject":"Audit probe"}'
{"exc_type":"ValidationError","_server_messages":"[\"{\\\"message\\\":\\\"App erpnext is not installed\\\",\\\"as_table\\\":false,\\\"title\\\":\\\"Message\\\",\\\"indicator\\\":\\\"red\\\",\\\"raise_exception\\\":1,\\\"__frappe_exc_id\\\":\\\"d920513db2e3d1234a04fb8e2377ea06cc05ec861c1ae4afe6e53d8c\\\"}\",\"{\\\"message\\\":\\\"Failed to get method for command erpnext.templates.utils.send_message with App erpnext is not installed\\\",\\\"as_table\\\":false,\\\"title\\\":\\\"Message\\\",\\\"indicator\\\":\\\"red\\\",\\\"raise_exception\\\":1,\\\"__frappe_exc_id\\\":\\\"b57dd066160bb1c7583d907c2fdf1afc323bc0368f0a168846e2c32a\\\"}\"]"}
HTTP 417

### curl -s -m 30 -L -w '\nHTTP %{http_code}\n' -X POST 'http://localhost:8080/api/method/erpnext.www.book_appointment.index.create_appointment' -H 'Accept: application/json' -H 'X-Requested-With: XMLHttpRequest' -H 'Content-Type: application/json' -d '{"date":"2026-10-20","time":"10:00:00","tz":"UTC","contact":"{\"name\":\"Audit Guest\",\"email\":\"audit-guest@example.com\",\"number\":\"123\"}"}' | sed 's/<[^>]*>//g' | grep -v '^\s*$' | head -20
{"exc_type":"Redirect"}
HTTP 301


### curl -s -m 30 -w '\nHTTP %{http_code}\n' -X POST 'http://localhost:8080/api/method/erpnext.templates.utils.send_message' -H 'Accept: application/json' -H 'Content-Type: application/json' -d '{"sender":"audit-guest3@example.com","message":"hello from audit 3","subject":"Audit probe 3"}'
{}
HTTP 200


### curl -s -m 30 -b cj.txt -w '\nHTTP %{http_code}\n' 'http://localhost:8080/api/resource/Lead?fields=%5B%22name%22,%22email_id%22,%22owner%22,%22creation%22%5D&limit_page_length=5'
{"data":[]}
HTTP 200


### curl -s -m 30 -b cj.txt -w '\nHTTP %{http_code}\n' 'http://localhost:8080/api/resource/Communication?fields=%5B%22name%22,%22subject%22,%22sender%22,%22reference_doctype%22,%22reference_name%22%5D&limit_page_length=5'
{"data":[]}
HTTP 200


### curl -s -m 30 -b cj.txt -w '\nHTTP %{http_code}\n' 'http://localhost:8080/api/resource/Opportunity?fields=%5B%22name%22,%22title%22,%22contact_email%22,%22owner%22%5D&limit_page_length=5'
{"data":[]}
HTTP 200
```

## Appendix D - govulncheck ./... (go1.22.2)

```text
=== Symbol Results ===

Vulnerability #1: GO-2026-6090
    Limit handshake messages we are willing to accept post-handshake in
    crypto/tls
  More info: https://pkg.go.dev/vuln/GO-2026-6090
  Standard library
    Found in: crypto/tls@go1.22.2
    Fixed in: crypto/tls@go1.25.13
    Example traces found:
      #1: pkg/db/db.go:75:23: db.New calls sql.Open, which eventually calls tls.Conn.Handshake
      #2: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls tls.Conn.HandshakeContext
      #3: cmd/server/main.go:70:15: server.main calls signal.Notify, which eventually calls tls.Conn.Read
      #4: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls tls.Conn.Write

Vulnerability #2: GO-2026-6089
    Apply ReadHeaderTimeout when doing unencrypted HTTP/2 check in net/http
  More info: https://pkg.go.dev/vuln/GO-2026-6089
  Standard library
    Found in: net/http@go1.22.2
    Fixed in: net/http@go1.25.13
    Example traces found:
      #1: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe

Vulnerability #3: GO-2026-5972
    Enforce maximum recursion depth in encoding/asn1
  More info: https://pkg.go.dev/vuln/GO-2026-5972
  Standard library
    Found in: encoding/asn1@go1.22.2
    Fixed in: encoding/asn1@go1.25.13
    Example traces found:
      #1: pkg/db/db.go:75:23: db.New calls sql.Open, which eventually calls asn1.Unmarshal

Vulnerability #4: GO-2026-5856
    Invoking Encrypted Client Hello privacy leak in crypto/tls
  More info: https://pkg.go.dev/vuln/GO-2026-5856
  Standard library
    Found in: crypto/tls@go1.22.2
    Fixed in: crypto/tls@go1.25.12
    Example traces found:
      #1: pkg/db/db.go:75:23: db.New calls sql.Open, which eventually calls tls.Conn.Handshake
      #2: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls tls.Conn.HandshakeContext
      #3: cmd/server/main.go:70:15: server.main calls signal.Notify, which eventually calls tls.Conn.Read
      #4: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls tls.Conn.Write

Vulnerability #5: GO-2026-5039
    Arbitrary inputs are included in errors without any escaping in
    net/textproto
  More info: https://pkg.go.dev/vuln/GO-2026-5039
  Standard library
    Found in: net/textproto@go1.22.2
    Fixed in: net/textproto@go1.25.11
    Example traces found:
      #1: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls textproto.Reader.ReadMIMEHeader

Vulnerability #6: GO-2026-5037
    Inefficient candidate hostname parsing in crypto/x509
  More info: https://pkg.go.dev/vuln/GO-2026-5037
  Standard library
    Found in: crypto/x509@go1.22.2
    Fixed in: crypto/x509@go1.25.11
    Example traces found:
      #1: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls x509.Certificate.Verify
      #2: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls x509.Certificate.VerifyHostname
      #3: pkg/db/db.go:187:33: db.DB.GetList calls fmt.Sprintf, which eventually calls x509.HostnameError.Error

Vulnerability #7: GO-2026-4971
    Panic in Dial and LookupPort when handling NUL byte on Windows in net
  More info: https://pkg.go.dev/vuln/GO-2026-4971
  Standard library
    Found in: net@go1.22.2
    Fixed in: net@go1.25.10
    Example traces found:
      #1: pkg/db/db.go:75:23: db.New calls sql.Open, which eventually calls net.Dialer.DialContext
      #2: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which calls net.Listen

Vulnerability #8: GO-2026-4947
    Unexpected work during chain building in crypto/x509
  More info: https://pkg.go.dev/vuln/GO-2026-4947
  Standard library
    Found in: crypto/x509@go1.22.2
    Fixed in: crypto/x509@go1.25.9
    Example traces found:
      #1: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls x509.Certificate.Verify

Vulnerability #9: GO-2026-4946
    Inefficient policy validation in crypto/x509
  More info: https://pkg.go.dev/vuln/GO-2026-4946
  Standard library
    Found in: crypto/x509@go1.22.2
    Fixed in: crypto/x509@go1.25.9
    Example traces found:
      #1: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls x509.Certificate.Verify

Vulnerability #10: GO-2026-4870
    Unauthenticated TLS 1.3 KeyUpdate record can cause persistent connection
    retention and DoS in crypto/tls
  More info: https://pkg.go.dev/vuln/GO-2026-4870
  Standard library
    Found in: crypto/tls@go1.22.2
    Fixed in: crypto/tls@go1.25.9
    Example traces found:
      #1: pkg/db/db.go:75:23: db.New calls sql.Open, which eventually calls tls.Conn.Handshake
      #2: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls tls.Conn.HandshakeContext
      #3: cmd/server/main.go:70:15: server.main calls signal.Notify, which eventually calls tls.Conn.Read
      #4: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls tls.Conn.Write

Vulnerability #11: GO-2026-4602
    FileInfo can escape from a Root in os
  More info: https://pkg.go.dev/vuln/GO-2026-4602
  Standard library
    Found in: os@go1.22.2
    Fixed in: os@go1.25.8
    Example traces found:
      #1: cmd/server/main.go:70:15: server.main calls signal.Notify, which eventually calls os.ReadDir

Vulnerability #12: GO-2026-4601
    Incorrect parsing of IPv6 host literals in net/url
  More info: https://pkg.go.dev/vuln/GO-2026-4601
  Standard library
    Found in: net/url@go1.22.2
    Fixed in: net/url@go1.25.8
    Example traces found:
      #1: cmd/server/main.go:70:15: server.main calls signal.Notify, which eventually calls url.Parse
      #2: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls url.ParseRequestURI

Vulnerability #13: GO-2026-4340
    Handshake messages may be processed at the incorrect encryption level in
    crypto/tls
  More info: https://pkg.go.dev/vuln/GO-2026-4340
  Standard library
    Found in: crypto/tls@go1.22.2
    Fixed in: crypto/tls@go1.24.12
    Example traces found:
      #1: pkg/db/db.go:75:23: db.New calls sql.Open, which eventually calls tls.Conn.Handshake
      #2: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls tls.Conn.HandshakeContext
      #3: cmd/server/main.go:70:15: server.main calls signal.Notify, which eventually calls tls.Conn.Read
      #4: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls tls.Conn.Write

Vulnerability #14: GO-2026-4337
    Unexpected session resumption in crypto/tls
  More info: https://pkg.go.dev/vuln/GO-2026-4337
  Standard library
    Found in: crypto/tls@go1.22.2
    Fixed in: crypto/tls@go1.24.13
    Example traces found:
      #1: pkg/db/db.go:75:23: db.New calls sql.Open, which eventually calls tls.Conn.Handshake
      #2: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls tls.Conn.HandshakeContext
      #3: cmd/server/main.go:70:15: server.main calls signal.Notify, which eventually calls tls.Conn.Read
      #4: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls tls.Conn.Write

Vulnerability #15: GO-2025-4175
    Improper application of excluded DNS name constraints when verifying
    wildcard names in crypto/x509
  More info: https://pkg.go.dev/vuln/GO-2025-4175
  Standard library
    Found in: crypto/x509@go1.22.2
    Fixed in: crypto/x509@go1.24.11
    Example traces found:
      #1: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls x509.Certificate.Verify

Vulnerability #16: GO-2025-4155
    Excessive resource consumption when printing error string for host
    certificate validation in crypto/x509
  More info: https://pkg.go.dev/vuln/GO-2025-4155
  Standard library
    Found in: crypto/x509@go1.22.2
    Fixed in: crypto/x509@go1.24.11
    Example traces found:
      #1: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls x509.Certificate.Verify
      #2: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls x509.Certificate.VerifyHostname

Vulnerability #17: GO-2025-4013
    Panic when validating certificates with DSA public keys in crypto/x509
  More info: https://pkg.go.dev/vuln/GO-2025-4013
  Standard library
    Found in: crypto/x509@go1.22.2
    Fixed in: crypto/x509@go1.24.8
    Example traces found:
      #1: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls x509.Certificate.Verify

Vulnerability #18: GO-2025-4011
    Parsing DER payload can cause memory exhaustion in encoding/asn1
  More info: https://pkg.go.dev/vuln/GO-2025-4011
  Standard library
    Found in: encoding/asn1@go1.22.2
    Fixed in: encoding/asn1@go1.24.8
    Example traces found:
      #1: pkg/db/db.go:75:23: db.New calls sql.Open, which eventually calls asn1.Unmarshal

Vulnerability #19: GO-2025-4010
    Insufficient validation of bracketed IPv6 hostnames in net/url
  More info: https://pkg.go.dev/vuln/GO-2025-4010
  Standard library
    Found in: net/url@go1.22.2
    Fixed in: net/url@go1.24.8
    Example traces found:
      #1: cmd/server/main.go:70:15: server.main calls signal.Notify, which eventually calls url.Parse
      #2: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls url.ParseRequestURI

Vulnerability #20: GO-2025-4009
    Quadratic complexity when parsing some invalid inputs in encoding/pem
  More info: https://pkg.go.dev/vuln/GO-2025-4009
  Standard library
    Found in: encoding/pem@go1.22.2
    Fixed in: encoding/pem@go1.24.8
    Example traces found:
      #1: pkg/db/db.go:75:23: db.New calls sql.Open, which eventually calls pem.Decode

Vulnerability #21: GO-2025-4008
    ALPN negotiation error contains attacker controlled information in
    crypto/tls
  More info: https://pkg.go.dev/vuln/GO-2025-4008
  Standard library
    Found in: crypto/tls@go1.22.2
    Fixed in: crypto/tls@go1.24.8
    Example traces found:
      #1: pkg/db/db.go:75:23: db.New calls sql.Open, which eventually calls tls.Conn.Handshake
      #2: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls tls.Conn.HandshakeContext
      #3: cmd/server/main.go:70:15: server.main calls signal.Notify, which eventually calls tls.Conn.Read
      #4: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls tls.Conn.Write

Vulnerability #22: GO-2025-4007
    Quadratic complexity when checking name constraints in crypto/x509
  More info: https://pkg.go.dev/vuln/GO-2025-4007
  Standard library
    Found in: crypto/x509@go1.22.2
    Fixed in: crypto/x509@go1.24.9
    Example traces found:
      #1: cmd/server/main.go:70:15: server.main calls signal.Notify, which eventually calls x509.CertPool.AppendCertsFromPEM
      #2: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls x509.Certificate.Verify
      #3: cmd/server/main.go:70:15: server.main calls signal.Notify, which eventually calls x509.ParseCertificate
      #4: pkg/db/db.go:75:23: db.New calls sql.Open, which eventually calls x509.ParsePKIXPublicKey

Vulnerability #23: GO-2025-3849
    Incorrect results returned from Rows.Scan in database/sql
  More info: https://pkg.go.dev/vuln/GO-2025-3849
  Standard library
    Found in: database/sql@go1.22.2
    Fixed in: database/sql@go1.23.12
    Example traces found:
      #1: pkg/db/db.go:155:42: db.DB.GetValue calls sql.Row.Scan
      #2: pkg/db/db.go:234:22: db.DB.GetList calls sql.Rows.Scan

Vulnerability #24: GO-2025-3750
    Inconsistent handling of O_CREATE|O_EXCL on Unix and Windows in os in
    syscall
  More info: https://pkg.go.dev/vuln/GO-2025-3750
  Standard library
    Found in: os@go1.22.2
    Fixed in: os@go1.23.10
    Platforms: windows
    Example traces found:
      #1: pkg/testutil/testutil.go:54:22: testutil.LoadFixturesFromTestdata calls os.Getwd
      #2: pkg/testutil/testutil.go:7:2: testutil.init calls os.init, which calls os.NewFile
      #3: cmd/server/main.go:70:15: server.main calls signal.Notify, which eventually calls os.Open
      #4: cmd/server/main.go:70:15: server.main calls signal.Notify, which eventually calls os.ReadDir
      #5: pkg/db/db.go:48:26: db.ReadSiteConfig calls os.ReadFile
      #6: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls os.Remove
      #7: pkg/testutil/testutil.go:61:23: testutil.LoadFixturesFromTestdata calls os.Stat
      #8: pkg/testutil/testutil.go:54:22: testutil.LoadFixturesFromTestdata calls os.Getwd, which eventually calls syscall.Open

Vulnerability #25: GO-2025-3563
    Request smuggling due to acceptance of invalid chunked data in net/http
  More info: https://pkg.go.dev/vuln/GO-2025-3563
  Standard library
    Found in: net/http/internal@go1.22.2
    Fixed in: net/http/internal@go1.23.8
    Example traces found:
      #1: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls internal.chunkedReader.Read

Vulnerability #26: GO-2025-3447
    Timing sidechannel for P-256 on ppc64le in crypto/internal/nistec
  More info: https://pkg.go.dev/vuln/GO-2025-3447
  Standard library
    Found in: crypto/internal/nistec@go1.22.2
    Fixed in: crypto/internal/nistec@go1.22.12
    Platforms: ppc64le
    Example traces found:
      #1: cmd/server/main.go:70:15: server.main calls signal.Notify, which eventually calls nistec.P256Point.ScalarBaseMult
      #2: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls nistec.P256Point.ScalarMult
      #3: cmd/server/main.go:70:15: server.main calls signal.Notify, which eventually calls nistec.P256Point.SetBytes

Vulnerability #27: GO-2025-3373
    Usage of IPv6 zone IDs can bypass URI name constraints in crypto/x509
  More info: https://pkg.go.dev/vuln/GO-2025-3373
  Standard library
    Found in: crypto/x509@go1.22.2
    Fixed in: crypto/x509@go1.22.11
    Example traces found:
      #1: cmd/server/main.go:70:15: server.main calls signal.Notify, which eventually calls x509.CertPool.AppendCertsFromPEM
      #2: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls x509.Certificate.Verify
      #3: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls x509.Certificate.VerifyHostname
      #4: pkg/db/db.go:187:33: db.DB.GetList calls fmt.Sprintf, which eventually calls x509.HostnameError.Error
      #5: cmd/server/main.go:70:15: server.main calls signal.Notify, which eventually calls x509.ParseCertificate
      #6: pkg/db/db.go:75:23: db.New calls sql.Open, which eventually calls x509.ParsePKIXPublicKey

Vulnerability #28: GO-2024-2887
    Unexpected behavior from Is methods for IPv4-mapped IPv6 addresses in
    net/netip
  More info: https://pkg.go.dev/vuln/GO-2024-2887
  Standard library
    Found in: net/netip@go1.22.2
    Fixed in: net/netip@go1.22.4
    Example traces found:
      #1: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls netip.Addr.IsLoopback
      #2: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which eventually calls netip.Addr.IsMulticast

Vulnerability #29: GO-2024-2824
    Malformed DNS message can cause infinite loop in net
  More info: https://pkg.go.dev/vuln/GO-2024-2824
  Standard library
    Found in: net@go1.22.2
    Fixed in: net@go1.22.3
    Example traces found:
      #1: pkg/db/db.go:75:23: db.New calls sql.Open, which eventually calls net.Dialer.DialContext
      #2: cmd/server/main.go:74:31: server.main calls http.Server.ListenAndServe, which calls net.Listen

Your code is affected by 29 vulnerabilities from the Go standard library.
This scan also found 16 vulnerabilities in packages you import and 18
vulnerabilities in modules you require, but your code doesn't appear to call
these vulnerabilities.
Use '-show verbose' for more details.
```
