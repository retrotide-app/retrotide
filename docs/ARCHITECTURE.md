# Architecture

## 1. Scope

RetroTide is a sprint retrospective tool. A team runs a board of columns and cards, its members write cards, vote on them and discuss them, and the actions they agree carry into the team's next retro. People belong to organisations, and organisations hold teams and boards. A board belongs to its organisation, and a team is optional: it only shares the board with that group, and a board with no team is open to every member. Guests join a board from a link, without an account. RetroTide does not try to match every feature of the larger retro tools, and it has no AI features of any kind: no AI summaries, no AI grouping of cards, no generated templates and no AI chat.

This document is for a developer joining the project. It explains how the code is laid out and why: the layers, the one rule at each boundary, and the subsystems that cross them. It is not a feature list, an operations guide or a catalogue of edge cases. The exact schema lives in the migrations, the wire shapes in `internal/dto`, and the edge cases in the tests; this document describes the shape they all live in.

The code is built in milestones. What exists today is M0, the backend skeleton: configuration, site settings, logging, telemetry, health checks, the database and the core of the data layer, the cache, the controller base, the whole Gin middleware chain, the architecture tests, the startup banner and the Docker files. Everything else is designed but not yet written, and wherever this document describes it, it names the milestone that brings it.

| ID   | Brings                                            | ID   | Brings                                                |
|------|---------------------------------------------------|------|-------------------------------------------------------|
| M0   | Backend skeleton, which is what exists today      | M9   | Facilitator controls, reveal, anonymity and the timer |
| M1   | Frontend skeleton                                 | M10  | Action items and carry-over                           |
| M2   | CI, image publishing, deploy, backups and staging | M11  | Exports and the board summary                         |
| M3   | Accounts, email login and Google sign-in          | M12  | Marketing site, templates and legal pages             |
| M4   | Organisations, teams and invites                  | M12a | Facilitation flow and presentation mode               |
| M5   | Boards, columns and cards                         | M12b | Card polish, images, board management and surveys     |
| M6   | Realtime board sync                               | M12c | Templates, Microsoft sign-in and two-factor           |
| M7   | Guests and share links                            | M13  | Hardening and the public free launch                  |
| M8   | Votes, comments, merging, sort and search         | M14  | Slack, Teams and outgoing webhooks                    |
|      |                                                   | M15  | Team insights                                         |
|      |                                                   | M16  | Billing and the paid Team plan                        |

One idea is worth holding before reading further, because it shapes more of the backend than any other: **cards are hidden by default, and hidden means absent.** Every new board opens in a writing phase in which each participant sees their own cards in full and everyone else's as blurred placeholders. The server never sends another participant's hidden card, only a count per column, so the blur is presentation over an empty card and there is nothing behind it to uncover. When the facilitator, who hosts the board, moves it from Write into its Vote stage, the cards are revealed and every card appears for everyone at once. No role can read a hidden card before then: not the facilitator, not a team lead, not the organisation's owner. Section 6.5 explains how.

## 2. System shape

The server is one Go binary. It embeds the compiled Vite bundle and serves the SPA, the JSON API and the WebSocket from a single process, which runs as one replica on a dedicated server behind Cloudflare and a Caddy reverse proxy. Board commands arrive over the JSON API, and each committed change is announced over the WebSocket to everyone on that board, in the form each of them is allowed to see.

Every part has one job. Controllers parse HTTP, services hold the rules, repositories own transactions and the cache seam, and DAOs own the SQL. The hub owns the sockets, the live board service owns the order in which a board changes and the order in which those changes are announced, and the projection package owns what each viewer may see.

### 2.1 Component map

```
   ┌──────────────────────────────┐   ┌──────────────────────────────┐
   │  Signed-in user (React 19)   │   │  Guest from a board link     │
   │  rt_session cookie + socket  │   │  rt_guest cookie + socket    │
   └──────────────┬───────────────┘   └──────────────┬───────────────┘
                  │        HTTPS and WSS             │
                  └────────────────┬─────────────────┘
                                   ▼
                  Cloudflare (TLS, WAF) → Caddy on the host
                                   │  127.0.0.1:2412 → container :4323
                                   ▼
   ┌──────────────────────────────────────────────────────────────────┐
   │                      Gin engine on net/http                      │
   │    one global middleware chain, then guards attached per route   │
   └─────────┬─────────────────────────────────────────┬──────────────┘
             ▼                                         ▼
   ┌──────────────────┐                      ┌──────────────────────┐
   │   Controllers    │                      │    WebSocket hub     │
   │  HTTP in, spec   │                      │ sockets by principal,│
   │  out, no rules   │                      │ groups by board ID   │
   └─────────┬────────┘                      └──────────▲───────────┘
             ▼                                          │ projected events,
   ┌──────────────────┐  board gate, version, deliver   │ in version order
   │     Services     │─────────────────────────────────┘
   │ board, org, team,│──────────▶ Google (sign-in, M3)
   │ identity,        │──────────▶ Microsoft (sign-in, M12c)
   │ billing          │──────────▶ Stripe (billing, M16)
   └─────────┬────────┘
             ▼
   ┌──────────────────┐   ┌────────────────┐
   │   Repositories   │──▶│ internal/cache │
   │ (composites +    │◀──│ Valkey, then   │
   │  cache seam)     │   │ in-memory LRU  │
   └─────────┬────────┘   └────────────────┘
             ▼
   ┌──────────────────┐   ┌────────────────┐
   │       DAOs       │──▶│ PostgreSQL 18  │
   │ (all SQL, sqlc)  │   │                │
   └──────────────────┘   └────────────────┘
```

Payment pages and the providers' sign-in pages never pass through RetroTide. The browser is redirected to Stripe, Google or Microsoft and back again. The only traffic between the Go process and those services is each provider's discovery document, signing keys and code exchange, the Stripe API calls, and Stripe's signed webhooks coming in.

### 2.2 One process, on purpose

RetroTide is designed for exactly one replica. Several things live in process memory and are correct only because there is one process: the hub's registry of sockets and the boards they watch; each board's gate, roster and outbox, which fix the order of its changes and of their announcement (section 6.5); the 10-second tick that holds back hidden-card counts on anonymous boards; the rate limiters' token buckets; and the settings listeners that hot-reload configuration.

The alternative, fan-out over Valkey pub/sub and ordering through Postgres advisory locks, pays in complexity for a second replica the product does not need. One process on a dedicated machine is expected to hold thousands of board sockets. That is a target, not a measurement, and the load tests in section 8 check it before launch.

The price is 10 to 30 seconds of "reconnecting" on each deploy, while the new container starts. The shutdown order (section 3.10) makes sure nothing is lost in that window: every committed change is announced before the sockets close, clients reconnect and ask whether their board changed, and a command that was in flight is retried with its original operation ID, so it is never applied twice. A second replica would be a rewrite of the realtime design, not a configuration change.

## 3. Backend

### 3.1 The layers at a glance

There are four layers, and calls go one way. A request enters at the top, and each layer hands work only to the layer directly beneath it.

```
   HTTP request
        │
        ▼
 ┌────────────────────────────────────────────────────────────────────────┐
 │ controller   internal/controllers                                      │
 │              parses the request into a spec (an input struct), reads   │
 │              the principal, calls one service method, maps errors.     │
 │              Owns no rule.                                             │
 ├────────────────────────────────────────────────────────────────────────┤
 │ service      internal/<domain>                                         │
 │              the rules: access, board policy, plan limits, the board   │
 │              gate, projection per viewer, notification fan-out.        │
 ├────────────────────────────────────────────────────────────────────────┤
 │ repository   internal/repository                                       │
 │              composites that open one transaction across several DAO   │
 │              calls, and the cache seam. Embeds the DAO interface.      │
 ├────────────────────────────────────────────────────────────────────────┤
 │ dao          internal/dao                                              │
 │              every SQL statement: one file per aggregate, one          │
 │              statement per method, one table written per method.       │
 ├────────────────────────────────────────────────────────────────────────┤
 │ model        internal/model, model/spec, dto and audit, plus the pure  │
 │              board/projection, board/policy and fracindex packages.    │
 │              They import nothing above them.                           │
 ├────────────────────────────────────────────────────────────────────────┤
 │ PostgreSQL   native UUID, CITEXT, TIMESTAMPTZ, composite foreign keys  │
 └────────────────────────────────────────────────────────────────────────┘
```

In M0 one domain runs from service to database: site settings. `settings.Service` holds the rules (validation, listeners, defaults), `SettingsRepository` adds the cache, `SettingsDAO` holds the SQL, and `store.New` wires them together. It is small, but it is the pattern every later domain copies. Its admin routes arrive in M3, and the health routes show the controller end today.

**A controller never reaches a repository.** The controller surface, `controllers.Service`, holds services, the guards, the session manager and the sign-in flow's cookie builder (only to read and write their cookies) and the embedded files, and later the hub's upgrade handler (M6) and the OG resolver (M12). A controller may name a DAO sentinel error so that it can map it to a status code, and nothing more.

**A service never writes SQL and never opens a transaction.** No non-test file outside `internal/dao` and `internal/db` contains a `SELECT`, `INSERT INTO`, `UPDATE ... SET` or `DELETE FROM`, and `db.WithTx` and `db.WithReadTx` appear only under `internal/repository`, `internal/dao` and `internal/db`. The arch tests check both.

**`*gin.Context` never leaves the HTTP layer.** Only `internal/controllers`, `internal/middleware`, `internal/routes`, `internal/utils` (which builds the engine) and the root package import Gin. A controller passes `c.Request.Context()` down, never `c`, so from the service down a request is only a `context.Context` carrying the trace span, the principal and the logger's fields. That is what lets a background job or the webhook worker use the lower layers exactly as a request does. The hub's upgrade handler is a plain `http.Handler`, and a streamed export receives an `io.Writer`, so neither learns about Gin either.

**Writes finish even when the client leaves.** The `detach` middleware runs every request except GET and HEAD on `context.WithoutCancel`, so a command whose client disconnects still commits, invalidates its cache entries and announces its events, while a read stops as soon as its client goes away. Work that must outlive a GET copies the values it needs and runs on its own detached context. It never captures `c`, because Gin reuses the context once the handler returns.

**Board state is written only under the board's gate (M5).** Only `internal/board` and its sub-packages may call the repository methods that write a board's rows, and they always hold that board's gate and bump its version. Work that starts elsewhere, such as removing an organisation member or deleting an account, hands the board write to the board service instead.

**Board payloads are built in exactly one package (M5).** Every snapshot, event and command response for a board is constructed in `internal/board/projection`, the single place that decides what a viewer may see.

**The wiring runs the other way.** `internal/store/new.go` is the only place the object graph is assembled: `store.New(db, cache)` builds every DAO, wraps each one in its repository, and hands the cache manager only to the repositories that cache. Construction runs bottom-up, calls run top-down, and neither goes through a DI container.

### 3.2 Controllers

A controller owns the shape of the HTTP request and nothing else. It parses path and query parameters, answering 400 to a malformed ID rather than falling back to a default. It binds the JSON body, reads the principal, builds one spec (an input struct from `internal/model/spec`), calls one service method with `c.Request.Context()`, and maps sentinel errors to status codes. Whether voting is open, or whether the organisation's plan allows another team, is decided one layer down.

**Routes are data.** Each controller file returns a list of `FRoute` values: a method, a path, the guards that run first, and the handler. `GetAPIRoutes` collects the lists mounted under `/api/v1` and `GetPageRoutes` the lists mounted at the root, and `RegisterRoutes` in `f_route.go` is the only code that registers a route. It also registers HEAD beside every GET, which Gin does not do by itself. The health routes show the shape:

```go
{Method: http.MethodGet, Path: "/health", Before: gin.HandlersChain{s.Guards.MetricsToken}, Handler: s.health},
```

Every endpoint and its access rule therefore sit on one line, in the file that owns the handler, and no route needs a setup method whose whole body is a single call. Production and every controller test mount the lists through the same function, and a route-table test mounts all of them on one engine, so a registration conflict, which Gin reports by panicking, fails CI rather than boot.

**Guards attach per route.** `middleware.Guards` is built once and handed to the controllers. Since M3 it holds `MetricsToken`, `Auth`, `OptionalAuth`, `Turnstile`, the staff `Permission` guard (which authenticates on its own, so a staff route names only it) and the rate limiters. A signed-in user passes the ban gate and, on writes, the lock gate; there is no email-verification gate, because an account with an unverified address never holds a session (section 6.3). M4 adds `OrgPermission` and `TeamPermission`, M7 adds `Principal` and `OptionalPrincipal` for guests, and every board command carries `BoardWriteLimit`, a token bucket per principal. An organisation or team check answers 404 to a non-member, so organisations cannot be enumerated, and 403 to a member without the permission. Board access is not a route guard: it mixes membership, link access, board settings and lock state, and the WebSocket join needs exactly the same check, so it lives in the live board service (section 6.4).

