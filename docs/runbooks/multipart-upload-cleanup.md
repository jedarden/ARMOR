# Production multipart upload cleanup

**Evidence date:** 2026-11-04 UTC  
**Stale threshold:** seven days after multipart initiation

This runbook records the production cleanup prompted by the 2026-11-03
read-only ARMOR review. The inventory used the S3 `ListMultipartUploads` API
with paginated `ListParts` calls to sum stored part bytes. Credentials were
read by reference from OpenBao and were never printed or stored in this
repository.

## Result

Only uploads initiated at least seven full days before the cleanup scan were
aborted. Uploads younger than the threshold were not selected. B2's native
bucket lifecycle configuration now independently aborts incomplete large files
after seven days; no Kubernetes Job or CronJob and no ARMOR-side sweeper is
required.

| Bucket / ARMOR scope | Before: stale count | Before: stale bytes | Before: oldest upload | After: total in progress | After: bytes | After: stale count | After: oldest upload |
|---|---:|---:|---|---:|---:|---:|---|
| `iad-ci` | 1,583 | 13,194,970,750 | 2026-05-22 12:03Z | 51 | 456,130,560 | 0 | 2026-10-30 06:49Z |
| `kalshi-tape` | 6 | 26,172,456,960 | 2026-10-26 19:35Z | 0 | 0 | 0 | — |
| `devimprint` | 3,927 | 459,620,661,910 | 2026-08-07 02:08Z | 45 | 14,297,390,760 | 0 | 2026-10-28 14:57Z |
| `nap-dashboard` / rs-manager + tradegraph | 13 | 2,047,845,537 | 2026-09-17 16:43Z | 0 | 0 | 0 | — |

The `after` scan found 51 `iad-ci` uploads and 45 `devimprint` uploads, all
younger than seven days. Every post-cleanup `ListParts` request succeeded.

## Ongoing safeguard

The cleanup mechanism is the Backblaze B2 bucket lifecycle rule
`daysFromStartingToCancelingUnfinishedLargeFiles: 7` with an empty
`fileNamePrefix`, so it covers every ARMOR namespace in each bucket. The
configuration location is the B2 bucket's native `lifecycleRules` field,
managed with `b2 bucket update`; the account-scoped management credential is
referenced from OpenBao at `secret/rs-manager/backblaze/api`. Existing B2
object hide/delete rules were preserved. The four configured buckets verify
the generic rule with value `7`.

ARMOR's source contains explicit aborts for failed in-process multipart
operations, but no abandoned-upload background sweeper. The B2 lifecycle rule
is therefore the production cleanup authority. Do not manually remove ARMOR
`.armor/` objects while investigating multipart uploads.

## Recheck

Use the same read-only inventory procedure: list multipart uploads for each
bucket, paginate `ListParts` for each upload, sum `Size`, and classify an
upload as stale only when `Initiated < now - 7 days`. Abort only that stale
set. Re-read the list afterward and record total count, total bytes, stale
count, and oldest initiation time for each bucket.
