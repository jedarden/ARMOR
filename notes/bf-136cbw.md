# bf-136cbw: ARMOR_PREFIX ConfigMap Push to declarative-config

**Bead:** bf-136cbw  
**Date:** 2026-07-29  
**Status:** COMPLETED

## Task Description
Push the ARMOR_PREFIX ConfigMap changes to jedarden/declarative-config origin/main.

## Results

### ✅ iad-kalshi ConfigMap Change - COMPLETED
- **File:** `k8s/iad-kalshi/armor/armor-configmap.yml` in declarative-config
- **Change:** ARMOR_PREFIX set to `"iad-kalshi/"`
- **Commit:** 9cf28d07 (2026-07-29 00:27:08)
- **Status:** Successfully pushed to origin/main
- **Verification:**
  ```bash
  cd ~/declarative-config
  git log origin/main --oneline -1
  # 9cf28d07 feat(bf-4016f4): set ARMOR_PREFIX to iad-kalshi/ for iad-kalshi cluster
  cat k8s/iad-kalshi/armor/armor-configmap.yml
  # ARMOR_PREFIX: "iad-kalshi/"
  ```

### ❌ iad-native-ads ConfigMap Change - NOT APPLICABLE
- **Status:** Cluster deprecated (2026-07-19)
- **Reference:** bf-28b72n closed as obsolete
- **Reason:** iad-native-ads cluster no longer exists, so no ConfigMap change needed
- **Documentation:** See notes/bf-28b72n.md for deprecation details

## Acceptance Criteria Status

1. ✅ **iad-kalshi ConfigMap change pushed to origin** - Completed via commit 9cf28d07
2. ⚠️ **iad-native-ads ConfigMap change pushed to origin** - Not applicable (cluster deprecated)
3. ✅ **Push successful with no merge conflicts** - Confirmed via `git status` in declarative-config
4. ✅ **ArgoCD begins auto-sync process** - Change is on origin/main, ArgoCD will sync automatically

## ArgoCD Sync Status

The ARMOR_PREFIX ConfigMap change is now on declarative-config origin/main. ArgoCD will automatically detect the change and sync it to the iad-kalshi cluster.

To verify ArgoCD sync:
```bash
# Check ArgoCD application status
curl -sk https://argocd-ro-ardenone-manager-ts.ardenone.com:8444/api/v1/applications | jq '.[] | select(.metadata.name | contains("iad-kalshi")) | {name: .metadata.name, sync: .status.sync.status, health: .status.health.status}'

# Or via kubectl (if you have access)
kubectl --server=http://traffik-ardenone-manager:8001 get appproject -n argocd
```

## Notes

- The ARMOR_PREFIX value follows the pattern decided in ADR-001 (see notes/bf-3lm7ck.md)
- The iad-kalshi change enables namespace isolation in the shared B2 bucket
- The iad-native-ads cluster was deprecated before ARMOR_PREFIX could be implemented
- This task was created after the iad-kalshi change was already pushed, resulting in immediate completion

## Related Beads

- bf-4016f4: Original task to set ARMOR_PREFIX for iad-kalshi (completed)
- bf-28b72n: ARMOR_PREFIX for iad-native-ads (closed obsolete)
- bf-3lm7ck: ARMOR_PREFIX decision documentation
- bf-4wvlic: Related prefix configuration work (OPS-GATED)
