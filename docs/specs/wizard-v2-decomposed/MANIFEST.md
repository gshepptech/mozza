# Wizard v2 Decomposed Spec — Manifest

## Domain Files

| # | Domain File | Description | Dependencies | Wave |
|---|------------|-------------|--------------|------|
| 1 | `01-interview-engine.md` | Core interview engine: question sequencing, skip logic, state management | None | 1 |
| 2 | `02-order-wizard-shell.md` | OrderWizard component shell, step navigation, layout with cart sidebar | 01 | 1 |
| 3 | `03-step1-place-order.md` | US-1: Alias picker, replica count, target toggle, inline alias creation | 01, 02 | 2 |
| 4 | `04-step2-traits.md` | US-2: Trait toggles (web-facing, stateful, worker), skip logic for Local | 01, 02 | 2 |
| 5 | `05-step3-workload-inference.md` | US-3: Recommendation engine, workload type cards, pre-selected defaults | 01, 02, 04 | 2 |
| 6 | `06-step4-networking.md` | US-4: Port input, public toggle, domain field, target-aware display | 01, 02 | 2 |
| 7 | `07-step5-dependencies.md` | US-5: Database/cache/queue toggle cards, auto-add slices, sidebar cart integration | 01, 02 | 3 |
| 8 | `08-step6-configuration.md` | US-6: Env var key-value editor, secret toggle, auto-populated from deps | 01, 02, 07 | 3 |
| 9 | `09-step7-anything-else.md` | US-7: Health checks, resource limits, scaling — expandable defaults | 01, 02 | 3 |
| 10 | `10-service-cart.md` | US-8: Sidebar service cart, multi-service management, add/remove/edit | 02 | 3 |
| 11 | `11-recipe-generation.md` | FR-3: Generate valid .mozza recipe from wizard answers | All steps | 4 |
| 12 | `12-review-editor.md` | US-9: Recipe review, inline editor, YAML toggle, validation | 11 | 4 |
| 13 | `13-deploy-progress.md` | US-10: Place Order, pizza-metaphor progress, error intelligence | 12 | 4 |
| 14 | `14-integration-wiring.md` | Wire OrderWizard into DeployWizard, replace GuidedWizard, route setup | All | 5 |

## Wave Execution Order

- **Wave 1:** Domain 1-2 (engine + shell) — foundation
- **Wave 2:** Domain 3-6 (steps 1-4) — core interview flow
- **Wave 3:** Domain 7-10 (steps 5-7 + cart) — advanced steps + multi-service
- **Wave 4:** Domain 11-13 (recipe gen + review + deploy) — output path
- **Wave 5:** Domain 14 (integration) — wire everything together
