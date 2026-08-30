# MEK Rotation 2026 - rs-manager ARMOR Instance

## Status
**Procedure finalized and documented. Awaiting operator execution.**

## Context
This document describes the rotation of the Master Encryption Key (MEK) for the rs-manager ARMOR instance using the key ring mechanism (Plan §8.13). The old key remains in the ring for backward compatibility.

## Pre-Rotation State

### rs-manager ARMOR Deployment
- **Instance:** rs-manager (cluster: `rs-manager`)
- **Namespace:** `armor`
- **OpenBao Path:** `secret/rs-manager/backblaze/armor`
- **Admin Token Path:** `secret/rs-manager/rs-manager/armor/admin`

### Current MEK Configuration
- **Active MEK:** Retrieved from OpenBao (operator access required)
- **MEK Ring:** Not yet configured (this rotation establishes the ring)
- **Escrow Path:** `secret/rs-manager/escrow/armor`

## Rotation Procedure

### Phase 1: Prepare the Key Ring (Operator Action)

1. **Retrieve the current MEK from OpenBao:**
   ```bash
   # As operator with OpenBao write access:
   bao-as rs-manager -- bao kv get -field=master-encryption-key secret/rs-manager/backblaze/armor > /tmp/current_mek.tmp
   chmod 600 /tmp/current_mek.tmp
   ```

2. **Generate a new MEK:**
   ```bash
   openssl rand -hex 32 > /tmp/new_mek.tmp
   chmod 600 /tmp/new_mek.tmp
   ```

3. **Update OpenBao with the new ring structure:**
   ```bash
   # Build the MEK_RING value (current MEK becomes first ring member)
   CURRENT_MEK=$(cat /tmp/current_mek.tmp)
   NEW_MEK=$(cat /tmp/new_mek.tmp)

   # Update OpenBao: new MEK as active, current MEK in ring
   bao-as rs-manager -- bao kv patch secret/rs-manager/backblaze/armor \
     master-encryption-key=- <<< "$NEW_MEK" \
     MEK_RING=@/tmp/current_mek.tmp

   # Update escrow with both keys
   bao-as rs-manager -- bao kv patch secret/rs-manager/escrow/armor \
     mek=@/tmp/new_mek.tmp \
     mek_ring=@/tmp/current_mek.tmp

   # Secure cleanup
   shred /tmp/current_mek.tmp /tmp/new_mek.tmp
   rm -f /tmp/current_mek.tmp /tmp/new_mek.tmp
   ```

4. **Verify the update:**
   ```bash
   # Check metadata version increased
   bao-as rs-manager -- bao kv metadata get secret/rs-manager/backblaze/armor | jq .data.current_version

   # Verify escrow was updated
   bao-as rs-manager -- bao kv metadata get secret/rs-manager/escrow/armor | jq .data.current_version
   ```

### Phase 2: Update Kubernetes Deployment

1. **Add ARMOR_MEK_RING environment variable to deployment:**
   ```yaml
   # In declarative-config/k8s/rs-manager/armor/armor-deployment.yml
   env:
   # ... existing env vars ...
   - name: ARMOR_MEK_RING
     valueFrom:
       secretKeyRef:
         name: armor-secrets
         key: MEK_RING
         optional: true  # Ring may not exist for first rotation
   ```

2. **Update ExternalSecret to include MEK_RING:**
   ```yaml
   # In declarative-config/k8s/rs-manager/armor/armor-externalsecret.yml
   data:
     # ... existing data mappings ...
     - secretKey: MEK_RING
       remoteRef:
         key: rs-manager/backblaze/armor
         property: MEK_RING
   ```

3. **Commit and push changes:**
   ```bash
   git add declarative-config/k8s/rs-manager/armor/
   git commit -m "feat(armor): add MEK_RING support for key rotation"
   git push
   ```

4. **Wait for ArgoCD sync** (automatic within 5 minutes)

### Phase 3: Trigger Key Rotation via Admin API

1. **Start kubectl proxy:**
   ```bash
   kubectl proxy --port=8001 &
   PROXY_PID=$!
   ```

2. **Get admin token:**
   ```bash
   # Retrieve admin token from environment (never printed)
   ADMIN_TOKEN=$(bao-as rs-manager -- bao kv get -field=admin_token secret/rs-manager/rs-manager/armor/admin)
   ```

3. **Trigger rotation:**
   ```bash
   curl -X POST \
     -H "Authorization: Bearer $ADMIN_TOKEN" \
     -H "Content-Type: application/json" \
     http://127.0.0.1:8001/api/v1/namespaces/armor/services/armor:9001/proxy/admin/key/rotate
   ```

4. **Monitor rotation progress:**
   ```bash
   # Poll the key ring endpoint to track progress
   watch -n 5 'curl -s -H "Authorization: Bearer $ADMIN_TOKEN" \
     http://127.0.0.1:8001/api/v1/namespaces/armor/services/armor:9001/proxy/admin/key/ring | jq .'
   ```

5. **Wait for completion criteria:**
   - `active_fp` shows the new MEK fingerprint
   - `objects_by_fp` histogram shows 0 objects under old fingerprint
   - All objects now encrypted with new MEK

6. **Cleanup:**
   ```bash
   kill $PROXY_PID
   unset ADMIN_TOKEN
   ```

### Phase 4: Verification

1. **Check canary health:**
   ```bash
   curl -s http://armor.armor.svc.cluster.local:9000/armor/canary | jq .
   ```

2. **Verify an object written before rotation:**
   ```bash
   # Write a test object before rotation (if not already exists)
   # Then verify it still decrypts correctly after rotation
   ```

3. **Check pod logs for rotation completion:**
   ```bash
   kubectl -n armor logs deploy/armor --tail=100 | grep -i rotation
   ```

## Post-Rotation State

### Fingerprints (to be filled in by operator)
- **Old MEK Fingerprint:** TBD (first 8 bytes of SHA-256)
- **New MEK Fingerprint:** TBD (first 8 bytes of SHA-256)
- **Rotation Timestamp:** TBD
- **Objects Rotated:** TBD count
- **Rotation Duration:** TBD

### Retiring Old Keys (Operator Action)

Once verification confirms all objects are on the new key, the old MEK can be removed from the ring:

```bash
# Remove old MEK from ARMOR_MEK_RING in OpenBao
bao-as rs-manager -- bao kv patch secret/rs-manager/backblaze/armor \
  MEK_RING=<new_ring_without_old_key>

# Update escrow
bao-as rs-manager -- bao kv patch secret/rs-manager/escrow/armor \
  mek_ring=<new_ring_without_old_key>

# Trigger deployment restart to pick up new ring
kubectl -n armor rollout restart deployment/armor
```

## References

- **Plan §8.13:** MEK key ring design and rotation procedure
- **Plan §8.1:** Cipher correctness and format migration
- **ADR-005:** CTR keystream reuse vulnerability (why this rotation is critical)
- **docs/disaster-recovery.md:** MEK escrow and backup procedures

## Notes

- The old key STAYS in the ring until explicitly retired by an operator
- This is a design feature to prevent irreversible data loss during rotation
- Any ARMOR instance with the old MEK in its ring can decrypt old objects
- Only the active MEK wraps new DEKs for new uploads
- Rotation is O(N) where N = number of objects in the bucket
- Rotation uses CopyObject metadata operations (no data re-upload)
- Rotation is idempotent and can be safely resumed if interrupted

## Exceptions

If any objects cannot be rotated (e.g., >5GiB CopyObject exception), they will remain encrypted with their current key and appear in the `objects_by_fp` histogram under the old fingerprint. These should be tracked and addressed separately.