**Every rejection aborts.** Gin's `c.Next()` is a loop over the handler chain, so a guard that wrote a 401 and simply returned would still let the handler run and append a second body. A middleware or guard that refuses a request therefore always answers with `c.AbortWithStatusJSON`, and an arch test fails any middleware that answers some other way.

**Every error leaves as JSON, and every cause is logged once.** Handlers return nothing. An expected failure is answered in place, and every error body has one shape, an error message and an optional code. An unexpected one goes to `utils.InternalError`, which records the cause with `c.Error` and answers a generic 500. `ErrorResponder`, installed just inside `Recover`, logs each recorded cause once with the trace ID and the route template, and writes a JSON body for any error status nothing answered, such as Gin's own 404 and 405. Wrapped database text never reaches a response, and the frontend has one error shape to parse.

**The body is read once.** `utils.BindJSON` reads through `ShouldBindBodyWithJSON`, which caches the bytes, so a guard that peeks at the body first (Turnstile, or a limiter keyed on the email) leaves it intact for the handler. An arch test bans the Gin binders that would read it a second time.

### 3.3 Services

The service holds the rules: who may act on this board, whether its settings allow the action, whether the organisation's plan allows it, what each viewer may see of the result, and who is notified.

**Dependencies arrive as interfaces through a constructor**, and the constructor returns the service's interface, never its struct. A service test builds every dependency as a strict mockery mock, so the rules run in milliseconds without a database. Mocks are generated from `.mockery.yml`, never written by hand, so a mock that drifts from its interface is a compile error rather than a test that quietly passes.

**Services open no transactions and write no SQL.** A service that needs several writes to succeed or fail together asks its repository for a composite method (section 3.6). What a service may hold is process state with no table behind it, kept private behind methods: the live board service's gates, rosters and outboxes, or the entitlements service's catalogue of plans.

**Rules are checked in Go, and the database backs them up.** While a board's gate is held nothing else can write that board, so the state the service reads is the state it writes against, and it checks every rule in plain Go: voting is open, the card is visible to this voter, the board is not locked. The repository's statements are then conditional as a second line of defence. The vote budget only spends while `used < max`, and the version bump only succeeds on the version the service read, so a code path that skipped a check still cannot break the rule.

### 3.4 Repositories

`internal/repository` owns the composites and the cache seam, not the queries. A repository interface embeds its DAO interface and adds only what cannot be a single statement. The settings repository, which exists today, is the whole pattern:

```go
SettingsRepository interface {
	dao.SettingsDAO

	Reconcile(ctx context.Context, s spec.SettingsReconcile, tx ...*sql.Tx) error
}
```

Because the struct embeds the DAO interface, Go promotes every plain read and write onto the repository with no forwarding code. A method written on the repository shadows the promoted one, which is how caching is added (section 3.7). Inside a composite the DAO call is always qualified, as in `r.SettingsDAO.Get(...)`, so a shadowing method never calls itself.

A repository takes `*sql.DB` only when it owns a transaction, takes the cache manager only when it caches, and may compose other repositories. `dao.NewSettings` returns `dao.SettingsDAO`, not a repository interface, so a DAO cannot be handed to a service, and an orchestration method written on a DAO by mistake satisfies no interface. The compiler enforces the split, not review.

### 3.5 DAOs

Every SQL statement lives under `internal/dao`, in one file per aggregate: `settings.go` today, and `user.go`, `organisation.go`, `board.go`, `card.go`, `card_vote.go` and the rest as the milestones land. A method is one logical statement, and it writes one table. A read may join other tables for display columns, but a write never touches a second table, which is what makes the transaction rule in section 3.6 enforceable.

**The SQL is generated, not typed by hand.** Queries are plain `.sql` files in `internal/dao/queries`. sqlc reads them together with the goose migrations and generates typed Go into `internal/dao/sqlcgen`, which is committed. A DAO method builds the generated params, calls `sqlcgen.New(db.TxOrDB(r.db, tx))`, and maps the generated row onto a `model` type by hand. Generated types never leave the DAO body, so a reordered column or a codegen change cannot ripple into a service, and a renamed column breaks `sqlc generate` with the query named rather than breaking a request in production.

**A conditional statement that matches nothing returns a named sentinel** from `internal/dao/errors.go`: `ErrNotFound` today, and later `ErrVersionConflict`, `ErrDuplicateOp`, `ErrVoteBudgetSpent`, `ErrQuotaExceeded` and a few more. Services turn them into domain errors, and controllers map those to status codes.

**Tenancy is checked at build time as well as by tests.** `sqlc.yaml` declares two `sqlc vet` rules: any read, update or delete of a board child table must filter by `board_id`, and any on an organisation table must filter by `org_id`. They already run on every regeneration, before any tenant table exists. The few legitimately global queries, such as resolving a tenant from an ID in the path, looking up a share link by its slug, and the retention sweeps, carry a vet disable annotation and sit on an allowlist in the tenancy arch test (M4), each with its reason. The vet rules are tripwires; the guarantee is the cross-tenant tests of section 8.

**One generic query family, the claim queue, arrives with M14.** Its shape (claim a row with `FOR UPDATE SKIP LOCKED`, count the attempts, dead-letter) is written once as a template under `internal/dao/queries/templates` and expanded for webhook deliveries (M14) and billing events (M16). There is no dynamic SQL: every statement RetroTide needs can be written at build time.

**DAO tests run against a real database.** Each test binary boots one `postgres:18` container through testcontainers, and each test gets its own database, cloned from a migrated template. Checking the SQL once, for real, is what lets every layer above be tested against mocks.

### 3.6 Transactions, and who may open one

**Every DAO and repository method takes a trailing `tx ...*sql.Tx`.** A caller holding a transaction passes it, and a caller without one passes nothing. `db.TxOrDB` returns the transaction when there is one and the pool otherwise, so no statement needs two versions of itself. **Every method also takes at most one argument besides `ctx` and `tx`;** two or more mean a spec struct in `internal/model/spec`.

**A repository that needs several writes to be atomic opens the transaction** with `db.WithTx`. It joins an inbound transaction rather than nesting one, because `database/sql` has no savepoints, and otherwise begins, defers a rollback and commits. A composite can therefore run on its own or as one step of a larger unit of work.

**`db.WithReadTx` gives a snapshot read and never joins.** It opens a read-only REPEATABLE READ transaction and refuses an inbound one, because joining a READ COMMITTED transaction would silently lose the snapshot. The board snapshot uses it, so its version and its rows come from the same moment.

**A DAO may open a transaction only to batch statements against the one table it writes**, as `settingsDAO.SetMultiple` does for `site_settings`. Anything that touches a second table moves up to a repository composite named after the unit of work.

**Every board write begins the same way (M5).** Its composite first bumps `boards.version`, which succeeds only on the version the service read under the gate, and then records the command's `op_id`, so a duplicate rolls back along with everything else. The row lock the bump takes also serialises every write to the board inside Postgres, a second line of defence behind the gate.

```
  service: AddVote(cmd), with the board's gate held
     │ resolve access and board state in one read, then check the board policy
     │
     └──▶ repository: AddVote(ctx, spec) runs db.WithTx(ctx, r.db, tx, func(tx) error {
             ├──▶ boards.BumpVersion    ──▶ UPDATE boards, version = expected
             ├──▶ boards.RecordOp       ──▶ INSERT INTO board_ops
             ├──▶ votes.SpendBudget     ──▶ UPDATE board_vote_budgets, used < max
             ├──▶ votes.IncrementVote   ──▶ INSERT INTO card_votes ON CONFLICT
             └──▶ cards.AddVoteTotal    ──▶ UPDATE cards SET vote_total
          }): commit, or roll all five back, the version bump included
     │
     ▼
  service: project per viewer class, append to the board's outbox, release the gate
```

No service and no controller opens a transaction anywhere in the tree.

### 3.7 The cache seam

`internal/cache` exposes one `*cache.Manager` over an ordered list of engines: Valkey first, then a byte-capped in-memory LRU that is always enabled. Its typed surface is four generic methods, `m.Get[T]`, `m.Set[T]`, `m.SetMany[T]` and `m.Load[T]`, and a nil manager is a valid receiver on all of them, which is why a DAO test can pass `nil`. Every namespace and its TTL is declared in `internal/cache/keys.go`, never at a call site. How the engines are chosen and switched is section 6.2.

**Interception sits in the repository, as a method that shadows the promoted one.** A cached getter wraps its DAO call in `r.cache.Load(ctx, ns, load, keyParts...)`, and a write calls the DAO and then updates or deletes the key. A method that is not cached costs nothing and is not written at all. The repository is the right place because it is the one narrow point that every read and every write of a table already passes through: a cached read and the write that invalidates it sit in the same file, each table has one writer, and the service above cannot even tell whether a cache is there.

**The seam has three rules:**

- **Anything that can grant access or money has a finite TTL.** Memberships, the team header and staff roles live for a minute, and subscriptions and entitlement overrides for five. A failed invalidation therefore heals itself within that window instead of lasting until a restart. Site settings are the one exception, below.
- **Invalidation runs after commit, from whoever opened the transaction.** A `Del` issued inside someone else's uncommitted transaction would let a concurrent reader put the old value straight back. The code enforces this: the settings repository's writes refuse an inbound transaction with `db.ErrInboundTx`, so they are always the outermost unit of work.
- **Board state is never cached.** Cards, votes, participants and board settings are read under the gate or inside the snapshot transaction, every time. Even the board header stays uncached: it is one primary-key read, and a stale header is exactly the bug that would leak hidden cards to someone who has just been removed.

**Settings have no TTL, and are written through.** A setting changes only through a settings write, so its cached copy must always match the database. After a write commits, the repository reads the committed value back, stores it over the cached copy, and reads the database once more, retrying up to three times if another write got in between. Cache fills use set-if-absent, so a slow read that loaded an old value can never put it back over a newer write. If the cache cannot be refreshed, the key is deleted and the admin is told, rather than the failure passing silently.

Caching something that is not a table read is allowed above the seam, and the OG resolver's meta cache for template pages (M12) is the one such case. The rule is "a table read is cached at the repository or not at all".

### 3.8 Request lifecycle

Every request runs one fixed global chain, picks up its guards at the route, and then flows down through controller, service, repository and DAO. `utils.NewEngine` installs the first three middleware and `middleware.Setup` the rest, all before the first route is registered, because Gin builds each route's handler chain at registration. A vote on a card walks this path:

```
 POST /api/v1/boards/:boardID/cards/:cardID/votes
   │
   ├─ cache headers   picks Cache-Control as the headers leave, so it covers every response
   ├─ recover         a panic becomes a JSON 500 and a logged stack
   ├─ errors          logs each recorded cause once; a JSON body for any unanswered error
   ├─ tracing         the request span, named by route template, never by raw path
   ├─ detach          everything but GET and HEAD runs on a context a disconnect cannot cancel
   ├─ host allow      Host must match base_url (/livez, /health and /metrics are exempt)
   ├─ sec headers     HSTS, an enforced CSP, frame-ancestors 'none', noindex on share paths
   ├─ cors            only the base_url origin, with credentials; any other origin gets 403
   ├─ content type    415 for text/plain, unparsable or compressed bodies
   ├─ client header   API writes must carry X-RetroTide-Client
   ├─ access log      by route template; slugs and tokens redacted; no IP on board routes
   ├─ maintenance     JSON 503 under /api while maintenance mode is on
   ├─ private site    (M3) on a private site, JSON 401 under /api without a session,
   │                  except the routes that sign a visitor in
   ├─ metrics         duration and in-flight counts by route template
   ├─ last seen       (M3) records a signed-in user's IP when it changes
   ├─ etag            buffers a GET or HEAD 200 and answers 304 when it matches
   ├─ body limit      the hot-reloadable max_body_size; 413 over it
   ├─ guards          per route: principal, permissions, rate limits
   ├─ controller      parses the IDs and the op_id body, builds one spec
   ├─ board gate      (M5) waits at most 5 seconds, else 503 board_busy
   ├─ service         access, board policy, vote rules
   ├─ repository      one transaction: bump version, record op, spend budget, add the vote
   ├─ DAO             one statement per method; a condition that matches nothing is a sentinel
   ├─ PostgreSQL      composite foreign keys and CHECK constraints as the backstop
   └─ deliver         after commit, still in the gate: project per viewer, append to the outbox
```

**net/http fixes a few positions.** Headers leave with the first byte, so a header set after the handler is silently dropped. The cache-header writer therefore sits outermost and picks `Cache-Control` at the moment the headers go out, which also covers the responses that `Recover` and `ErrorResponder` write. Etag holds the response in a buffer, so it can still add `ETag` after the handler, and it sits inside the access log and metrics so they record the 304 that actually went out. The body limit runs last, because nothing before it reads a body.

