# Custom Kubernetes Scheduler

A production-grade custom Kubernetes scheduler written in Go that implements three pluggable scheduling policies — **Bin Packing**, **Load Balancing**, and **Affinity** — plus a **Hybrid** mode that blends all three.

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                   Kubernetes API Server                   │
└───────────────────────┬─────────────────────────────────┘
                        │  Watch (Pods with schedulerName)
                        ▼
┌─────────────────────────────────────────────────────────┐
│                   Custom Scheduler                        │
│                                                          │
│  ┌──────────┐    ┌──────────┐    ┌───────────────────┐  │
│  │  Pod     │───▶│  Filter  │───▶│   Score (Policy)  │  │
│  │  Queue   │    │  Phase   │    │                   │  │
│  └──────────┘    └──────────┘    │ ┌───────────────┐ │  │
│                                  │ │  Bin Packing  │ │  │
│                                  │ ├───────────────┤ │  │
│                                  │ │Load Balancing │ │  │
│                                  │ ├───────────────┤ │  │
│                                  │ │   Affinity    │ │  │
│                                  │ ├───────────────┤ │  │
│                                  │ │    Hybrid     │ │  │
│                                  │ └───────────────┘ │  │
│                                  └─────────┬─────────┘  │
│                                            │             │
│                                   ┌────────▼──────────┐ │
│                                   │  Bind (best node) │ │
│                                   └───────────────────┘ │
│                                                          │
│  Prometheus /metrics  ·  /healthz  ·  /readyz            │
└─────────────────────────────────────────────────────────┘
```

---

## Scheduling Policies

### 1. Bin Packing (`--policy=bin-packing`)

Maximises node utilisation by preferring already-loaded nodes.

**Score formula:**
```
score = w_cpu × (usedCPU + reqCPU) / allocCPU
      + w_mem × (usedMem + reqMem) / allocMem
```

| Config key              | Default | Description                                      |
|------------------------|---------|--------------------------------------------------|
| `CPUWeight`            | 0.6     | Weight for CPU utilisation component             |
| `MemoryWeight`         | 0.4     | Weight for memory utilisation component          |
| `TargetUtilization`    | 0.85    | Hard cap — nodes above this are excluded         |
| `FirstFitDecreasing`   | true    | Sort candidates tightest-first                   |

**Best for:** Batch jobs, CI runners, cost optimisation.

---

### 2. Load Balancing (`--policy=load-balancing`)

Spreads workloads evenly to minimise hotspots.

**Score formula:**
```
score = w_cpu    × (1 - usedCPU/allocCPU)
      + w_mem    × (1 - usedMem/allocMem)
      + w_pods   × (1 - podCount/maxPods)
```

| Config key           | Default | Description                             |
|---------------------|---------|------------------------------------------|
| `CPUWeight`         | 0.4     | Weight for free CPU                      |
| `MemoryWeight`      | 0.3     | Weight for free memory                   |
| `PodCountWeight`    | 0.3     | Weight for free pod slots                |
| `MaxPodsPerNode`    | 110     | Node pod capacity ceiling                |

**Best for:** Latency-sensitive microservices, long-running daemons.

---

### 3. Affinity (`--policy=affinity`)

Respects pod affinity/anti-affinity and topology spread constraints.

**Score components:**
- **Preferred node affinity terms** — bonus per matching expression
- **Pod affinity** — bonus for co-locating with matching pods
- **Pod anti-affinity** — penalty for nodes already running conflicting pods
- **Topology spread** — prefers under-populated topology domains (zone/rack)

**Best for:** Stateful services, HA deployments, cache-adjacent workloads.

---

### 4. Hybrid (`--policy=hybrid`)

Combines all three policies. The blend is driven by a pod annotation:

| Annotation value       | Bin   | LB    | Affinity |
|-----------------------|-------|-------|----------|
| `bin-packing`          | 70 %  | 20 %  | 10 %     |
| `load-balancing`       | 10 %  | 70 %  | 20 %     |
| `affinity`             | 10 %  | 20 %  | 70 %     |
| *(none)*               | 33 %  | 33 %  | 34 %     |

```yaml
annotations:
  scheduler.k8s.custom/hint: bin-packing
```

---

## Scheduling Pipeline

```
Pod added to queue
      │
      ▼
Re-fetch pod (guard against race conditions)
      │
      ▼
List all nodes
      │
      ▼
