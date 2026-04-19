# Swarmex Affinity

Service affinity and anti-affinity via placement constraints for Docker Swarm.

Part of [Swarmex](https://github.com/ccvass/swarmex) — enterprise-grade orchestration for Docker Swarm.

## What It Does

Controls service placement by co-locating related services on the same node, spreading replicas across nodes, or keeping conflicting services apart. Translates high-level labels into Docker placement constraints.

## Labels

```yaml
deploy:
  labels:
    swarmex.affinity.colocate: "svc-cache"   # Place on same node as this service
    swarmex.affinity.avoid: "svc-other-db"   # Place on different node than this service
    swarmex.affinity.spread: "true"          # Spread replicas across all available nodes
```

## How It Works

1. Watches for services with affinity labels via Docker events.
2. Resolves the target service's current node placement.
3. Applies placement constraints: same node for colocate, different node for avoid.
4. For spread, adds constraints to distribute replicas across available nodes.
5. Runs once per event — no polling loops.

## Quick Start

```bash
docker service update \
  --label-add swarmex.affinity.colocate=svc-cache \
  my-app
```

## Verified

Colocate placed services on the same node. Avoid placed services on different nodes. 2 log entries confirmed — no reconciliation loop.

## License

Apache-2.0
