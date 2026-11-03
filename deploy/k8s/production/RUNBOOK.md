# Production on k3s — runbook (LGN-136, LGN-137)

> **Status:** written from the repository only. Nothing here has been executed
> against a VPS or a cluster; rehearse every step on k3d first (section 3).
> Cutover, DNS changes and real credentials are operator actions.

## 1. VPS and k3s

```bash
# Firewall: SSH, HTTP/S, and the API server only from your own address.
ufw default deny incoming && ufw allow 22/tcp && ufw allow 80/tcp && ufw allow 443/tcp
ufw allow from <your-ip> to any port 6443 proto tcp && ufw enable

curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=v1.36.5+k3s1 \
  sh -s - server --secrets-encryption --write-kubeconfig-mode 600
```

Also: key-only SSH, unattended security updates, and keep k3s' bundled Traefik
(the manifests use its CRDs and ingress class).

## 2. Platform, secrets, GitOps

```bash
make k8s-platform        # cert-manager, Sealed Secrets, ClusterIssuer (edit the e-mail first)
```

Back up the Sealed Secrets key **now** and store it off the VPS:
`kubectl -n kube-system get secret -l sealedsecrets.bitnami.com/sealed-secrets-key -o yaml`.

Create the plain env files (gitignored): `overlays/production/secrets.env`
(see `overlays/local/secrets.env.example` for keys), `backup-s3.env`
(`S3_ENDPOINT`, `S3_BUCKET`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`),
`overlays/production-ops/oauth2-proxy.env` (`OAUTH2_PROXY_CLIENT_ID`,
`OAUTH2_PROXY_CLIENT_SECRET`, `OAUTH2_PROXY_COOKIE_SECRET`,
`OAUTH2_PROXY_GITHUB_USER`) and `grafana-admin.env` (`admin-user`,
`admin-password`). Then `scripts/k8s-seal-secrets.sh`, review and commit the
`sealed-*.yaml` files.

Before the first sync: merge a `gitops-bump` PR (or edit `newTag` by hand) so the
overlay points at real `sha-` tags, and decide `EMAIL_PROVIDER` — the base
defaults to `log`, which sends no e-mail; production needs `resend` plus
`RESEND_API_KEY`/`EMAIL_FROM` in the sealed secret and the ConfigMap.

```bash
make k8s-observability   # add -f deploy/k8s/observability/values-production.yaml for production
make k8s-keda
make k8s-gitops          # Argo CD + the two Applications
```

## 3. Rehearse the data migration on k3d

```bash
# Dump from the compose MongoDB (business database only; users come from the init script)
docker compose -f deploy/docker-compose.yml exec -T mongodb sh -c \
  'mongodump -u root -p "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin --db legnaris --archive --gzip' > legnaris.archive.gz
# Restore into the cluster
kubectl exec -i mongodb-0 -- sh -c \
  'mongorestore -u root -p "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin --archive --gzip --drop' < legnaris.archive.gz
```

Check sign-in and a sheet read, then record the duration: it is the maintenance
window's length.

## 4. Cutover (maintenance window)

1. Lower the DNS TTL of `api.`, `app.`, `ops.` a day ahead.
2. Stop writes: `docker compose stop app relay worker web` (keep `mongodb` up).
3. Final dump from compose, restore into the cluster (section 3, on the VPS).
4. Point the three DNS records at the k3s node; wait for certificates
   (`kubectl get certificate`).
5. Verify: `https://api.legnaris.com.br/ready`, the web client, sign-in, an
   OTP e-mail, `https://ops.legnaris.com.br/grafana` behind GitHub login.
6. Keep the compose stack **stopped, not removed**, with its volumes, as the
   fallback for an agreed period.

## 5. Rollback to compose

1. Scale the cluster app down (`kubectl scale deploy app relay worker --replicas=0`
   — pause the Argo CD `selfHeal` first, or it reverts this).
2. Dump the cluster database and restore it into the compose MongoDB (the
   reverse of section 3); data written since cutover would otherwise be lost.
3. `docker compose start`, move DNS back, verify as in step 4.5.

## 6. Backups

`mongodb-backup` CronJob (03:00) uploads to the bucket. Set a bucket lifecycle
rule for retention. Verify restores regularly:

```bash
kubectl apply -f deploy/k8s/backup/restore-verify.yaml && kubectl logs -f job/mongodb-restore-verify
```

## 7. Known gaps

- `ops.` exposes Grafana only; Prometheus, RabbitMQ console and Mongo Express
  are reachable by `kubectl port-forward`.
- Single node, single Mongo: no high availability. The PodDisruptionBudget and
  replicas protect against pod loss, not node loss.
- Compose features not ported: Cloudflare tunnel, Loki/Alloy log pipeline.
