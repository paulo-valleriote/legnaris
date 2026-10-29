# The broker image the stack runs: the pinned release with its Prometheus
# plugin switched on at build time, so the plugin is active before the broker
# ever starts and the health check never races a broker still reconfiguring
# itself (BRK-06, BRK-14). Bump the tag here and nowhere else; the deployment's
# checks read this line.
FROM rabbitmq:4.4.0-management

RUN rabbitmq-plugins enable --offline rabbitmq_prometheus

COPY provision.sh /usr/local/bin/legnaris-provision
COPY entrypoint.sh /usr/local/bin/legnaris-entrypoint
ENTRYPOINT ["legnaris-entrypoint"]
CMD ["rabbitmq-server"]
