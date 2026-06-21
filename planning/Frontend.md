# Frontend Status And Maintenance Guide

Last reviewed against the codebase: June 21, 2026.

## Current Implementation

The frontend is a local React 19, TypeScript, and Vite application. React Router provides the Home,
Learn, Configure, Dashboard, Console, Workloads, Cluster, Eviction, and Benchmarks routes. Zustand
persists the selected API base URL; Recharts and React Flow render metrics and topology.

HTTP and SSE data is structurally validated in `src/api/validate.ts` before it reaches components.
Malformed payloads produce explicit errors instead of being confused with an unavailable node.
Operational routes remain useful when the backend is stopped and recover when it returns.

The interface includes visible keyboard focus, a skip link, named navigation regions, labelled
controls, live-region status where appropriate, text summaries for charts and topology, and reduced
motion behavior. The tested layouts cover 1366x768 laptops, 1920x1080 projectors, and 768x1024
tablets without horizontal overflow.

The Benchmarks page accepts Go benchmark output or JSON and includes a deterministic built-in
sample. `examples/demo-dataset.json` plus `scripts/load-demo-dataset.ps1` provides reproducible live
data for presentations.

## Source Map

| Location | Responsibility |
| --- | --- |
| `src/app` | Application shell and routes |
| `src/api` | HTTP/SSE clients, response types, and runtime validation |
| `src/hooks` | Health, cluster, and event lifecycles |
| `src/store` | Persisted API target |
| `src/pages` | Home, learning chapters, and operational tools |
| `src/components/charts` | Accessible chart wrappers |
| `src/components/cluster` | Cluster topology |
| `src/components/visuals` | Interactive learning diagrams |
| `e2e` | Edge end-to-end and accessibility coverage |

## Validation

```powershell
cd frontend
npm.cmd ci
npm.cmd run lint
npm.cmd run build
npm.cmd run test:e2e
```

The end-to-end suite starts isolated backend and frontend processes. It verifies navigation, API
target changes, offline behavior, malformed response handling, live dashboard values, real command
execution, built-in benchmarks, keyboard focus, reduced motion, WCAG A/AA rules, and responsive
viewports.

## Future Options

- Component tests for focused parser and interaction failures.
- Visual-regression coverage for presentation-critical routes.
- Phone-sized layout work if mobile becomes a supported target.
- A generated API contract shared with Go response models.
- A controlled side-by-side eviction comparison if it adds value to the diploma demonstration.