**Unmatched requests run the same chain.** A path that exists only under another method gets a JSON 405 with an `Allow` header. Every other unmatched path reaches the SPA handler, Gin's NoRoute handler, which answers JSON 404 under `/api/` and serves the app everywhere else. An unmatched request has no route template, so tracing and metrics label it `other`, and the access log redacts the slug segment of a `/b/*` or `/boards/*` page from the raw path.

**The client header is the CSRF defence.** `SameSite=Lax` cookies, CORS limited to `base_url` and the refusal of `text/plain` bodies already protect JSON writes. Bodiless writes, such as signing out, are protected by `X-RetroTide-Client`, which the SPA adds to every request and which a cross-origin page cannot send without a preflight that fails.

**Rate limits attach per route, not globally.** From M3 there are per-IP and per-account buckets on the credential routes and on anything that sends email, a join limit keyed on share slug plus guest device (with a high per-IP ceiling, so a whole office behind one NAT can join a retro at once), and `BoardWriteLimit` on board commands. Each is a `golang.org/x/time/rate` token bucket per key, built once, so every route that names a limiter shares its budget. Wrong board passwords are counted per slug by the live board service itself, because only the service knows the password was wrong.

**The client IP is trusted only from the proxy.** The engine reads `CF-Connecting-IP` alone, and only from a loopback or private peer, which is how Caddy reaches the container through the Docker bridge. Gin's defaults would trust every peer and read `X-Forwarded-For`, so they are switched off. The host accepts web traffic only from Cloudflare, and Caddy overwrites the header, so nobody who finds the server's address can spoof the IP-keyed limits.

### 3.9 Cross-cutting services

Some services belong to no single domain. They are built before any domain service and handed to the domains that need them, and they follow the same rules as any other service: interfaces through a constructor, the database only through a repository, and process state kept private behind methods.

| Service             | What it holds                                                 | Arrives | Section |
|---------------------|---------------------------------------------------------------|---------|---------|
| settings            | typed site settings, validators and hot-reload listeners      | M0      | 6.2     |
| health              | the dependency checks behind `/health`                        | M0      | 6.9     |
| session, guest      | server-side sessions and guest devices                        | M3, M7  | 6.3     |
| authz, orgauthz     | the staff and tenant permission catalogues                    | M3, M4  | 6.4     |
| notification, email | the notification feed and outgoing mail                       | M3      | 6.7     |
| entitlements        | the catalogue of plans, their limits and the quotas           | M4      | 6.6     |
| live.Service        | gates, rosters, outboxes and board access                     | M5, M6  | 6.5     |
| ws.Hub              | the socket registry                                           | M6      | 6.5     |

### 3.10 Composition and shutdown

Wiring is explicit and split across four files at the repository root: `init_db.go`, `init_services.go`, `server.go` and `init_jobs.go`. There is no DI container: `initServices` builds every service in dependency order, so its body is the dependency graph. Today it builds the health service and loads the embedded pages, after applying the stored settings to the cache engines. Boot runs top to bottom:

```
  main.go           config.Load: .env, then the environment
  server.go         cache.New: the manager asks its EngineFactory for the Valkey engine, then the in-memory LRU
  init_db.go        telemetry.Init → db.Open → db.Migrate → store.New(db, cache)
                    → settings.NewService → Refresh → log level, OTLP, Pyroscope
  init_services.go  cross-cutting services, then domain services, in dependency order
  server.go         utils.NewEngine → middleware.Setup → middleware.NewGuards
                    → controllers.Service → routes.PublicRoutes → pprof
  init_jobs.go      settings listeners and validators, then the jobs
  server.go         http.Server with explicit timeouts, and the cleanup function
  main.go           utils.Serve: bind, print the banner, serve until a signal
```

**Settings live in the database, not the environment.** An environment variable named after a setting, such as `BASE_URL`, `VALKEY_URL` or `LOG_LEVEL`, only replaces that setting's default. At boot, `settings.Refresh` writes the defaults of any settings missing from the database. It never deletes a row it does not recognise, because that row may belong to a newer release that has just been rolled back; a retired setting is removed by a later migration, once no release still in use reads it. From then on the database value wins, changed at runtime from the admin panel (M3) and applied through the listeners. Only process-level values, such as the listen address, the Postgres connection and `LOG_FORMAT`, come from the environment alone.

`main.go` loads the configuration, sets Gin's release mode, routes Gin's own writers into zerolog, and builds its context with `signal.NotifyContext`. `utils.Serve` binds the listener, prints the startup banner (or one JSON line under `LOG_FORMAT=json`), and serves until that context ends. The `http.Server` sets every timeout explicitly, including a 3-minute idle timeout that outlasts Caddy's 2-minute upstream keep-alive, so Caddy never reuses a connection the app has just closed.

Two dependencies are genuinely circular, and they are closed with setters after construction rather than by a container: the session manager (M3) and the guest manager (M7) each get the hub as their disconnector, so revoking a session or a guest device closes its sockets.

**Shutdown runs the other way, in a fixed order.** `http.Server.Shutdown` goes first, with 8 seconds. It stops accepting requests and waits for the ones in flight, so every board command already running publishes to its outbox. It neither closes nor waits for sockets, which is why the hub has its own step. The cleanup function then drains inside one 15-second budget:

1. the job tickers stop and their contexts are cancelled, so a running sweep stops before its next item and anything it already committed, such as an idle retro it ended, is published by the outbox flush (M3, M9);
2. the board timer registry stops: it takes no new timer and waits for a firing in flight, so a board it locked at its timer's end is published by the flush too, and a timer that comes due during the deploy fires when the next process starts (M12a);
3. the live board service stops its tick and flushes every outbox, so each committed event reaches its sockets (M6);
4. the hub closes every socket with code 1012, service restart, so clients reconnect with backoff instead of treating the drop as a failure (M6);
5. the claim-and-process worker finishes its in-flight deliveries (M14);
6. the cache client closes.

Today only the last step exists. The two budgets together, 23 seconds, fit inside the compose file's 30-second stop grace period.

### 3.11 Why the boundaries exist

Each boundary earns its place by keeping one question cheap to answer. A boundary that makes no question cheaper is decoration.

**Controller and service**, so that "what does this endpoint accept" and "what is the rule" are never the same question. The first is answered by twenty lines in `internal/controllers` that never mention a table, the second by a service method that never mentions a status code.

**Service and repository**, so the rules can be tested without a database. The repository interface is the seam mockery generates against, and that is the difference between board-policy tests that run in milliseconds and a suite that needs Docker to find out whether a locked board refuses a vote.

**Repository and DAO**, so that every table has one writer and every query has one home. One writer is what makes the cache seam correct at all. One home makes a schema change a search of one package.

**DAO and database**, so that "which statements touch this table" is answered by opening one file.

**The gate and the version**, so that "in what order did this board change, and in what order was that announced" has one answer. Without the gate, two votes committed as versions 42 and 43 could be sent as 43 then 42, and every busy board would refetch after every burst.

**The projection package**, so that "can this viewer see this" has one answer. Snapshots, events, command responses and exports all build their payloads there, so a privacy bug lives in one package, with one golden test suite watching it.

The tests in `internal/arch` turn these boundaries into build failures:

| Test                   | Fails when                                                                                    | Since |
|------------------------|-----------------------------------------------------------------------------------------------|-------|
| `wrappers_test.go`     | a function's whole body is a single pass-through call                                         | M0    |
| `imports_test.go`      | Gin, `coder/websocket`, `internal/db` or a `dao.New*` call appears outside its home           | M0    |
| `imports_test.go`      | a provider's library appears outside its adapter, or an adapter in the identity core          | M3    |
| `gin_test.go`          | a middleware answers other than by aborting, a binder reads the body twice, or `c` escapes    | M0    |
| `sql_test.go`          | SQL or a transaction opener appears outside the data layer                                    | M0    |
| `tenancy_test.go`      | a tenant DAO method takes no tenant ID, outside the allowlisted global queries                | M4    |
| `board_writes_test.go` | anything outside `internal/board` and its sub-packages writes a board                         | M5    |
| `projection_test.go`   | a board payload is built outside the projection package                                       | M5    |

## 4. Frontend

The frontend is a React 19 single-page app, for the web only: no native build, no bearer tokens and no over-the-air bundles. Its skeleton arrived in M1 (the layout, the light and dark themes, the API and socket clients, and the test harness), and the board arrives in M5 and M6. It has four layers and one named sub-layer, on the same principle as the backend: each layer has one rule, and lint enforces every rule.

### 4.1 The layers at a glance

```
 ┌────────────────────────────────────────────────────────────────────────┐
 │ 1 render         src/pages, src/components, src/App.tsx                │
 │                  draws what it is given and owns only view state; the  │
 │                  only layer that may import @dnd-kit                   │
 ├────────────────────────────────────────────────────────────────────────┤
 │ 2 orchestration  src/hooks, src/context                                │
 │                  queries, mutations, socket subscriptions, derived     │
 │                  state, the optimistic overlay, navigation             │
 │ 2a data hooks    src/hooks/queries, src/hooks/mutations                │
 │                  the only modules outside src/api that may import the  │
 │                  endpoints or the query client                         │
 ├────────────────────────────────────────────────────────────────────────┤
 │ 3 pure           src/domain, src/utils, src/types                      │
 │                  the board reducer, sequencer, overlay and selectors,  │
 │                  drop targets and clock offset. No React, no api.      │
 ├───────────────────────────────────┬────────────────────────────────────┤
 │ 4 api  src/api                    │ 4 platform  src/platform           │
 │   client, endpoints, queryKeys,   │   sound, clipboard, site origin,   │
 │   realtime/ with sync/, cache/    │   error reporting. Never api.      │
 └───────────────────────────────────┴────────────────────────────────────┘
```

`src/types/api/**` holds the wire types, one file per Go `internal/dto` file, and every layer may import them, because a type is not a dependency. `src/main.tsx` is the module-graph composition root and `src/App.tsx` the React one, and both are exempt from the import rule by design.

`src/api/realtime/sync/**` is the server-push half of the transport: hooks such as `useBoardSync` that subscribe to typed socket events and patch the query cache. They use React, yet they belong to layer 4, because they sit beneath the data hooks that read the cache entries they write.

The pure layer is where the board logic lives, and all of it is testable with no renderer: the reducer that applies events, the sequencer that orders them, the optimistic overlay that lays pending commands over confirmed state, the function that turns a drag into `{column_id, after_card_id}`, and the server clock offset the timer needs.

**Hidden cards are drawn from counts, never from cards.** A pure selector turns each column's count of other people's hidden cards into that many identical blurred placeholders, which hold nothing to unblur and cannot be dragged, voted on or opened (section 6.5.4).

### 4.2 Allowed imports

| From \ May import           | render | orchestration | data hooks | pure | api (non-transport) | api/endpoints, api/queryClient | platform                     | types |
|-----------------------------|--------|---------------|------------|------|---------------------|--------------------------------|------------------------------|-------|
| render                      | yes    | yes           | yes        | yes  | **no**              | **no**                         | yes, unenforced              | yes   |
| orchestration               | no     | yes           | yes        | yes  | yes                 | **no**                         | yes                          | yes   |
| data hooks                  | no     | no            | yes        | yes  | yes                 | yes                            | no                           | yes   |
| pure (domain, utils)        | no     | no            | no         | yes  | **no**              | **no**                         | no                           | yes   |
| api                         | no     | **no**        | no         | yes  | yes                 | yes                            | **no**                       | yes   |
| platform                    | no     | **no**        | no         | yes  | **no**              | **no**                         | yes                          | yes   |
| test-utils                  | yes    | yes           | yes        | yes  | yes                 | **no**                         | yes                          | yes   |
| main.tsx                    | yes    | yes           | yes        | yes  | yes                 | yes                            | yes                          | yes   |

"api (non-transport)" means `api/queryKeys`, `api/realtime`, `api/cache` and `api/telemetry`. Behind the table sit a few hard rules: no layer is skipped, render never fetches and never touches cache internals, pure never imports React, query keys are built only in `src/api/queryKeys.ts`, and the adapters never import upward. One more is specific to the board: **`@dnd-kit` is imported only under `src/components/board`.** Dragging is a render concern, and the render layer hands a drop to a callback as a `{column_id, after_card_id}` intent.

### 4.3 How the rules are enforced

Every rule except the colour check is an oxlint rule, run at `--max-warnings=0`, so a new violation fails on its first commit.

