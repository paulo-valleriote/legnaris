# Local Kubernetes (k3d)

Kustomize manifests (`base/` + `overlays/local/`) that run the whole stack in a
k3d cluster (k3s inside Docker).

```bash
make k8s-up     # cluster + images + manifests; generates overlays/local/secrets.env
make k8s-down   # destroys the cluster and its volumes
```

Requires `docker`, `k3d`, `kubectl` and `openssl`.

- API: <http://api.legnaris.localhost:8080> — Web: <http://app.legnaris.localhost:8080>
  (`*.localhost` resolves to loopback; otherwise `curl --resolve`).
- Secrets are never committed: `secrets.env` is gitignored and generated with
  random values; `secrets.env.example` shows the keys.
- Images are built locally and imported (`legnaris-{app,web,rabbitmq}:local`).
  To run CI images, change the `images:` block in the overlay to
  `ghcr.io/<owner>/legnaris-app:sha-<commit>` (see `.github/README.md`).

> **Status:** validated on k3d (Docker Desktop/WSL2, k3d v5.9, k3s 1.35).
> Measured: API and web through the Ingress; register/login; killing one `app`
> pod (267/267 requests 200); `rollout restart` under continuous requests
> (630/630 200); OTP e-mails with 2 `relay` and 2 `worker` replicas, each message
> delivered once; relay/worker exit within ~2 s of SIGTERM. A first-boot bug was
> found and fixed (MongoDB liveness probe killing the init; now gated by a
> startup probe). Not yet run: the e2e sign-in from a browser.

## Checks to run

```bash
kubectl get pods                                  # 2x app, web, relay, worker; mongodb-0, rabbitmq-0, redis
kubectl delete pod -l app=app --wait=false        # service keeps answering (2 replicas + PDB)
# rolling update without 5xx: loop requests while restarting
( while true; do curl -s -o /dev/null -w '%{http_code}\n' http://api.legnaris.localhost:8080/ready; sleep 0.1; done ) &
kubectl rollout restart deployment/app && kubectl rollout status deployment/app
# e-mail OTP flow (provider `log`): outbox -> relay -> queue -> worker
kubectl logs deploy/worker
```

Sign-in cookies are `Secure`; Chrome, Edge and Firefox treat `*.localhost` as a
secure context. Both origins must stay in `CORS_ALLOWED_ORIGINS`/`API_BASE_URL`
(`overlays/local`).

## Design notes

- **Rollouts:** `maxUnavailable: 0`, `maxSurge: 1`, readiness (`/ready`) gates
  each step, a 5 s `preStop` sleep lets the endpoint leave the Service before
  SIGTERM, and `terminationGracePeriodSeconds: 30` exceeds preStop +
  `SHUTDOWN_TIMEOUT` + DB close.
- **relay/worker:** same image as the API with `command: ["/relay"]` /
  `["/worker"]`, two replicas each. The relay leases outbox rows and the worker
  consumes from a queue, so replicas do not process a message twice (to be
  confirmed by the checks above).
- **Ingress:** Traefik (k3s default); `api.` and `app.` hosts.

## Compose parity — what is left out

oauth2-proxy and the `ops.` domain, Cloudflare tunnel, TLS/Let's Encrypt, Mongo
Express, the Mongo exporter and the whole observability stack (Prometheus,
Grafana, Loki, Alloy — see LGN-133). RabbitMQ's console is not exposed.

## Primer

- **Pod** — smallest runnable unit: one or more containers sharing network and
  storage. Disposable; never created by hand.
- **Deployment** — keeps N identical Pods running and performs rolling updates.
- **Service** — stable name and virtual IP load-balancing across the ready Pods
  behind a label selector.
- **Ingress** — HTTP routing from outside the cluster (host/path) to Services;
  Traefik implements it in k3s.
- **StatefulSet** — like a Deployment but with stable identity (`mongodb-0`) and
  per-replica storage; used for databases and brokers.
- **PVC** — a claim for persistent storage; StatefulSets create one per replica
  (`volumeClaimTemplates`) so data survives Pod restarts.

## Observability (LGN-133)

```bash
make k8s-observability     # helm: kube-prometheus-stack (pinned) + PodMonitors, rules, dashboards
kubectl -n monitoring port-forward svc/kps-grafana 3000:80    # http://localhost:3000 (admin/admin, local only)
```

- `deploy/k8s/observability/values.yaml` — Helm values (remote-write receiver on
  for k6, selectors open so objects outside the release are picked up).
- `podmonitors.yaml` — one PodMonitor per workload, so **every replica** is a
  target; `job` is relabelled to the compose names so rules and dashboards work
  unchanged.
- `prometheusrule.yaml` — **generated** from `deploy/prometheus/rules.yml`
  (`make k8s-rules`); a Go test fails on drift.
- Dashboards (`deploy/grafana/dashboards/*.json`) are loaded by Grafana's
  sidecar from labelled ConfigMaps.
- Loki + Alloy for logs are not installed (optional in the ticket).
- Validated on k3d: all replicas are targets (app 2, relay 2, worker 2,
  rabbitmq 1), the 11 rules load `ok`, the three dashboards are imported with
  datasource uid `prometheus`, and `EmailWorkQueueHasNoConsumer` fired ~150 s
  after the worker was scaled to 0.

## Autoscaling and load tests (LGN-134, LGN-135)

- **HPA** (`base/hpa.yaml`): the app scales on CPU (70 % of `requests.cpu`),
  2–6 replicas, 120 s scale-down window (short for the local cluster).
  `requests.cpu` (100m) is a starting guess: calibrate it from the k6 capacity
  result. A custom metric (`legnaris_http_in_flight_requests`/RPS) needs
  prometheus-adapter and is deferred until the CPU result is known.
- **KEDA** for worker and relay: see `keda/README.md` (`make k8s-keda`).
- **Load test:** `make k8s-loadtest SCRIPT=capacity.js` (or `auth-storm.js`, ...)
  applies `overlays/loadtest` — **disposable cluster only**: it lifts the rate
  limits and runs k6 as a Job pushing results to Prometheus, so the loadtest
  dashboard can show replicas × CPU × p95. Watch scaling with
  `kubectl get hpa app -w`.
- **MongoDB bottleneck:** the Mongo exporter is not deployed in k8s; track the
  `mongodb-0` pod's CPU/memory with the kube-prometheus-stack container
  dashboards during the run and record where the p95 breaks.
- **bcrypt:** login cost scales with CPU, so `auth-storm.js` is the run that
  shows whether the HPA reacts fast enough; record the result here.

### Measured (k3d, laptop, `capacity.js`, 6 replicas max, requests 100m / limit 500m)

| Step (req/s) | p95 | Failures |
|---|---|---|
| 50 / 100 / 200 | 23 / 15 / 25 ms | 0 % |
| 400 | 44 ms | 0 % |
| 800 | 208 ms (budget 200) | 1.8 % (PATCH) |

The HPA went from 2 to 6 replicas under the ramp (CPU 102–135 % of request at
the cap) and back to 2 about 5 minutes after the load ended. The budget holds
up to 400 req/s on this setup; at 800 the cap of 6 replicas was reached and the
budget was breached, so the limit is documented here rather than hidden. k6 shares
the host's CPU with the cluster, which lowers the figure. The non-200 answers at
800 req/s were not broken down by status, and the Mongo bottleneck and the
`auth-storm.js` run were not examined.
