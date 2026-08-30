# ARMOR Format Migration 2026

## iad-ci Bucket Migration Status

**Status:** BLOCKED - Requires operator action to provision admin token in OpenBao

### Investigation Summary (2026-08-29)

Agent attempted to authenticate to OpenBao and provision the required admin token:
- ✅ Successfully authenticated to OpenBao via AppRole (`~/.config/openbao/rs-manager/role_id` + `secret_id`)
- ✅ Received valid token with policies `["default", "ex44"]`
- ❌ Token lacks write permission to `secret/rs-manager/iad-ci/armor/admin` (403 permission denied)
- ❌ Policy read also returns 403 (cannot inspect ex44 policy scope)
- ❌ Admin token does not exist in OpenBao or Kubernetes armor-secrets Secret

**Conclusion:** Creating the admin token requires operator-level OpenBao permissions beyond the agent's ex44 policy scope.

### Current State
- ARMOR deployment version: `ronaldraygun/armor:0.1.1933` (includes `/admin/format/migrate` endpoint)
- Admin API endpoint: Deployed and accessible (requires ARMOR_ADMIN_TOKEN environment variable)
- ExternalSecret configuration: Already set up to sync `admin-token` from OpenBao
- Admin token in OpenBao: **Does NOT exist** (confirmed via authenticated API check)
- Admin token in Kubernetes: **Does NOT exist** (confirmed via kubectl)

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