- **Import rules** are `no-restricted-imports`, configured per layer glob in `frontend/oxlint.layers.ts`. The globs match test files as well as source, so a test is bound by the layer it sits in.
- **Six AST rules** in `frontend/oxlint-plugin-layers.mjs` cover what an import ban cannot: query keys built only in `queryKeys.ts`, no dynamic import of `src/api`, no stubbing of global `fetch`, render tests that mock hooks rather than the transport, pure tests with no DOM, and `@dnd-kit` only under `src/components/board`. The last is an AST rule rather than another import block because oxlint holds one set of `no-restricted-imports` options per file, so a later block would silently replace a layer's bans.
- **House rules**: no single-line `if`, no function whose whole body is one pass-through call, no `forEach`, no `void`, and no comments except the `given` / `when` / `then` markers in tests.
- **Colours come from tokens.** A script fails CI when a hex, `rgb(` or `hsl(` literal appears in a CSS module. Only `src/styles/variables.css` defines colours, for light and dark, and column and card colours travel as token keys such as `green` and `amber`, both on the wire and in the database, never as hex.

There is one documented exception: `src/api/client.test.ts` stubs global `fetch`, because `client.ts` is the transport itself and the only module that calls `fetch`. There are no holdout lists. A file that breaks a layer rule is fixed, never listed.

## 5. Where the halves meet

One process serves both halves. `//go:embed all:static` in `server.go` compiles the built bundle into the binary, so the browser loads the SPA and calls the API on the same origin. The `all:` prefix matters: without it the embed skips files whose names start with `_`, which Vite can emit. Three surfaces cross the boundary, and nothing else does.

- **JSON under `/api/v1`.** Every API route hangs off one group and answers with a type from `internal/dto`. Those types are mirrored by hand, with snake_case fields, in `frontend/src/types/api`. Nothing generates either side from the other, so a field added in Go is real to the frontend only once it is added there too. Every request goes through `src/api/client.ts`, which carries the cookie and the client header and parses the one error shape. Every board payload is already projected for its viewer, and a field the projector sometimes leaves out, such as a card's author on an anonymous board, is optional in the TypeScript type so the render layer cannot assume it.
- **The WebSocket at `/api/v1/ws` (M6).** It uses the same origin and the same cookies, `rt_session` for users and `rt_guest` for guests, and never a token in the query string. Writes never travel over it: inbound it carries only pings and board join, leave and viewer-state frames, and every command goes over REST. Outbound, each board frame is projected for its class of viewer.
- **The HTML document itself.** The SPA handler serves the embedded `index.html` for every page route, and React renders the page in the browser. From M12 an OG resolver rewrites the title, description, canonical link and `og:*` tags per path before serving it, so shared links unfurl with the right preview; search engines index the rendered pages from the sitemap. The public pages are the landing page, pricing, the EasyRetro comparison at `/compare/easyretro`, the self-hosting install page at `/self-host` (which links its compose file, `selfhost/docker-compose.yml`, embedded in the binary and served at `/self-host/docker-compose.yml`), the templates index and one page per template at `/templates/:key`, and the legal pages at `/legal/:page`; the template pages read the same Go catalogue as `GET /api/v1/templates` and `GET /api/v1/templates/:key`, which are public because the catalogue is the same for everyone and holds nothing private. Board, share, organisation and invite paths always get generic meta ("Join a retrospective on RetroTide") and `X-Robots-Tag: noindex, nofollow`, because a link pasted into Slack is unfurled by a third party and must never show a board title. The sitemap lists marketing, template and legal pages only, never the blank template and never a board. A self-hosted instance (`DEPLOYMENT_MODE=self_hosted`) offers none of the marketing, template or legal pages: the SPA sends them to sign-in, the resolver serves the default meta, the sitemap is empty and `robots.txt` disallows everything. That is the one place the server has an opinion about the page the client is about to render.

Stripe and the identity providers, Google and Microsoft, sit beside these surfaces rather than on them. The frontend asks the API for a Checkout, Portal or provider sign-in URL (`POST /api/v1/auth/:provider/start`) and navigates the whole page to it. Stripe sends the browser back to the billing page, and the new plan reaches the frontend over the socket and the entitlements endpoint, never through the redirect's query string. A provider sends the browser back to the API's callback (`GET /api/v1/auth/:provider/callback`), which sets the session (or, for an account with two-factor on, sends the browser to the two-factor step) and redirects into the SPA, so the SPA learns who signed in the way it always does. The SPA learns which providers to offer, and what to call them, from `identity_providers` in the site info.

## 6. Subsystem detail

### 6.1 Data layer and schema

The rules of sections 3.4 to 3.6 hold throughout: all SQL in `internal/dao`, composites in `internal/repository`, one wiring point in `internal/store`, and `internal/model`, `internal/model/spec`, `internal/dto` and `internal/audit` as leaf packages that import nothing above them.

**Migrations** are goose SQL files in `internal/db/migrations`, embedded in the binary, applied at boot, and always created with the goose CLI. A migration that exists is never edited: every schema change is a new migration file, so an existing database, a developer's included, picks it up at the next boot without being reset. Every change is expand then contract: a release only adds tables, indexes and nullable or defaulted columns, and a later release removes what no deployed code reads. The previous release's binary therefore always boots against the new schema, so rolling back never needs a down migration, and from M2 a CI job proves it on every change. A round-trip test migrates a throwaway database down and up again, so every down half is exercised.

**The app connects as the database owner, never as a superuser.** An init script creates a `retrotide` role that owns the database, so goose can run DDL as that role, and a `retrotide_monitor` role for the metrics exporter. The bootstrap superuser is used only by the Postgres container itself.

**Conventions:**

- entity IDs are v4 UUIDs. Board columns, cards, comments and action items get their ID from the caller, built in the browser, so a retried create carries the same ID as the first attempt, and a supplied ID that is not v4 is refused. Postgres 18's `uuidv7()` is never used for board content, because a v7 ID embeds the moment it was made;
- times are `TIMESTAMPTZ`, and `updated_at` is set explicitly in the UPDATE, never by a trigger;
- closed value sets are `TEXT` with a named `CHECK`, which is easier to extend than an enum;
- ordering keys are fractional-index strings from `internal/fracindex`, resolved on the server under the gate, so two people dropping cards into the same gap never collide;
- built-in templates are Go code in `internal/boardtemplate` (and survey templates in `internal/surveytemplate`), not rows, and there is no content seeding: a fresh database holds only what the migrations create and the settings defaults. The one template that is a row is a saved one (M12c);
- uploaded images live in Postgres as WebP, re-encoded on upload, because the app's root filesystem is read-only and Postgres is already backed up (M12b).

**The tenant is the organisation, and the schema enforces it.** Organisation tables carry `org_id`, and board child tables carry `board_id`, a finer partition, because a board can move between teams but never between organisations. Every parent that a child references declares a composite key that leads with its partition column, and every child references it through that key, so Postgres refuses a cross-board or cross-organisation link even from a buggy query. Row-level security is deliberately not used: plain reads run on the pool, so a session-level tenant variable would leak between pooled connections, and the retention jobs and the billing worker are cross-tenant by nature. Composite keys, the vet rules, the tenancy arch test and the cross-tenant tests do the job instead.

**A board is the shareable, realtime unit.** Its identity, share link, version, lifecycle and facilitator settings all live in one `boards` row, from M12a including which columns are revealed or hidden and what the facilitator is presenting, highlighting or focusing, so every visibility decision reads the row the command already holds. Its participants, applied ops, columns and cards hang off it by `board_id`. There is no generic layer above it: anything else that is ever made shareable gets its own tables and its own design.

The tables, by area:

| Area               | Tables                                                                                           | Arrives |
|--------------------|--------------------------------------------------------------------------------------------------|---------|
| Settings           | `site_settings`                                                                                  | M0      |
| Accounts           | `users`, `sessions`, `user_roles`, `role_permissions`, reset and verification tokens             | M3      |
| Site invitations   | `invitations`                                                                                    | M3      |
| Sign-in            | `user_identities`, `oidc_flows`                                                                  | M3      |
| Activity           | `notifications`, `audit_log`                                                                     | M3      |
| Tenancy            | `organisations`, `organisation_members`, `teams`, `team_members`, `organisation_invites`         | M4      |
| Boards             | `boards`, `board_participants`, `board_ops`, `board_columns`, `cards`                            | M5      |
| Guests             | `guest_devices`                                                                                  | M7      |
| Votes and comments | `card_votes`, `board_vote_budgets`, `card_comments`                                              | M8      |
| Action items       | `action_items`, `action_item_reviews`, `action_item_mentions`                                    | M10     |
| Mentions           | `card_mentions`, `card_comment_mentions`                                                         | M10     |
| Card polish        | `card_reactions`, `board_pins`, `media`                                                          | M12b    |
| Surveys            | `board_surveys`, `survey_questions`, `survey_responses`                                          | M12b    |
| Saved templates    | `saved_templates`                                                                                | M12c    |
| Two-factor         | `user_totp`, `two_factor_challenges`, `two_factor_recovery_codes`, `two_factor_trusted_devices`  | M12c    |
| Billing            | `billing_customers`, `subscriptions`, `billing_events`, `org_entitlement_overrides`              | M16     |

**What the schema relies on:**

- **The hot row stays hot.** Every board command updates `boards.version` and `boards.updated_at`. Neither is indexed and the table has `fillfactor = 70`, so the update stays a heap-only update and never touches an index.
- **Merging makes a stack.** Merging sets `parent_card_id`, and each card keeps its own votes, comments and author, so unmerging just clears the column.
- **Severing is a column write.** A card's author is a nullable column, so ending an anonymous retro can remove authorship in one composite (section 6.5).
- **Deletes follow the foreign keys.** Deleting an organisation cascades to everything below it. Deleting a team that still has boards fails, and the team service checks first so that it can say why.

### 6.2 Cache

Section 3.7 gives the rules of the seam. This section is how the manager behaves.

**Two engines, one manager.** The Valkey engine takes its address from the `valkey_url` setting and reconnects when it changes, with no restart, and a validator probes a new URL so that an unreachable address is refused when it is saved. After a failed command it marks itself unhealthy, waits out a cooldown and reconnects in the background. The in-memory engine is a byte-capped LRU sized by `cache_in_memory_max_mb`, and it is always enabled, so losing Valkey degrades the cache to process-local rather than switching it off. Reads and writes go to the first enabled engine, and deletes go to every engine.

**An engine is flushed before it is trusted.** Whenever the engine in use changes, and whenever Valkey starts a new generation (which it does when a failed command marks it unhealthy, and on every URL change), the manager flushes that engine before serving from it. A write that could not reach Valkey therefore never leaves an old value behind: Valkey is flushed before it serves again, and the in-memory engine is flushed before it takes over. An engine that cannot be flushed is skipped. The first engine used after boot is flushed too, which also clears anything a previous process left behind.

```
   Valkey serving ────── a command fails ──────▶ Valkey unhealthy, new generation
        ▲                                                   │
        │                                                   ▼
   Valkey flushed, then serves                  in-memory flushed, then serves
        ▲                                                   │
        └──────────────── Valkey reconnects ◀───────────────┘
```

**Fills never overwrite a write.** `Load` fills a miss with a set-if-absent (`SET NX`), so a read that loaded a value just before a write cannot put that value back afterwards.

The namespaces in `internal/cache/keys.go`:

| Namespace                | Prefix               | TTL        | Invalidated by                           |
|--------------------------|----------------------|------------|------------------------------------------|
| `Setting`                | `setting:`           | none       | written through by settings writes       |
| `UserRole`               | `role:`              | 1 minute   | staff role writes                        |
| `RolePermissions`        | `authz:role-perms`   | 1 minute   | permission writes                        |
| `OrgMembership`          | `org:member:`        | 1 minute   | member add, role change, removal         |
| `TeamMembership`         | `team:member:`       | 1 minute   | team member writes                       |
| `TeamHeader`             | `team:header:`       | 1 minute   | team archive and delete                  |
| `Subscription`           | `billing:sub:`       | 5 minutes  | the billing worker's upsert              |
| `EntitlementOverrides`   | `billing:overrides:` | 5 minutes  | staff override writes                    |
| `OGMeta`                 | `og:meta:`           | 5 minutes  | expiry only (template pages)             |

Every key includes the tenant ID wherever it has one, as in `org:member:{orgID}:{userID}`. Some things are never cached: sessions and guest devices, so deleting either row stops its token on the next request; sign-in identities and flows, so an unlinked identity stops signing anyone in at once; and the board header and all board state (section 3.7).

Valkey is a cache only. RetroTide runs its own small instance, capped at 256 MB with `volatile-lru` and no persistence, because everything in it can be rebuilt. `volatile-lru` evicts only keys that have a TTL, so settings, the one namespace without one, are never evicted.

### 6.3 Auth, sessions and guests

**Sessions (M3)** are server-side, stored in Postgres, with no JWTs and no renewal.

