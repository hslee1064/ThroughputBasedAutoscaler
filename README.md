# TBA — Throughput Based Autoscaler

A Kubernetes operator that autoscales ML microservice pipelines based on **data processing throughput** instead of raw CPU/GPU utilization.

This is the reference implementation of the autoscaler proposed in the master's thesis
*"Kubernetes Autoscaling of ML Services Based on Data Processing Throughput"* (ML 서비스의 데이터 처리량 기반 쿠버네티스 오토스케일링 기법), Korea University, 2024.

## Overview

ML services are usually deployed as a pipeline of inference models (e.g. NER → EL → RE → post-processing). When GPU resources in a cluster are limited, models compete for resources and one slow stage becomes a bottleneck that drags down the throughput of the whole pipeline.

Kubernetes' built-in **Horizontal Pod Autoscaler (HPA)** only reacts to instantaneous CPU/GPU usage, so it cannot allocate resources to the stage that actually needs them. TBA instead places a **message queue (Redis Stream)** between every model and scales each model by comparing its measured throughput against the current queue length. This lets it find the bottleneck stage and give it more pods, while returning idle GPUs so other models can use them.

In the thesis experiments TBA improved end-to-end throughput by up to **51.6%** over HPA and increased GPU utilization, with the gap widening as the number of GPUs in the cluster grew.

## How it works

```
 PRE → [queue] → NER → [queue] → EL → [queue] → RE → [queue] → POST
                                            ▲
                                       bottleneck
                                            │
                          ┌─────────────────┴──────────────────┐
                          │            TBA controller           │
                          │  poll queue length + throughput     │
                          │  → pick bottleneck model            │
                          │  → pick node type (GPU first)       │
                          │  → compute desired replicas         │
                          │  → scale deployment                 │
                          └─────────────────────────────────────┘
```

- Each model stage reads from its input **Redis Stream** and writes to the next stage's stream. Pods of one Deployment form a single Redis **consumer group**.
- Each model also publishes its real-time per-device throughput into Redis (`<model>-cpu-throughput`, `<model>-gpu-throughput`).
- The controller runs a polling loop (default every **10s**) that, for every model stream, computes:

  ```
  modelThroughput = cpuThroughput * cpuReplicas + gpuThroughput * gpuReplicas
  ```

  If `modelThroughput < queueLength` the stage is a bottleneck and is **scaled up**; otherwise it is **scaled down**.

### Scaling formula

The desired replica count follows the thesis algorithm, where `CR` = current replicas, `QL` = queue length, `MT` = model throughput, `DT` = per-device throughput, `AR` = available (unallocated) GPUs:

```
ScaleOut:  DR = min( CR + ceil( (QL - MT) / DT ),  CR + AR )
ScaleIn:   DR = max( CR + floor( (QL - MT) / DT ),  0 )
```

- GPUs are allocated and reclaimed first; scale-out can never exceed the available GPUs, and scale-in never drops below 0 (a single pod is kept while a stream still has unprocessed messages).
- Replica count is capped at **20** (`ReplicasLimit`) and the polling interval is **10s** (`SleepInterval`), both in [internal/controller/constants.go](internal/controller/constants.go).
- A `lastScaledTime` label implements a stabilization window to avoid flapping.

The core loop lives in [internal/controller/autoscaler_eventsource.go](internal/controller/autoscaler_eventsource.go).

## Custom Resources

The project defines two CRDs (`api.github.com/v1`):

| Kind         | Purpose                                                                 |
|--------------|-------------------------------------------------------------------------|
| `Autoscaler` | Drives the throughput-based scaling loop (`queue_length`).              |
| `Inference`  | Describes an inference model and the devices it can run on (`devices`). |

Target Deployments are discovered by the label `app: custom-autoscaler`; whether a Deployment is the CPU or GPU variant of a model is inferred from its `nvidia.com/gpu` resource limit.

## Components

The autoscaler is meant to run alongside the observability stack used in the thesis:

- **Redis (Streams)** — message queues between models and live throughput metrics.
- **Prometheus** — metric collection (Pull) from exporters.
- **Node Exporter / DCGM Exporter** — node CPU/memory and GPU utilization.
- **Grafana** — visualization.
- **Rancher Local Path Provisioner** — local PVs for fast model file mounting.

> The Redis connection address/password is currently hard-coded in `PollingRedis` (`localhost:6379`); change it to your in-cluster Redis (e.g. `redis-master.redis.svc.cluster.local:6379`) before deploying.

## Getting Started

### Prerequisites
- go version v1.21.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster (with NVIDIA GPU nodes for the GPU path).
- A reachable Redis instance with Streams.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/custom-autoscaler2:tag
```

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/custom-autoscaler2:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/custom-autoscaler2:tag
```

This generates an `install.yaml` in the `dist` directory containing all the resources built with Kustomize.

2. Users can install the project with:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/custom-autoscaler2/<tag or branch>/dist/install.yaml
```

## Results (from the thesis)

| Test   | GPUs   | HPA       | TBA       | Improvement |
|--------|--------|-----------|-----------|-------------|
| Test A | T4 × 2 | 2.268 rps | 2.483 rps | +9.47%      |
| Test B | T4 × 4 | 3.302 rps | 3.876 rps | +17.38%     |
| Test C | T4 × 6 | 3.985 rps | 5.671 rps | +42.30%     |
| Test D | T4 × 8 | 4.379 rps | 6.639 rps | +51.60%     |

The OASYS knowledge-graph extraction pipeline (NER / EL / RE) over DBpedia text was used as the workload. HPA leaves GPUs idle once a stage's average utilization drops, while TBA keeps reassigning GPUs to the bottleneck stage — so its advantage grows with cluster size.

## Future work

- Proactive autoscaling via time-series forecasting or reinforcement learning.
- GPU virtualization (MIG, vGPU) for finer-grained allocation.
- ML model partitioning for distributed inference.

## Built with Kubebuilder

This project is scaffolded with [Kubebuilder](https://book.kubebuilder.io/introduction.html). Run `make help` for all available `make` targets.

## License

Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
</content>
