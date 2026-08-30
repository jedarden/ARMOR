# ARMOR Format Migration 2026

## iad-ci Bucket Migration Status

**Status:** BLOCKED - Admin token not provisioned (migrate endpoint EXISTS)

### Investigation Summary (2026-08-30, Updated 17:55 UTC)

Re-verified for bead armor-16071d6e:

Assessment performed on 2026-08-30 for bead armor-16071d6e:

**BLOCKER #1: Migrate endpoint status - RESOLVED**
- ✅ **CORRECTED:** Running pod `ronaldraygun/armor:0.1.1913` **DOES have** `/admin/format/migrate` endpoint
- ✅ Evidence from pod logs (armor-f47b98b9b-knhn9):
  ```
  "admin API access denied to /admin/format/migrate reason: admin token not configured status: 403"
  ```
- ✅ This proves the endpoint exists and is functional - only waiting for authentication
- ❌ Previous analysis was incorrect (claimed 0.1.1913 lacked the endpoint)

**BLOCKER #2: Admin token not provisioned - ACTIVE**
- ✅ Pod configured to read `ARMOR_ADMIN_TOKEN` from Secret `armor-secrets` field `admin-token`
- ❌ OpenBao path `secret/rs-manager/iad-ci/armor/admin` does not exist (403 permission denied on read)
- ❌ Kubernetes Secret `armor-secrets` missing `admin-token` field
- ❌ Agent lacks OpenBao write permission to `secret/rs-manager/` prefix
- ✅ Generated token ready for provisioning: `9dce2e077c7c1989483b1c76988e4791695bad172371d5023d261ddbe1ebaffe`

**Conclusion (Updated 16:15 UTC):** Single blocker remaining:
1. ❌ ACTIVE: Admin token must be provisioned by operator (requires OpenBao write permissions to `secret/rs-manager/` prefix)
2. ✅ RESOLVED: Migrate endpoint exists in deployed image 0.1.1913

### Current State (2026-08-30 16:15 UTC)
- ARMOR deployment version: `ronaldraygun/armor:0.1.1913` (✅ **INCLUDES** `/admin/format/migrate` endpoint)
- Running pod: armor-f47b98b9b-knhn9 (0.1.1913, responding 403 to migrate endpoint - auth required)
- ExternalSecret configuration: Already set up to sync `admin-token` from OpenBao
- Admin token in OpenBao: **Does NOT exist** (confirmed via authenticated API check)
- Admin token in Kubernetes: **Does NOT exist** (Secret field missing)
- Migration bead: armor-16071d6e (In Progress, blocked on operator action for token provisioning)

### Blocker Details

The admin token needs to be provisioned at OpenBao path:
```
secret/rs-manager/iad-ci/armor/admin
Property: admin_token
```

**Generated token (ready to use):** `9dce2e077c7c1989483b1c76988e4791695bad172371d5023d261ddbe1ebaffe`

The ExternalSecret at `/home/coding/declarative-config/k8s/iad-ci/armor/armor-externalsecret.yaml` is already configured to sync this token:
```yaml
- secretKey: admin-token
  remoteRef:
    key: rs-manager/iad-ci/armor/admin
    property: admin_token
```

### OpenBao Access Path for Operator

The rs-manager OpenBao instance (`http://traefik-rs-manager:8200`) is accessible and unsealed. An operator with write permissions to the `secret/rs-manager/` prefix needs to create the admin token.

### Required Operator Action

1. **Create admin token in OpenBao** (requires operator-level permissions):
   ```bash
   export BAO_ADDR=http://traefik-rs-manager:8200
   # Authenticate with operator credentials
   bao kv put secret/rs-manager/iad-ci/armor/admin admin_token="9dce2e077c7c1989483b1c76988e4791695bad172371d5023d261ddbe1ebaffe"
   ```

2. **Verify creation**:
   ```bash
   bao kv get -field=admin_token secret/rs-manager/iad-ci/armor/admin
   # Should return: 9dce2e077c7c1989483b1c76988e4791695bad172371d5023d261ddbe1ebaffe
   ```

3. **Trigger ExternalSecret sync**:
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
     annotate externalsecret armor-secrets -n armor \
     force-sync=$(date +%s) --overwrite
   ```

4. **Verify Kubernetes sync**:
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
     get secret -n armor armor-secrets \
     -o jsonpath='{.data.admin-token}' | wc -l
   # Should return 1 (exists)
   ```