- The `rt_session` cookie carries a random 32-byte token and nothing else, and the `sessions` row stores its SHA-256, so a database leak hands over no usable session.
- **One token helper.** `internal/token` is the only place a token is drawn or hashed: `New` gives 32 random bytes as lower-case hex, and `Hash` gives their SHA-256 as lower-case hex. Sessions, sign-in flows, and the verification and reset links all use it, so every stored hash has the same shape.
- Cookies are `HttpOnly`, `SameSite=Lax` and `Path=/`, with no `Domain`, and whenever `base_url` is https they carry the `__Host-` prefix, which stops a sibling subdomain planting or shadowing one. **One cookie builder** in `internal/cookie` applies these rules for every cookie, given only its name and lifetime; the session manager uses it for `rt_session` and the sign-in flow for `rt_oidc`.
- Password login is by email, and there is no username. It takes the same time whether or not the account exists: an unknown address, or an account without a password, still runs one bcrypt comparison of the same cost against a fixed hash built once, before the same refusal.
- **No sign-in until the address is verified.** Registration creates the account and emails the link, but issues no session. The right password on an unverified account is refused with `email_unverified`, and the login page offers to send the link again through a public route that answers the same for any address. The link marks the address verified and creates no session either, so the user then signs in. An unverified account therefore never holds a session, nothing gates a signed-in user on verification, and a staff unverify also signs the account out. An account created through a provider that vouches for the address starts verified.
- A password reset, a ban or a staff unverify deletes every session of the user and disconnects their sockets through the hub; "sign out other devices" keeps only the current one.
- Changing an email address is two-phase: the new address waits beside its verification token and replaces the old one only when its link is used, so an account never holds an address its owner has not proved.

**The site mode (M3)** is a site setting, `public` by default or `private`.

- **Public:** anyone can register, by password or through a provider. Password sign-up waits for email: until an email provider is configured it is closed (apart from the first account), because a new account could never receive its verification link, and it reopens as soon as an admin saves working email settings. A provider sign-up needs no email, because the provider has already verified the address. Email settings that are set but broken count as no email, never as an error page: the cause is logged once and the admin dashboard shows email as failing.
- **Private:** the API is closed to anyone without a session. A middleware just after maintenance answers 401 to every API request except the site info, the session check, signing in and out, registering, verifying an address, recovering a password, and looking up and accepting a site invitation or an organisation invite (M4), so someone invited to an organisation who has no account creates one while accepting; the SPA sends everyone else to the login page. Pages, assets and the probes are untouched, and guest board links (M7) pass as well, because organisation admins share them with people who hold no account: the two share-link routes are open, and a live guest device passes on the board routes and the socket as a live session does. A private site signs in by password only: it lists no identity provider, and the provider routes answer 404.
- **The first account** can always register, whatever the mode, and becomes the super admin, because otherwise nobody could ever get into a private site. After that a private site refuses registration on the server, and the site info tells the SPA whether registration is open, so the page and its link disappear.
- **Email invitations** are how people join a private site. Staff invite an address, and the email links to a "Create your account" page that shows the address, which cannot be changed, and asks for a name and a password. An invitation stores only a hash of its token, lasts 7 days and works once. Accepting it creates the account already verified, because the link proves the mailbox, and spends the invitation in the same transaction; it creates no session, so the page then offers Sign in. Staff can list, resend (a new link and expiry) and cancel pending invitations, and can neither invite an address that already has an account nor invite anyone while the site is public.

**Signing in with an identity provider (M3)** sits beside email and password from launch on a public site, with Google as the first provider, and ends in the same `session.Manager.Create`, so everything after it (the guards, socket revalidation, bans) is unchanged.

- **A neutral core and one adapter per provider.** `internal/identity` holds the rules every provider shares: the flow, sign-up, sign-in, the linking rule, linking, unlinking, the notices and the audit entries. It never imports an adapter and never names a provider. Each provider is an adapter in its own subpackage behind the `identity.Provider` interface, `internal/identity/google` and, from M12c, `internal/identity/microsoft`: it gives its id and display name, reads its own settings to say whether it is switched on, builds the authorisation URL, and turns the callback's code into the identity the core needs, including its own decision on whether it verified the address and is authoritative for it. The adapters are composed in `init_services.go`. Only the adapters import their providers' libraries (`go-oidc` and `x/oauth2`, shared by both), so Microsoft sign-in arrived as a second adapter with no new route, controller or core rule.
- **Generic routes.** `POST /api/v1/auth/:provider/start` and `GET /api/v1/auth/:provider/callback` serve every provider, so Google's registered redirect URI is `<base_url>/api/v1/auth/google/callback`. A provider that is switched off, that no adapter claims, or on a private site answers 404 on both as if it did not exist. The site info lists the switched-on providers as `identity_providers`, each with its id and name, and the SPA draws one button per entry.
- **The flow** is the authorisation code flow with PKCE and, for an OpenID Connect provider, a nonce. The core draws the state, the nonce and the verifier, writes a flow row that lives for 10 minutes, and binds it to the browser with a short-lived `rt_oidc` cookie. The callback consumes the row exactly once, only at the callback of the provider it was started for, and checks the state, so nobody can finish their own sign-in in someone else's browser.
- **Identities are matched by the provider's account ID, never by email.** Once linked, an identity keeps signing in its own account even after its address at the provider changes.
- **The linking rule** is the security-critical part, and the core applies it to every provider alike. An identity whose address the provider has not verified is refused outright. A verified identity links to an existing account automatically only when the provider is also authoritative for the address (for Google, a `@gmail.com` or `@googlemail.com` address, which are the same mailbox and match each other, or a Workspace account) and the account's own email is verified. Otherwise the user signs in with their password and links the provider from account settings, which asks for the current password. Someone who registers an address they do not control therefore gains nothing when its owner later signs in with a provider.
- **Little is kept.** RetroTide keeps the provider's account ID and the address at link time, uses the name once as a starting display name, stores no provider token and calls no provider API. The SPA loads no provider script.
- **The callback always answers with a redirect** into the SPA, never with JSON, because it is a page navigation. A refusal lands on the login or account page with a provider-neutral `error` code and the provider's id as `provider`, so the page can name the provider in its message.
- **Microsoft (M12c)** signs in work accounts from Microsoft Entra ID: any organisation's by default, one tenant when a tenant ID is set, and personal accounts only when a setting allows them. The multi-tenant endpoint has no single issuer, so the adapter checks the issuer and tenant itself, and it keys the identity on the tenant and object IDs. Entra sends no "email verified" claim, so Microsoft is authoritative for an address only when its domain-verified claim says so; otherwise a Microsoft identity links only from account settings.
- **A passwordless account confirms a new link by signing in again** with a provider it already holds (M12c): the core runs a re-authentication flow first, checks the returned identity belongs to the signed-in account, and only then starts the link at the new provider.

**Two-factor authentication (M12c)** is TOTP, written over the standard library. Turning it on needs the current password and a confirming code; turning it off needs a code. Both sign-in paths ask one gate after the first factor and before a session exists: with two-factor on, the server issues a short-lived challenge cookie instead of a session, and a separate verify step, with five attempts in five minutes, creates the session. A code is accepted once. An automatic provider link to an account with two-factor on is refused, so nothing is linked before the second factor. Turning it on signs every other device out and shows 10 single-use recovery codes once, stored only as hashes and never shown again; a recovery code works at sign-in and for turning two-factor off. "Don't ask again on this device" stores a hashed device token for 30 days, which skips the code for that account only and is forgotten when the password changes or two-factor is turned off. A password sign-in without "Remember me" gets a one-day session in a cookie that ends with the browser.

**Password sign-in can be switched off (M12c)** on a public site that has a provider configured, so a company can sign in with its Microsoft accounts alone. The server then refuses every password route before looking anyone up, and the admin settings refuse a save that would leave the site with no way in.

**Guests (M7) are real principals.** `internal/guest` has the same shape as the session manager. The `rt_guest` cookie carries a random token, `guest_devices` stores its hash, and it lasts 90 days from creation. One cookie per browser covers any number of boards, and other people only ever see participant IDs, which are scoped to one board.

**The principal is resolved per route.** `Guards.Principal` tries `rt_session`, then `rt_guest`, and puts the principal into the request context, where services read it through `authctx`. A database error answers 503. It is never quietly treated as a guest, which would hand a signed-in user a guest identity during a blip. The socket resolves its principal itself, in the same order, through the same two managers.

**Joining by link.** A board's share link carries a random slug of at least 128 bits, separate from the board's ID. Looking the slug up says only whether a password or an account is needed, and the title stays hidden until access is granted. Joining checks the organisation's guest policy, the board's link access and the password, issues `rt_guest` when needed, and writes the participant row under the board's gate. Rotating the link or changing the password bumps an access epoch that evicts everyone who came in through the link, while members keep access through their membership.

```
   /b/<slug> ────────▶ GET /api/v1/b/:slug → says only whether a password or an account is needed
   name, password ───▶ POST /api/v1/b/:slug/join
                       │ organisation allows guests → board link access → password
                       │ no principal yet: issue rt_guest, its hash stored in guest_devices
                       │ under the board's gate: upsert the participant, bump the version
                       ▼
                    the board's ID, then the SPA opens /boards/<id>, outside any organisation,
                    and joins the board over the socket
```

**Claiming is opt-in.** When a browser holding `rt_guest` signs in, the user may bring their contributions on named boards with them. The SPA asks for the offer once per signed-in user, whatever the sign-in path, and declining only clears the guest cookie. Anonymous boards are never offered, because claiming would link anonymous cards to an account. Claiming never grants organisation membership.

**Leaving.** Deleting an account detaches the user from every board through the live board service, under each board's gate, so the boards show "Deleted user" and the change is versioned and announced like any other. An owner of an organisation with other members must transfer ownership first.

### 6.4 Permissions

Access has four independent layers (site staff, organisation and team roles, board roles, and guests), and each is a catalogue declared once in code. Ownership checks, such as "is this your own card", stay in the services.

- **Site staff** (`internal/authz`, M3) are `super_admin`, `admin` and `support`. Admins hold every permission and cannot be edited, and `support` is editable at runtime from the admin panel. While the users table is empty, the first registration is allowed whatever the site mode says and becomes `super_admin`, atomically, so two simultaneous first registrations cannot both win; on a private site every later one is refused (section 6.3). That first account starts with its email verified, on the password and the provider path alike, so it signs in at once and configures everything, email included, without an email provider. The super admin bypasses every account-level restriction (the ban, the lock, maintenance, the DroneBL block and the unverified-email refusal) and every permission check, but not the rate limits or Turnstile on the public routes, and nobody can ban, lock, unverify or demote it. Staff with `manage_settings` pass maintenance, and staff holding `manage_user_account` manage the email invitations of a private site. Staff have no implicit access to tenant boards: a staff role opens the admin panel and the organisation list, never a board, a card or an export.
- **Organisation roles** (`internal/orgauthz`, M4) are fixed in code: owner includes admin, and admin includes member. Owners hold every organisation permission. Admins hold all of them except managing admins, transferring ownership and deleting the organisation, so an admin can manage members but can never create another admin. Members may only view.
- **Team roles** are lead and member, and organisation owners and admins count as lead on every team. Leads hold every team permission; members may view the team, create boards and manage its action items.
- **One role hierarchy** (`internal/hierarchy`, M4) answers every question of whether one person may act on another, for staff and organisation actions alike: site staff sit above every organisation role, each person stands on their highest rung, and nobody may act on anyone at their own level or above, or grant a role at or above their own. No other package compares ranks, which an arch test enforces.
- **Board roles** are facilitator and participant, derived afresh on every request and every socket join, as described below. Each participant action belongs to a stage, and the server refuses it in any other: a participant writes, edits, moves and deletes their own cards in Write, votes in Vote, comments and merges cards in Discuss (when the board allows it), and adds, edits and marks action items in Actions. A facilitator is bound by the same stage rules for content, and also runs the board (settings, moving it through its stages of Write, Vote, Discuss, Actions and End, one step forward or one step back, the timer, columns, vote resets, per-column reveal and hiding, presenting, highlighting and focusing, pins, surveys, removals, locking and ending the retro) and manages it (renaming, sharing, the password, link rotation, the background, archiving, moving between teams, cloning, saving as a template and deleting). A board setting, "Anyone can edit" (M12a), lends every seated participant the timer and the shaping of columns. A participant may export only as an organisation member.
- **Guests** exist only on boards, and can only ever be participants.

An invite bound to an email address is accepted only by a signed-in user whose verified email matches it. Organisations are created explicitly: registration creates none, and a new user who is not joining through an invite names their workspace on the onboarding page, in one transaction that creates the organisation, the owner's membership and a first team.

