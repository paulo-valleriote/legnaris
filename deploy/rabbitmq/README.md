# Broker image

`Dockerfile` derives the stack's broker from the pinned `rabbitmq:4.x.y-management`
release and enables the `rabbitmq_prometheus` plugin while the image is built.

- **Why at build time:** enabling the plugin in the service's command would make
  the broker's first start mutate itself while the health check is already
  racing it. Built in, the plugin is on before the broker starts (BRK-14).
- **Why an exact tag:** the image defines the queue format and the plugin set.
  A floating tag lets a deploy silently replace the broker holding every queued
  code (BRK-06). The deployment's checks (`go test ./deploy/...`) fail on a
  floating tag.
- **`rabbitmq.conf`:** mounted by the stack as `conf.d/20-legnaris.conf`. It
  moves the console under `/rabbitmq` and states the metrics port.

- **`entrypoint.sh` and `provision.sh`:** the image starts the broker through
  the wrapper, which runs the provisioning beside it on every start (the
  delivery vhost, the operator's console account, the application's rights).
  They run in the broker's container because `rabbitmqctl` reaches the broker
  by its node name, which no other container knows. Documented in
  `../README.md` ("RabbitMQ provisioning").