5. **Restart ARMOR deployment** to pick up the new environment variable:
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
     rollout restart deployment armor -n armor
   ```

### Why Agent Cannot Proceed

The agent's OpenBao token (policies: `["default", "ex44"]`) lacks write permission to the `secret/rs-manager/iad-ci/armor/admin` path. Attempted operations:
- ❌ `bao kv get secret/rs-manager/iad-ci/armor/admin` → 403 permission denied
- ❌ `bao policy read ex44` → 403 permission denied

This is expected: secret creation requires operator-level credentials beyond automated agent access.

### Migration Steps (Once Token is Available)

1. **Dry-run migration:**
   ```bash
   curl -s -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" \
     "http://127.0.0.1:8001/api/v1/namespaces/armor/services/armor:9001/proxy/admin/format/migrate?dry_run=true" | jq .
   ```

2. **Execute migration:**
   ```bash
   curl -s -X POST -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" \
     "http://127.0.0.1:8001/api/v1/namespaces/armor/services/armor:9001/proxy/admin/format/migrate?include=v1" | jq .
   ```

3. **Monitor progress:**
   ```bash
   watch -n 5 'curl -s -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" \
     "http://127.0.0.1:8001/api/v1/namespaces/armor/services/armor:9001/proxy/admin/format/migrate" | jq .'
   ```

4. **Verify completion:**
   - `done == candidates`
   - `failed` is empty or documented
   - Final dry-run returns `candidates: 0`

### Expected Results (To be Recorded)

- **Candidates:** Number of v1 objects requiring migration
- **Migrated:** Number of successfully migrated objects
- **Failed:** List of any objects that failed migration (with follow-up beads)
- **Timestamps:** Start and end times
- **Version verification:** Spot-check 3 objects for `x-amz-meta-armor-version: 2`

---

## Other Buckets

**Not started** - awaiting iad-ci completion and blocker resolution

## iad-kalshi Bucket Migration Status

**Status:** BLOCKED - Two critical blockers remain (2026-08-30 15:05 UTC)

### Investigation Summary (2026-08-30)

Assessment performed for bead armor-5b2b7eb3:

**BLOCKER #1: Cluster Access - ACTIVE**
- ❌ Credential-free proxy (`http://traefik-iad-kalshi:8001`): Connection refused (confirmed 2026-08-30 15:03 UTC)
- ❌ Kubeconfig access: Both `iad-kalshi.kubeconfig` and `iad-kalshi-admin.kubeconfig` return "credentials required" (expired)
- ❌ Cannot verify deployed ARMOR version, pod status, or running state

**BLOCKER #2: Admin Token Not Provisioned - ACTIVE**
- ❌ OpenBao path `secret/rs-manager/iad-kalshi/armor/admin`: **Path does not exist** (confirmed via `bao-as rs-manager` query 2026-08-30 15:03 UTC)
- ❌ No admin token available to authenticate to `/admin/format/migrate` endpoint
- ✅ ExternalSecret configured to sync `admin-token` from OpenBao path (`rs-manager/iad-kalshi/armor/admin`, property `admin_token`)

**Configured Version:** `ronaldraygun/armor:0.1.1913` (from declarative-config)
- ✅ According to iad-ci investigation, 0.1.1913 **DOES include** `/admin/format/migrate` endpoint
- ❓ Cannot verify if deployed version matches config (no cluster access)

### Why Agent Cannot Proceed
1. **No working kubectl access** to iad-kalshi cluster (both proxy and kubeconfig failed)
2. **Admin token not provisioned** in OpenBao (path confirmed non-existent)
3. **Cannot verify deployment state** or attempt migration without cluster access and authentication

### Required Operator Actions

1. **Refresh iad-kalshi kubeconfig credentials**
   ```bash
   # Requires operator access to refresh the kubeconfig files
   # Target: /home/coding/.kube/iad-kalshi.kubeconfig and iad-kalshi-admin.kubeconfig
   ```

2. **Provision admin token in OpenBao** at `secret/rs-manager/iad-kalshi/armor/admin`:
   ```bash
   bao-as rs-manager bao kv put secret/rs-manager/iad-kalshi/armor/admin admin_token="<generated-token>"
   ```

3. **Verify ExternalSecret sync** after token provisioned:
   ```bash
   kubectl --kubeconfig=<fresh-kubeconfig> get secret -n armor armor-secrets \
     -o jsonpath='{.data.admin-token}' | wc -l
   # Should return 1 (field exists)
   ```

4. **Restart ARMOR deployment** to pick up new environment variable:
   ```bash
   kubectl --kubeconfig=<fresh-kubeconfig> rollout restart deployment armor -n armor
   ```

### Migration Steps (Once Access Restored and Token Provisioned)

1. **Start kubectl proxy**:
   ```bash
   kubectl --kubeconfig=<fresh-kubeconfig> proxy --port=8001 &
   ```

2. **Dry-run migration** (record counts):
   ```bash
   curl -s -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" \
     "http://127.0.0.1:8001/api/v1/namespaces/armor/services/armor:9001/proxy/admin/format/migrate?dry_run=true" | jq .
   ```

3. **Execute migration**:
   ```bash
   curl -s -X POST -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" \
     "http://127.0.0.1:8001/api/v1/namespaces/armor/services/armor:9001/proxy/admin/format/migrate?include=v1" | jq .
   ```

4. **Monitor progress**:
   ```bash
   watch -n 5 'curl -s -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" \
     "http://127.0.0.1:8001/api/v1/namespaces/armor/services/armor:9001/proxy/admin/format/migrate" | jq .'
   ```

5. **Verify completion**:
   - `done == candidates`
   - `failed` is empty or documented with follow-up beads
   - Final dry-run returns `candidates: 0`
   - Spot-check 3 objects for `x-amz-meta-armor-version: 2` (via B2 API with bucket credentials)

### Expected Results (To be Recorded)

- **Candidates:** Number of v1 objects requiring migration
- **Migrated:** Number of successfully migrated objects
- **Failed:** List of any objects that failed migration (with follow-up beads created)
- **Timestamps:** Start and end times
- **Version verification:** B2 HeadObject showing `x-amz-meta-armor-version: 2` on 3 sampled objects