**The effective board role is derived, never stored as a permission.** `live.Service.Resolve` works it out on every request and every socket join. On a team board, being effective lead of the board's team gives facilitator, and being a team member gives participant. On an organisation board, which has no team, an organisation owner or admin facilitates and every other member participates. Whoever creates a board is stored as its facilitator, and a stored promotion counts only while membership access holds. Access through the link alone is capped at participant, and counts only while the link admits that kind of principal and the participant's access epoch is current. A removed participant fails even where membership would otherwise admit them, although nobody can remove a team lead from their own team's board. No candidate means 404, so boards cannot be enumerated. Board state then narrows the role: an archived, closed or locked board is read-only apart from the facilitator's own controls, and a board in a frozen team is read-only for everyone (section 6.6).

**Access is re-evaluated whenever it could have shrunk**: removing a participant or a member, lowering a role, rotating a link, changing a password, narrowing link access, or turning guests off. `live.Service.ReevaluateHeld` takes each affected board's gate in turn and resolves its viewers again, and anyone who now fails is dropped from the roster and receives nothing further.

**The audit log never records card, vote or comment writes**, so it cannot reveal who wrote an anonymous card. It records organisation, membership, team, board-sharing and board lifecycle, billing, sign-in identity, two-factor, site invitation and staff actions.

**There is no content filter.** Nothing users type is filtered, names and titles included. Boards are closed team spaces, and abuse is handled by facilitators removing cards and participants, the report link on guest pages, and staff moderation.

### 6.5 The realtime board

A board is edited by up to a few hundred people at once, some signed in and some guests, some able to see cards that others cannot, and all of them getting live updates. Five decisions shape this subsystem:

- **Writes go over REST and are fanned out over the socket after commit**, so every board command gets the controller layer, the guards, the error contract and the test harness for free.
- **Each board has one in-process gate and one version**, so the order of changes and the order of announcements are the same order.
- **Every command carries a client-generated `op_id`**, so a retry is never applied twice.
- **One pure projector decides what each viewer may see.**
- **Recovery is a full snapshot, never a diff.** There is no event log, no tombstone and no delta endpoint.

The gate, the version and the projector arrive with boards in M5, the hub and the board frames in M6, and reveal, anonymity and the timer in M9.

**The board code is split by job, and the imports run one way.** `internal/board/live` is the realtime engine: the gates, board access (`Resolve`), the rosters and outboxes, the socket side of `ws.BoardPresence`, re-evaluation, joining by link and the guest claim. `internal/board` is the command service for boards, columns, cards, comments, votes, merges, facilitation, sharing and participants, together with the command runner (`board.Runner`) and the people a board may name. `internal/board/actionitems` holds the action item commands and runs them through the same `board.Runner`, and M12b added four more sub-services in the same pattern: `cardpolish` (reactions and pins), `images` (card images and the background), `management` (archive, delete, restore, move, CSV import, board pins and bulk actions) and `surveys`. `internal/board/policy` and `internal/board/projection` are pure. `board` and its sub-services import `live`, the sub-services import `board` for the shared command rules, and `live` imports none of them.

#### 6.5.1 The hub

- **The hub is an `http.Handler` on `coder/websocket`**, mounted with `gin.WrapH`, so Gin never enters `internal/ws`. It checks the `Origin` against `base_url` and resolves the principal from the cookies before accepting a socket.
- **Clients are keyed by principal**, so revoking a session or a guest device closes its sockets, and every five minutes each socket revalidates its principal and every board it has joined.
- **Slow consumers are cut off** rather than allowed to stall a broadcast, and recover through a snapshot. Each payload is marshalled once per class of viewer, not once per viewer.

#### 6.5.2 The gate, the version and the write path

`live.Service` (`internal/board/live`) owns, for every live board, a gate (a one-slot channel), a roster of who is viewing and in what role, and an outbox of projected events waiting to be sent. None of them outlives its use, so there is nothing to evict. A channel is used rather than a mutex because a wait on it can be cancelled: every caller waits at most five seconds, so a stalled Postgres answers 503 `board_busy` instead of piling up goroutines. The gate is always taken before a transaction begins, never inside one, and every board command follows the path drawn in sections 3.6 and 3.8. Because the gate is held until the result is in the outbox (section 3.3), events join the outbox in version order.

- **A failed command burns no version**, because its sentinel rolls back the whole transaction, the version bump included.
- **Announcements are batched.** The outbox flushes on a short window, so a burst of fifty votes reaches each viewer as a handful of frames rather than fifty.
- **Retries are safe.** A duplicate `op_id` is answered as a replay and changes nothing. `board_ops` has no participant column, so it cannot link a person to an action, and it is purged after 24 hours.
- **Writes that touch several boards**, such as deleting a user or re-evaluating access, take the gates one at a time in ascending board-ID order, so they cannot deadlock.

#### 6.5.3 Joining and resynchronising

A tab joins a board over the socket with a `board_join` frame. The join runs under the gate, adds the viewer to the roster and replies with the current version, so every change after that version is guaranteed to reach the socket. Each change then arrives in a numbered frame covering the versions it spans for that recipient. A change they may not see simply does not appear, so an empty frame never tells anyone that someone else has just saved a hidden card. Presence, notifications and the viewer's own vote budget travel beside these frames, unnumbered.

Recovery is the same handshake on every socket open: join under the gate, get the version, refetch a snapshot only if the cached board is behind (an unchanged board answers 204), and apply the frames buffered meanwhile. While live, a gap in the numbering triggers one refetch. There is no polling anywhere, and after a deploy most clients reconnect straight onto the 204 fast path.

#### 6.5.4 Hidden cards and the blur model

Section 1 states the rule; this is how it holds.

- **Hidden is the default.** Every new board starts in the writing phase with its cards hidden, unless its creator chooses otherwise.
- **Each viewer receives their own cards in full**, marked as hidden so the UI can say "only you can see this", **and everyone else's only as a count per column.** Nothing else describes another participant's hidden card: no ID, position, length, author, votes, comments or timestamp. In Go this is structural, because a hidden card is simply absent from the snapshot, along with everything that hangs off it.
- **The client draws that many identical blurred placeholders.** The blur is CSS on a card that holds no text, so devtools have nothing to recover.
- **The facilitator reveals by moving the board into its Vote stage.** That step is the reveal: one reveal event carries every card and comment, projected once for everyone except that each author's copy marks their own cards and comments as theirs, so every placeholder becomes its real card at the same version.
- **There is no peek.** Facilitators, team leads and organisation owners see their own cards and the same counts as everyone else, no setting changes that, and an export they take shows no more. Re-hiding is cosmetic, and the UI says so, because the text has already been delivered.
- **Nothing hangs off a hidden card for others.** Votes and comments on it reach only its author, and neither a merge nor an action item can be made from it.

```
  Column "Went well" while cards are hidden: Sam and Ali have each written one card

  the server stores           Sam's browser receives          Sam's screen
  ─────────────────           ──────────────────────          ────────────
  Sam: "Deploys were   ──▶    Sam's card in full,       ──▶   his card, marked
        smooth"               marked as hidden                "only you can see this"

  Ali: "Standups ran   ──▶    a count of 1 for          ──▶   one blurred placeholder
        long"                 this column                     holding no text

  the facilitator      ──▶    one reveal event with     ──▶   every card, for everyone,
  moves to Vote               every card in full              at the same version
```

#### 6.5.5 Anonymity and the other privacy rules

Anonymity has two levels, and only ever gets stronger: a board may switch from named to anonymous at any time, but not back once any card exists.

- **Named** boards store the author and show them.
- **Anonymous** boards store the author only while the retro is open, and send it to nobody, facilitators included; the server needs it only so that people can see, edit and delete their own cards. Anonymous payloads carry no timestamps.
- **While cards are hidden on an anonymous board**, counts move only on a 10-second board tick, even in a refetched snapshot, and presence shows no active or idle state, so timing cannot point at an author.
- **Revealing an anonymous board** shuffles every card's ordering key, so the order on screen does not follow the order the cards were typed.
- **Ending the retro severs authorship.** "End retro", or the auto-end job after 14 days without a command, clears the author of every card and comment and folds the votes into the card totals. After that, authorship survives only in backups, for at most 6 months.

Side channels are closed by design. Logs and spans name requests by route template, redact share slugs and record no IP for board routes or the socket, and Caddy writes no access log for share and board paths. Removing a participant never deletes their cards, so a removal cannot reveal which were theirs. Other people see only a vote total per card, and your own votes are sent only to you. The audit log, notifications, content IDs, exports and link previews follow the same rule (sections 6.4, 6.7, 6.1, 6.10 and 5). What cannot be closed is documented: the version still moves when someone saves a hidden card, so a viewer can tell that something changed, but never what, where or by whom.

**One pure projector decides all of it.** `internal/board/projection` takes committed state and a viewer and returns DTOs. Snapshots, events, command responses, exports and the summary page all go through it, an arch test forbids building a board DTO anywhere else, and every new board event, snapshot field or export column needs a golden privacy test (section 8) before it merges.

#### 6.5.6 Votes, moves and the timer

Votes are conditional statements (section 3.3), with an optional per-card cap beside the per-board one and, from M12a, an optional budget per column, and deleting a card refunds every vote on it; a move names the card it should follow and is placed by the server under the gate, so two people dropping cards into the same gap never collide. The timer's end time lives on the board row and the server never ticks it: each client draws the countdown locally, corrected by its measured offset from the server's clock. The one exception is auto-lock (M12a): on a board that asks for it, an in-process registry of one timer per board locks the board when its timer ends. A firing takes the board's gate, re-reads the board and does nothing if the timer moved, the registry is rebuilt from the database at boot, so a timer that ended during a deploy still locks, and it stops in its own step of the shutdown order (section 3.10).

#### 6.5.7 Facilitation, card polish and surveys (M12a, M12b)

Everything M12a and M12b added to the live board follows the same two rules as everything before it: every change is a board command under the gate, and every payload is built by the projector, with a golden privacy test.

- **Per-column reveal** reveals one column's cards while the rest stay hidden, with the same one-event reveal and the same anonymous shuffle, column by column. A card cannot move between a revealed column and a hidden one, because the move would publish or unpublish it.
- **Hidden columns** disappear for everyone but facilitators, and nothing in them can change until they are shown again. Events about a hidden column reach facilitators only, through a third delivery class in the projector beside "the author" and "everyone else", and anyone else who names the column is told it does not exist. A facilitator still cannot read another person's hidden card inside one.
- **Presentation, highlight and focus** are one "spotlight" state on the board row, followed live by everyone. It can only ever name a card or column everyone can see: hiding anything clears the spotlight on it in the same statement.
- **Reactions and pins** exist only on a card everyone can see. Reactions show counts and the viewer's own choices, never who reacted, and an anonymous board's End severs them like authorship. Survey answers are severed the same way, and a survey's results reach nobody, facilitators included, until it is closed.
- **Card images** are uploaded by organisation members, re-encoded to WebP by `cwebp`, stored with no uploader, and served only to someone who can open the board, from a server-built path, never a client-supplied URL. PDF and PNG exports are a print page in the browser, not a server export.

#### 6.5.8 The frontend half

- **Confirmed and pending state are kept apart.** `useBoardSync` patches the confirmed board, under its own query key root, through the pure reducer, and `useBoardView` lays each pending command over it until the confirmed version catches up. Nothing optimistic is ever written into the cache, so a failed command's change disappears by itself and a rollback can never overwrite somebody else's change.
- **Commands are retried with the same `op_id`.** `useBoardCommands` generates the `op_id`s and the IDs of new content from `crypto.getRandomValues`, so they work outside a secure context. Network errors and gateway failures are retried for about 30 seconds, conflicts and refusals become toasts, and everything else is reported. No error is swallowed.

### 6.6 Entitlements and billing

**Entitlements are decided in one place** (`internal/entitlements`, M4). `plans.go` holds the catalogue of plans in code. `For(ctx, orgID)` combines the live subscription's plan (Free when there is none), the status rules (a trial counts as paid, a failed payment keeps the paid plan through a grace period, and a cancellation runs to the end of the period) and any unexpired staff overrides. Services call `Require` or `CheckWithin`, and a denial becomes a 402 `upgrade_required`, which opens the upgrade prompt in the SPA. The entitlements endpoint only gives the SPA hints for disabled buttons and badges.

**Pricing is per team.** The Free plan allows one team and ten organisation members, and from M12b and M12c three saved templates and 5 MiB per uploaded image. The paid Team plan is priced per team and counts members per team, up to 30, so one person in two teams takes a place in each. Guests never count as members. The prices themselves are a business decision and live outside this document.

**Quotas are held under a row lock, with the comparison in SQL.** A quota-bound create, such as a team or a member, locks the organisation row and runs a conditional insert that succeeds only while the count is below the limit the service passed in. The service decides the limit and the statement enforces it, the same shape as the vote caps.

**A downgrade never deletes data.** Creating anything over a limit is refused, and the teams beyond the allowance are frozen, read-only and answering 402, until the organisation upgrades or archives teams. Abuse limits, such as boards per day or participants per board, are site settings rather than plan limits, and answer 429 on every plan.

