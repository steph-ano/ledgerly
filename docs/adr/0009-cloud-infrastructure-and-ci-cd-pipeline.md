# ADR-0009: Cloud Infrastructure, Kubernetes Deployment, and Multi-Stage CI/CD

## Status
Accepted

## Context
As a financial core processing money and BNPL installments, Ledgerly requires an enterprise-grade infrastructure posture and automated continuous integration pipeline. The system must ensure zero regressions in financial invariants (zero-sum balances, idempotency, race conditions, memory safety) before any code reaches production.

Additionally, production deployments require high availability, zero-downtime rolling updates, Horizontal Pod Autoscaling (HPA) under surge traffic (e.g., Black Friday retail sales), and database isolation.

## Decision
1. **Continuous Integration (GitHub Actions)**:
   - **Multi-Matrix CI**: Runs on every pull request and push to `main`.
   - **Go Concurrency & Memory Race Auditing**: Mandates `-race` flag across all Go unit and integration tests.
   - **Property-Based Invariant Gates**: Rapid property tests must pass 100 iterations of random monetary split generation.
   - **Real Service Containers**: CI spins up a dedicated PostgreSQL 16 container for end-to-end integration tests.
   - **Frontend & Mobile Validation**: Strict TypeScript compilation (`tsc -b`, `tsc --noEmit`) and production bundling for both Web and Mobile apps.
   - **Container Builds**: Automated multi-stage Docker builds with Trivy vulnerability scanning.

2. **Infrastructure as Code (Terraform / OpenTofu)**:
   - Modular cloud topology targetable to AWS / GCP:
     - **VPC & Networking**: Public subnets (NAT Gateways, ALBs) and isolated Private Subnets for computing nodes and databases.
     - **Relayed RDS PostgreSQL**: Multi-AZ deployment, automated daily snapshots, KMS storage encryption, and private subnet isolation.
     - **Kubernetes Cluster (EKS/GKE)**: Managed control plane, node auto-scaling groups, and IAM Roles for Service Accounts (IRSA) enforcing least privilege.

3. **Orchestration & Kubernetes Packaging (Helm)**:
   - Dedicated Helm chart (`deploy/helm/ledgerly`) separating:
     - `ledger-api`: Scaled deployment for synchronous double-entry transactions.
     - `bnpl-api`: Scaled deployment for consumer orders and payments.
     - `bnpl-worker`: Dedicated asynchronous background worker executing installment leasing (`SELECT ... FOR UPDATE SKIP LOCKED`) and outbox webhook dispatching.
     - `web`: Nginx-backed SPA deployment.
   - Resiliency controls:
     - `PodDisruptionBudget` (PDB) guaranteeing minimum available pods during cluster maintenance.
     - `HorizontalPodAutoscaler` (HPA) scaling pods dynamically based on CPU/Memory utilization.
     - `Liveness` and `Readiness` probes bound to `/healthz` and `/readyz`.

## Consequences
- Guarantees fintech reliability through automated pre-merge gates.
- Ready for one-command deployment to any cloud Kubernetes environment.
