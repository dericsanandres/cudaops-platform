# Initial service objectives

These are operational targets for a future Kubernetes environment, not measured claims about the current local deployment. Establish a baseline before enforcing them as release gates.

| Objective | Target | Measurement |
| --- | --- | --- |
| API readiness | 99.5% per calendar month | Successful `GET /readyz` probes from the cluster monitoring system. |
| Job completion | 99% per calendar month | `succeeded / (succeeded + failed)` from `cudaops_jobs_total`. Exclude client validation failures from a later, explicitly defined calculation. |
| GPU scheduling | 99% of `device=cuda` requests start on a GPU-capable worker | Job status and Kubernetes scheduling events. Explicit CUDA failure remains correct when no GPU capacity is available. |
| Recovery | Reclaim interrupted jobs within 90 seconds | Redis pending-entry age and job attempt history. The application reclaims after 60 seconds. |

## Error-budget policy

For the API-readiness target, the monthly error budget is 0.5% of the observed monitoring window. Pause non-critical deployment changes when that budget is exhausted, investigate through the relevant runbook, and resume only after the service is stable.

## Review cadence

Review objectives after the first sustained Kubernetes deployment and after material changes to job persistence, worker concurrency, GPU topology, or retry behavior. Record the query definitions, monitoring coverage, exclusions, and actual results with each revision.
