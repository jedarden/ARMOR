# bf-nzm9 — Epic: ARMOR web dashboard (finalize in Go, remove Rust scaffold)

**Status:** CLOSED. Tracking/parent epic; all four children closed and the CLOSE
conditions verified.

## Close-condition verification (2026-07-19)

### Children — all CLOSED (dependency order)

| Bead    | Title                                              | Outcome                                                                 |
|---------|----------------------------------------------------|-------------------------------------------------------------------------|
| bf-2928 | Remove abandoned Rust/Actix scaffold               | CLOSED — `Cargo.toml`/`Cargo.lock`/`src/main.rs` absent from tree+index |
| bf-4ox4 | JSON object-listing API (`GET /dashboard/api/list`) | CLOSED — implemented, routed, tested, documented                        |
| bf-1daa | Verify bucket browser UI acceptance + fill gaps    | CLOSED — behaviors covered by named tests                               |
| bf-668r | Verify encryption status + cache stats display     | CLOSED — per-object badges + cache stats in HTML, JSON covered by tests |

### Build & test

```
go build ./...   # BUILD_EXIT=0
go test ./...    # TEST_EXIT=0  (all packages green, incl. internal/dashboard, internal/server)
```

### Docs consistency (no edits required — final state already consistent)

- `docs/dashboard.md`: endpoint table includes `/dashboard/api/list?prefix=<p>`
  (bf-4ox4) with auth notes; cache-stats / encryption-status covered.
- `docs/plan/plan.md`: Phase 3 "Web dashboard (bucket browser, encryption
  status, cache stats)" marked ✅ COMPLETE — consistent with the Go direction.
- No stale Rust build instructions in `docs/` or `README.md`. The only Rust
  mentions are historical language-evaluation notes under `docs/research/`
  (legitimate record of the Go-vs-Rust decision). `disaster-recovery.md`
  ("root of t**rust**") and `cloudflare-architecture.md` ("Zero T**rust**")
  hits are false-positive substring matches.

## Direction

ARMOR's core and dashboard are Go. The abandoned Rust/Actix-web scaffold
(commits 3609ad2, a5e96b5, 931684d) produced by a misdirected worker session
was removed under bf-2928. The Go dashboard lives in `internal/dashboard/`
(`dashboard.go` + `dashboard_test.go`), wired into `internal/server/server.go`
`AdminHandler()` at `/dashboard/*` on `ARMOR_ADMIN_LISTEN` (default
`127.0.0.1:9001`).