**Billing arrives with M16, behind a provider-neutral service.** Until then `billing_enabled` stays off, every organisation resolves to Free plus any overrides, and the billing routes answer 404.

- `internal/billing` exposes checkout, the portal, webhook ingestion, event processing, reconcile and the team-quantity sync. The Stripe adapter lives in `internal/billing/stripe`, so a second provider would be a second adapter with no schema change.
- Checkout and the Customer Portal are Stripe-hosted pages. The billing service never grants access directly: it writes `subscriptions`, and the repository invalidates the entitlement caches after commit.
- **Webhooks go into an inbox.** The handler verifies the signature on the raw body, inserts the event into `billing_events`, ignoring duplicates, nudges the worker and answers 200 at once. The claim-and-process worker from M14 then handles each event, fetching the subscription's current state from Stripe rather than trusting the event, so Stripe's duplicates and out-of-order delivery do not matter. A failing event backs off and, after ten attempts, is dead-lettered and raises an alert.
- An hourly reconcile job repairs anything the webhooks missed, including a subscription quantity that no longer matches the number of active teams.

### 6.7 Notifications and email

A notification is both a database row, so it appears on the notifications page, and a live event, so the bell counter updates without a reload. `internal/notification` (M3) delivers to signed-in users only, never to guests: `Notify` writes the row, pushes it to the user's sockets through the hub, and sends an email when a provider is configured and the type calls for one.

- **There are few types**: an organisation invite, an action item assigned to you, the billing notices to the owner, and the warnings before a dormant free organisation is deleted. Only the action-item notice follows the user's email preference; the rest are service messages and are always emailed.
- **Security mail goes straight through `internal/email`**: verification, password reset, the email-change notice, a private site's invitations, and the notices that a provider's sign-in (Google's or Microsoft's) was linked to or removed from the account, which name the provider by its adapter's display name.
- **Notifications never carry board content.** No card, vote or comment write raises one, and none carries card text or a card ID, so the table cannot link a person to an anonymous card.
- **Notifying happens outside the board's gate**, because it writes a row and may wait on the mail provider, while every other command on that board waits at most five seconds for the gate.
- **Mail** goes through SMTP or Cloudflare Email Sending, chosen by a setting. While neither is configured a send fails with `email.ErrNotConfigured`, never a silent success, and every flow that must send email refuses up front with `email_unavailable` instead of claiming a link went out.

The only uploads are card images and board backgrounds (M12b), handled by the media pipeline in `internal/media` (section 6.5.7).

### 6.8 Background jobs

Recurring work runs as plain goroutines started from `registerListeners` in `init_jobs.go`, each through one `scheduleJob` helper. A job runs once at boot and then on a ticker, logs a count only when it did something, is labelled for pprof, and stops on a shared channel at shutdown, which also cancels the context it runs on, with a wait group so that a deploy never kills a sweep halfway through an item: a sweep stops between items and the next run picks up the rest. Every job is an idempotent sweep, and each arrives with the tables it sweeps.

| Job                     | Runs       | What it does                                                |
|-------------------------|------------|-------------------------------------------------------------|
| expired sessions        | hourly     | deletes expired sessions                                    |
| sign-in flows           | hourly     | deletes provider sign-in flows past their 10-minute expiry  |
| two-factor challenges   | hourly     | deletes two-factor challenges past their 5-minute expiry    |
| trusted devices         | daily      | deletes two-factor trusted devices past their 30 days       |
| guest devices           | daily      | deletes guest devices expired or unseen for 90 days         |
| stored IPs              | daily      | nulls IPs past their 30 or 90 days                          |
| soft-delete purge       | daily      | purges boards and organisations deleted over 30 days ago    |
| unattached media        | daily      | deletes images older than 24 hours that nothing names       |
| retro auto-end          | hourly     | ends retros idle for 14 days, severing anonymous authorship |
| board ops               | hourly     | purges `board_ops` rows older than 24 hours                 |
| dormant organisations   | daily      | warns, then deletes free organisations dormant 18 months    |
| expired invites         | hourly     | deletes invites past expiry                                 |
| notifications           | daily      | prunes notifications older than 90 days                     |
| audit log               | daily      | deletes audit rows older than 400 days                      |
| deliveries (M14)        | continuous | delivers webhooks, retries, dead-letters, alerts            |
| billing events (M16)    | continuous | claims `billing_events` on the same worker                  |
| billing payloads (M16)  | daily      | clears `billing_events` payloads older than 90 days         |
| billing reconcile (M16) | hourly     | re-fetches Stripe state and repairs drift                   |

The auto-end job ends a retro through the live board service, under its gate, exactly as the "End retro" button does, so on an anonymous board it severs authorship. The board timer registry (M12a, section 6.5.6) is not a job: the board service owns it and the shutdown order stops it. The two continuous rows run on the one queue, the claim-and-process worker (`internal/worker`, M14). It is not a ticker job: it wakes on a nudge as well as on a short interval, claims each row in a short transaction so that no transaction is held across a network call, and stops in its own step of the shutdown order.

Today `registerListeners` wires the settings listeners and validators that make hot reload work. The listeners cover the log level, the OTLP endpoint, the Pyroscope URL, the request body limit, and every cache engine that implements `settings.Listener`, which is how the Valkey URL and the in-memory size reach their engines. The validators probe a new `valkey_url`, put a floor under `max_body_size` and check the log level, so a bad value is refused when it is saved rather than when it is first used.

### 6.9 Observability

There are five independent signals. Traces, profiling and the log level are changed at runtime from the site settings, and log collection belongs to the observability stack on the server, not to the app.

- **Metrics** are a Prometheus registry on `/metrics`, served through `gin.WrapH` and guarded by `metrics_token`, and Caddy blocks the path publicly, so the scraper reaches it over the internal network. HTTP metrics are labelled by route template, never by raw path. Beside them sit the pool, cache and socket metrics, board metrics such as gate wait time and resyncs (M6), billing inbox metrics (M16), and product counters such as sign-ups and boards created, which give product numbers without a cookie or a third-party script. A failing collector costs only its own series, not the whole scrape.
- **Traces** are an OpenTelemetry span per request from RetroTide's own tracing middleware rather than `otelgin`, which would record the raw path, share slugs included, and the client address. Spans are named by route template, with a span per SQL statement through otelsql and per Valkey command. The OTLP exporter is swapped live when its setting changes.
- **Profiling** is Pyroscope from `pyroscope_url`, plus `net/http/pprof` under `/debug/pprof/`, which needs a session holding the `manage_settings` staff permission; the metrics token opens no profile.
- **Logs** go through zerolog. With `LOG_FORMAT=json` the app writes JSON to stdout, and Grafana Alloy ships it to Loki, which keeps 30 days. A line logged for a request carries `trace_id` and `span_id`, and Gin's and net/http's own writers are routed into the same stream. The access log redacts share slugs and sensitive query values such as tokens and passwords; from M3 the Google callback's `code` and `state` join them. From M13 the SPA reports its errors to `POST /api/v1/client-errors`, which is rate-limited per IP, refuses a body over 4 KB, stays open on a private site and logs each report once, truncated and with the page's route template (such as `/b/:slug`) rather than its path or query.
- **Health** is two endpoints. `/livez` always answers 200 and is the container healthcheck. `/health` runs the dependency checks, with Postgres hard and Valkey soft, answers 503 when a hard check fails, gates each deploy, and sits behind the metrics token.

### 6.10 Exports

`internal/export` (M11) builds every export through the projector, for the requester's current view. Hidden cards stay hidden exactly as they are on screen (section 6.5.4), whatever the requester's role. Authors appear only on named boards, and anonymous boards show no dates. Exporting needs `board.export`, which a participant has only as an organisation member, never through a link alone.

- **CSV**, in the familiar layout of column titles over cards with their vote counts and comments. Cells that could run as a spreadsheet formula are escaped, so card text written by a guest can never execute in someone's spreadsheet.
- **Markdown**, with its control characters escaped, for pasting into Confluence, Notion and GitHub.
- **JSON**, a board export DTO built from the requester's own snapshot, with no participant, user, card, comment or action item IDs.
- **An organisation-wide export** for owners and admins, with each board projected exactly as in a board export, **a bulk export** of chosen boards from the dashboard (M12b), leaving out any the requester may not export, and **a personal data export** for every user: their account, linked sign-in identities, organisation memberships with role and teams, and the cards, comments and action items still linked to them.
- **PDF and PNG** (M12b) are not server exports: a print page renders the board the viewer can see, and the browser prints it or turns it into an image.

A board export is `GET /api/v1/boards/:boardID/export?format=csv|markdown|json`, and a missing or unknown format is a 400 `invalid_format`. Every export carries the board's action items, each under the card it was made from, with its title (mentions as plain text), assignee, due date and status. An item with no source card, or whose card the requester cannot see, goes in a trailing "Other actions" group that never names or quotes that card, so an action item can never leak hidden card text.

**The board summary** is a read-only report at `/o/:orgSlug/boards/:boardId/summary` (and the shared-board equivalent), built in the frontend from the same projected snapshot the board page loads, so it needs no endpoint of its own and shows anyone who can view the board exactly what the board shows them. The board header carries the Summary link for every viewer and an Export dropdown for viewers with `board.export`; the account page has "Your data", and organisation settings show "Export organisation" to owners and admins.

Streamed exports write straight to the response through an `io.Writer`, so the export service never learns about Gin. The service checks access and loads its first page before it writes, so a failure there is still a JSON error. A failure after the first byte aborts the connection, so the client sees a failed download rather than a truncated file that looks complete.

### 6.11 Seams for later

Each of these attaches at a named point without reshaping anything above it:

- **Planning poker and a health-check radar** are paid features after launch, each designed as its own feature with its own tables; nothing in the board code is kept generic for them.
- **The paid halves of the free features** (unlimited saved templates, a larger image size and survey trends) are plan limit values and a read across a team's closed surveys, on what M12b and M12c built.
- **Organisation SSO** uses verified domains and a SAML sidecar, so RetroTide never parses signed XML, and enforces itself at the same point before a session is created where the two-factor gate already sits.

Surveys, reactions, reveal per column, focus mode, server-side timer expiry, card images, Sign in with Microsoft and TOTP were seams here until M12a to M12c built them (sections 6.3, 6.5.6 and 6.5.7).
- **Strict anonymity** never stores the author at all: ownership becomes a secret held by the author's browser.
- **Integrations** are Slack, Teams and outgoing webhooks on the M14 worker, and Jira, Linear and GitHub as adapters.
- **A second replica** means rewriting section 6.5 around Valkey pub/sub and Postgres advisory locks (section 2.2).

AI features have no seam. RetroTide has no AI summaries, grouping, template generation or chat, and none is intended.

## 7. Technology choices

This section gives the reasons behind the stack. Exact versions live only in `go.mod`, `frontend/package.json` and the image tags, and RetroTide takes each new major as it lands and fixes forward rather than pinning back.

### 7.1 Backend

- **Go 1.27.** The floor is load-bearing, because the generic methods on `*cache.Manager` need it. Tests also lean on `testing/synctest` for anything with goroutines or clocks.
- **Gin on `net/http`.** The engine is a plain `http.Handler` served by `http.Server`, so handlers reach the standard `*http.Request` and its context, tests drive the engine with `httptest`, graceful shutdown is `http.Server.Shutdown`, and `promhttp` and `net/http/pprof` mount through `gin.WrapH` and `gin.WrapF` with no adapter. RetroTide uses only Gin's router, context, binding and rendering, with JSON through `encoding/json`. The engine comes from `gin.New()`, never `gin.Default()`, and `gin.Recovery` is not used: it can dump request headers, and with them the session cookies, into the log, it answers a panic with an empty 500, and it swallows the abort that a streamed export relies on. Recovery, the error responder, etag, the security headers, host authorisation, the cache headers and the rate limiters are RetroTide's own middleware, and CORS is `gin-contrib/cors`.
- **`jackc/pgx/v5` through `database/sql`.** Going through the standard interface keeps `*sql.DB` and `*sql.Tx` as the currency of the data layer, which keeps `TxOrDB`, `WithTx` and `WithReadTx` a few lines each and lets `otelsql` wrap the driver for a span per statement.
- **goose** for migrations, run in-process at boot as a library, with new migrations created through the goose CLI; and **sqlc** for typed queries and its `vet` tenancy rules. sqlc, **mockery** and **staticcheck** run from the `go.mod` tool block, so none of them needs a separate install.
- **`valkey-io/valkey-go`**, behind the engine interface, so the app is correct with no Valkey at all.
- **`coder/websocket`** for the hub (M6). It is context-first, so cancelling a connection's own context ends its pending read, its pinger and any join waiting at the gate together, and every method except reading is safe to call concurrently. `gorilla/websocket` was not chosen, because it has had no release since mid-2024 and allows only one writer at a time.
- **`coreos/go-oidc/v3` and `golang.org/x/oauth2`** for Google sign-in (M3) and Microsoft sign-in (M12c). go-oidc discovers the provider, caches its signing keys across rotations and verifies ID tokens, which would otherwise be written by hand over a JWT library, and it ships a fake provider for tests. A hosted identity service was not chosen, because it would hold every user's email outside RetroTide.
- **`stripe-go`** (M16), the official SDK, whose webhook verification works on the raw body and which pins an API version.
- **zerolog**, **`prometheus/client_golang`**, the **OpenTelemetry SDK** with the OTLP/HTTP exporter, **`grafana/pyroscope-go`** and **`hellofresh/health-go`**, all configured from settings at runtime.
- **`golang.org/x/time/rate`** for every rate limiter, **`wneessen/go-mail`** for SMTP, `x/crypto` for bcrypt and `x/text` for normalisation.
- **testify**, **testcontainers-go** with its Postgres module, and the valkey-go mock for tests.
- **The standard library** for share slugs, for tokens (`crypto/rand` and `crypto/sha256`, behind `internal/token`), for the ordering keys and, from M12c, for TOTP (RFC 6238 over `crypto/hmac`).
- **`cwebp`** (M12b), from Alpine's `libwebp-tools` in the runtime image, which `internal/media` runs to re-encode every uploaded image to WebP, stripping its metadata.

