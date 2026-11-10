# 0004 — GitOps with Argo CD and Sealed Secrets

Status: superseded (024). The Kubernetes/GitOps stack was removed; production runs on the Hostinger VPS through the CD workflow and `deploy/vps/deploy.sh`. Kept for the record of the decision.

## Decision

Argo CD syncs `deploy/k8s/overlays/production` (and `production-ops`) from `main`;
secrets live in Git as Sealed Secrets.

## Why

- **Argo CD over Flux:** a UI that shows drift and sync state for a one-person
  team, and first-class `selfHeal`/`prune`. Flux is lighter, but its
  strengths (multi-tenancy, image automation) are not needed here.
- **Sealed Secrets over SOPS:** the controller decrypts in-cluster, so there is
  no key distribution to CI or Argo CD (SOPS would need Argo CD plugin setup).
  Cost: the sealing key must be backed up, or every secret is re-sealed after a
  cluster rebuild.
- **Image updates by PR:** `gitops-bump.yml` pins the overlay to the immutable
  `sha-<commit>` tag. Merging deploys, `git revert` rolls back; no write access
  from the cluster to Git.

## Consequences

- Manual `kubectl` edits are reverted by `selfHeal`.
- Stateful data (Mongo PVC) is not rolled back by `git revert`; see the backup
  and restore procedure in `deploy/k8s/production/RUNBOOK.md`.
