# KEDA (LGN-135)

```bash
make k8s-keda      # installs KEDA (pinned chart) and applies the ScaledObjects
```

Requires `make k8s-observability` first (the triggers query Prometheus).

| | HPA (`base/hpa.yaml`) | KEDA (`scaledobjects.yaml`) |
|---|---|---|
| Scales on | CPU/memory of the pods (metrics-server) | External signals: queue depth, Prometheus queries, cron… |
| Fits | Request-serving workloads whose load shows up as CPU (the API) | Workers whose load is a backlog that CPU does not show until it is too late |
| Scale to zero | No (min 1) | Yes, if the alerts allow it |
| Mechanism | Built in | Operator that creates and drives an HPA per ScaledObject |

Use the HPA for the API and KEDA for the worker and relay. Do not put both on
the same Deployment.

- **worker:** `rabbitmq_queue_messages_ready` for `email.otp`, 20 messages per replica, 1–6.
- **relay:** `legnaris_email_outbox_backlog` (max across replicas), 100 per replica, 1–4.
- **Scale-down damping:** 120 s HPA stabilisation window. KEDA's `cooldownPeriod`
  and `pollingInterval` only matter when scaling to/from zero, so they are not set.
- **Zero replicas:** not used (min 1). `EmailWorkQueueHasNoConsumer` alerts on a
  consumer-less queue for 2 minutes; scaling to zero would hold it firing. To
  enable scale-to-zero, first change that rule to also require
  `rabbitmq_queue_messages_ready > 0`.
- **No message lost or duplicated:** relies on the worker acknowledging after
  delivery and cancelling its consumer on SIGTERM so unacked messages return to
  the queue (see LGN-132 checks); verify during a scale-down.

Validated on k3d: both ScaledObjects reach `Ready` and the triggers read the
Prometheus series (`keda-hpa-worker` 0/20, `keda-hpa-relay` 0/100). Not
exercised: scale-up under backlog — with the `log` provider the worker drains
`email.otp` instantly, so it needs a deliberately slow provider or a paused
worker to build a queue.