### 7.2 Frontend

- **React 19, TypeScript 7 and Vite 8.** `npm run build` type-checks before it builds, so a type error fails the build.
- **React Router**, imported as `react-router`, never `react-router-dom`.
- **TanStack Query v5** as the server-state layer. Socket events replace polling, and `useMutationState` drives the optimistic overlay.
- **Plain `fetch`** in one client module, with no HTTP library and no runtime schema validation.
- **CSS Modules** with tokens, and no CSS framework.
- **dnd-kit** for drag and drop. It is headless, so it fits home-made components, and it has keyboard sensors and one pointer sensor for mouse and touch. It is pre-1.0, so it is pinned exactly, and render tests lock down the behaviour RetroTide relies on.
- **Vitest** with Testing Library and jsdom, **oxlint** and **oxfmt**; **marked** and **DOMPurify** only for the legal pages, which are Markdown under `src/content/legal`; optional **Turnstile**; **qrcode** for the share dialog and two-factor QR codes and **html-to-image** for the PNG export (M12b, M12c); and self-hosted fonts, so a page load makes no third-party request.
- **Accessibility target WCAG 2.2 AA.** A polite live region announces remote changes such as a new card or a reveal, and every drag has a keyboard path. The UI is en-GB only, and template content is catalogue data, so a translation needs no code.

### 7.3 Infrastructure

- **One multi-stage `Dockerfile`.** A `node:lts-alpine` stage builds the SPA into `static/`, a `golang:1.27-alpine` stage builds a static binary with `-trimpath` and the version stamped in, embedding `static/`, and an `alpine:latest` runtime, with `libwebp-tools` installed for image uploads (M12b), runs it as uid 10001 on port 4323, with a healthcheck on `/livez`.
- **Floating image tags**, so every build picks up the latest patches and any breakage is fixed forward. The one exception is Postgres, pinned to its major version (`postgres:18`), because each major changes the on-disk data format.
- **Hardened containers**: every capability dropped, no new privileges, CPU, memory and PID limits, read-only root filesystems for the app, Valkey and the metrics exporter, and no Postgres or Valkey port published in production. The app's `GOMEMLIMIT` sits below its memory limit, because the Go runtime follows the cgroup CPU quota but not the memory limit.
- **One production compose file for every stack.** Production and staging run from the same file, with names, ports and limits taken from each stack's `.env`.
- **A development compose** that starts only RetroTide's own Postgres and Valkey, on uncommon loopback ports, so it never meets other services on the same machine. The containerised app sits behind a profile, with its own database.
- **Caddy behind Cloudflare.** The host firewall accepts web traffic only from Cloudflare's ranges, and Caddy requires Cloudflare's origin-pull certificate, overwrites `CF-Connecting-IP`, deletes `code`, `state` and `token` and the `Referer` header from its own log lines, and writes no access log at all for share and board paths.
- **Operations** (the server, deploys, backups and the rebuild runbook) are in a private runbook kept outside the repository.

### 7.4 External services

None is needed to boot the server, and each degrades cleanly when it is not configured.

- **Stripe** (M16) for Checkout, the Customer Portal and subscriptions, as merchant of record.
- **Google** (M3) and **Microsoft Entra ID** (M12c) as OpenID Connect providers, each off while its client ID or secret is empty and discovered on first use, so the server boots without reaching either. Neither receives board content.
- **Cloudflare** for DNS, TLS, the WAF and, optionally, email sending and Turnstile, or an **SMTP relay** for mail.
- Optional, and outside the repository: a Prometheus scraper, an OTLP collector, Pyroscope, Loki with Alloy, an uptime monitor on `/livez`, and an offsite store for the nightly backups.

## 8. Testing strategy

A test belongs to the layer its subject sits in, and reaches at most one layer down. Every test uses the `given` / `when` / `then` markers, and mocks are always generated with mockery. Locally, only the packages a change touches are run; CI runs everything.

| Layer         | How it is tested                                                                                                        |
|---------------|-------------------------------------------------------------------------------------------------------------------------|
| controllers   | `testutil.NewHarness` builds the production engine, mounts one route list as production does, mocks the service         |
| middleware    | one middleware and a terminal route on a fresh engine; rate limiters run under `testing/synctest`                       |
| engine        | `utils.NewEngine` itself: JSON 500 on panic, 405 with `Allow`, no trailing-slash redirect, the client-IP rules          |
| root package  | the route-table test mounts every route list on one engine, and the pprof guard is tested end to end                    |
| services      | strict mockery mocks of every dependency, with `testing/synctest` wherever goroutines are involved                      |
| pure packages | values in, values out: table tests for the ordering keys, the board policy and the entitlement rules                    |
| repositories  | a mocked DAO and Valkey, asserting the exact cache commands after commit: write-through for settings, deletes elsewhere |
| DAOs          | a real `postgres:18` through testcontainers, with a fresh database per test                                             |
| architecture  | the `internal/arch` tests of section 3.11                                                                               |

A few suites carry more weight than the rest:

- **Cross-tenant tests** are the main guarantee of isolation. Every tenant table has a DAO test that creates rows in organisation A and reads and writes through organisation B's IDs, expecting not found with A unchanged, and controller tests cover every organisation, team and invite route of another organisation, a record of another organisation reached through your own, another organisation's board, a removed member's old board and a guest after a link rotation, all answering 404.
- **Golden privacy tests** for the projector marshal a board holding the card text `SECRET-TEXT-9f2c` and a known author for every event, frame, command response, snapshot and export. They run for every kind of viewer (the author, another participant, a guest, the facilitator, the organisation owner, a removed participant and an export viewer), on named and anonymous boards, with cards hidden and shown, and from M12a with single columns revealed or hidden, and assert that neither the secret nor the author's ID appears anywhere it should not. M12b adds reactions, images and surveys to the matrix, so a reaction never names who reacted and an open survey's results reach nobody.
- **Concurrency tests** against Postgres: 40 goroutines voting on one card with a budget of 5 give exactly 5, a failed command leaves the version unchanged, 20 concurrent inserts after one anchor give 20 distinct, ordered positions, and 8 concurrent acceptances of one invitation create exactly one account.
- **The private-site gate** is tested route by route: every route a visitor needs to get in passes with and without a session and reads neither the mode nor the session, the list of those routes is asserted exactly, and every other API route, an unmatched one included, answers 401 without a live session.
- **Socket tests** (M6) run on a real test server, dial with `coder/websocket` and run under `-race`.
- **The Google and Microsoft adapters** run against fake OIDC providers, covering a wrong audience, issuer, nonce or verifier, an expired token, an unknown signing key and an unverified address, and for Microsoft another tenant and a personal account while those are off, and the provider-neutral linking rule is a table of cross-account cases over a made-up provider, each asserting what was written as well as the outcome. The architecture tests keep each provider's libraries inside its own adapter and every adapter out of the neutral core.
- **Migrations** have a round-trip test that runs every down half and, from M2, a CI job that boots the previous release's image against the new schema.
- **Load tests** are k6 scripts that drive 150 participants joining one board and 50 voting at once, run against staging and never against production.

The frontend follows the same principle, layer by layer:

| Layer         | What its test does                                                                    | What it may not do                                                            |
|---------------|---------------------------------------------------------------------------------------|-------------------------------------------------------------------------------|
| pure          | calls the function and compares values: no renderer, no providers, no mocks           | import `@testing-library/react`, React or `src/api`                           |
| api endpoints | drives `endpoints/testHarness.ts` and asserts each call's method, path and body       | reach the network or stub global fetch                                        |
| data hooks    | `renderHook` over one mocked endpoint module, asserting the key and the invalidation  | restate transport shape, which the endpoint test already owns                 |
| realtime sync | drives `FakeWebSocket` and `emitRealtimeEvent` and asserts the cache after each frame | open a real socket or reach `api/endpoints` directly                          |
| orchestration | `renderHook` over the real hook with its data hooks mocked                            | render the component that consumes it                                         |
| render        | `renderWithProviders` and Testing Library queries, with the data hook mocked          | `vi.mock` `api/endpoints`, `api/queryKeys`, `api/client` or `api/queryClient` |
| platform      | stubs the device API, re-importing under `vi.resetModules()` where module scope reads | import `src/api`                                                              |

The board's pure logic has its own tests (the reducer, the sequencer, the optimistic overlay, drop targets, sort and filter, and the clock offset), and render tests assert the live-region announcements, a keyboard path for every drag, and that a column with a hidden count of n draws n identical placeholders holding no text. There are no coverage thresholds. `npm run test:names` lists every test name, so a refactor can prove it dropped none.

## 9. File map

The backend, as it stands after M0:

```
  main.go               entry: configuration, signals, serve, the startup banner
  server.go             the cache, the engine, middleware, guards, routes, http.Server, cleanup
  init_db.go            telemetry, database, migrations, repositories, settings
  init_services.go      cache engine start-up and every service, in dependency order
  init_jobs.go          settings listeners and validators; the background jobs from M3
  pprof.go              net/http/pprof as route data
  sqlc.yaml             sqlc generation and the vet tenancy rules; .mockery.yml lists the mocked interfaces

  internal/controllers  the HTTP surface: route lists, health, the SPA handler; utils/ for binding and errors
  internal/routes       mounts the API list under /api/v1 and the page list at the root; the SPA is NoRoute
  internal/middleware   the global chain of section 3.8, and middleware.Guards
  internal/utils        NewEngine, Serve and the startup banner
  internal/repository   repository interfaces, composites and the cache seam
  internal/dao          DAO interfaces, sentinels and row mapping; queries/, sqlcgen/, daotest/
  internal/model        spec/ holds the input specs today; rows and typed constants arrive with the first domain (M3)
  internal/dto          the wire types
  internal/store        store.New, the single wiring point
  internal/db           connection, migrations, WithTx and WithReadTx, dbtest
  internal/cache        the manager, its engines and the namespace table
  internal/config       process configuration and the site setting definitions
  internal/settings     the settings service: typed reads, validators, hot-reload listeners
  internal/logger       zerolog set-up
  internal/telemetry    tracing and profiling
  internal/health       the dependency checks behind /health
  internal/arch         the architecture tests

  scripts/              regen_sqlc.sh, regen_mocks.sh, test.sh, check-links.mjs, loadtest/
  deploy/               deploy.sh, backup.sh, restore.sh, systemd/, Caddyfile.example, postgres/ (the init script that creates the database roles)
  .github/              ci.yml and deploy.yml, CODEOWNERS
```

The packages still to come, by milestone: `session`, `token`, `cookie`, `authz`, `identity` (the provider-neutral core, with one adapter subpackage per provider, `google/` first), `user`, `email`, `notification` and `admin` in M3; `org`, `team`, `orgauthz` and `entitlements` in M4; `board` (with `policy/` and `projection/`), `fracindex` and `boardtemplate` in M5; `ws` in M6; `guest` in M7; `actionitem` in M10; `export` in M11; `og` and `sitemap` in M12; the board timer registry in `board` in M12a; `media`, `surveytemplate` and the board sub-services `cardpolish/`, `images/`, `management/` and `surveys/` in M12b; `twofactor`, `savedtemplate` and the `identity/microsoft/` adapter in M12c; `worker` and `integration` in M14; and `billing` (with `stripe/`) in M16. The frontend arrives in M1 under `frontend/`, and its directories are the layers of section 4.1, with `oxlint.layers.ts` spelling each layer as a glob.