Filter Phase  ──────────────────────────────────────────────────────
│  ✓ Node is Ready                                                  │
│  ✓ Node is schedulable                                            │
│  ✓ Sufficient CPU + Memory (resource sum of existing pods)        │
│  ✓ Node selector labels match                                     │
│  ✓ Taints / tolerations satisfied                                 │
└─────────────────────────────────────────────────────────────────
      │
      ▼
Score Phase  (active policy)
      │
      ▼
Select highest-scoring node
      │
      ▼
Bind  (POST /api/v1/namespaces/{ns}/pods/{name}/binding)
      │
      ▼
Emit Kubernetes Event
      │
      ▼
Record Prometheus metrics
```

---

## Metrics

| Metric                                       | Type      | Description                          |
|---------------------------------------------|-----------|--------------------------------------|
| `custom_scheduler_scheduled_pods_total`      | Counter   | Total pods successfully scheduled    |
| `custom_scheduler_scheduling_errors_total`   | Counter   | Total scheduling failures            |
| `custom_scheduler_scheduling_duration_seconds` | Histogram | End-to-end scheduling latency      |
| `custom_scheduler_node_score{node, policy}` | Gauge     | Per-node scores per scheduling cycle |

---

## Quick Start

### Prerequisites

- Go 1.21+
- `kubectl` configured with cluster access
- Docker (for container builds)

### Run locally against a cluster

```bash
# Clone
git clone https://github.com/your-org/custom-k8s-scheduler
cd custom-k8s-scheduler

# Build
go build -o bin/scheduler ./cmd/scheduler

# Run (uses ~/.kube/config)
./bin/scheduler \
  --scheduler-name=custom-scheduler \
  --policy=bin-packing \
  --log-level=debug
```

### Deploy to cluster

```bash
# Build and push image
docker build -t ghcr.io/your-org/custom-scheduler:latest .
docker push ghcr.io/your-org/custom-scheduler:latest

# Apply RBAC + Deployment
kubectl apply -f deploy/scheduler.yaml

# Verify
kubectl -n custom-scheduler get pods
kubectl -n custom-scheduler logs -f deploy/custom-scheduler

# Schedule a test pod
kubectl apply -f deploy/example-pods.yaml
kubectl get events --field-selector reason=Scheduled
```

### Switch policies at runtime

```bash
kubectl -n custom-scheduler patch configmap custom-scheduler-config \
  --patch '{"data":{"policy":"load-balancing"}}'
# Restart scheduler pods to pick up new policy
kubectl -n custom-scheduler rollout restart deploy/custom-scheduler
```

---

## Project Structure

```
k8s-scheduler/
├── cmd/scheduler/
│   └── main.go               # Entry point, flag parsing, wiring
├── pkg/
│   ├── config/
│   │   └── config.go         # All tunables with safe defaults
│   ├── scheduler/
│   │   └── scheduler.go      # Core loop: filter → score → bind
│   ├── policies/
│   │   ├── policy.go         # Policy interface + factory
│   │   ├── bin_packing.go    # Bin-packing algorithm
│   │   ├── load_balancing.go # Load-balancing algorithm
│   │   ├── affinity.go       # Affinity + topology spread
│   │   └── hybrid.go         # Weighted blend of all three
│   └── metrics/
│       └── server.go         # Prometheus HTTP server
├── deploy/
│   ├── scheduler.yaml        # Namespace, RBAC, Deployment, Service
│   └── example-pods.yaml     # One example pod per policy
├── Dockerfile                # Distroless multi-stage build
├── go.mod
└── README.md
```

---

## Configuration Reference

```
--scheduler-name   Name registered with Kubernetes (default: custom-scheduler)
--policy           bin-packing | load-balancing | affinity | hybrid
--metrics-addr     Prometheus scrape endpoint (default: :9090)
--log-level        debug | info | warn | error (default: info)
--resync-period    Informer full resync interval (default: 30s)
--kubeconfig       Path to kubeconfig; empty = in-cluster auth
```

---

## Tech Stack

| Layer            | Technology                    |
|-----------------|-------------------------------|
| Language         | Go 1.21                       |
| Kubernetes SDK   | client-go v0.28               |
| Metrics          | Prometheus client_golang v1.17|
| Logging          | Uber Zap                      |
| Container        | Distroless (gcr.io/distroless)|
| Orchestration    | Kubernetes 1.28+              |

---

## License

MIT © 2024
