# AGENTS.md

## Working style
- First inspect the repository and summarize the implementation plan before changing code.
- Prefer minimal and backward-compatible changes.
- Do not refactor unrelated modules.
- Reuse existing order, payment, fulfillment, inventory, and route patterns where possible.
- Before finishing, run the project's existing lint / typecheck / test / build commands if available.
- In the final summary, include:
  - changed files
  - API compatibility notes
  - DB migration notes
  - manual test steps
  - risks / follow-up items

## Business requirements
1. Replace "gift card" semantics with "product redemption code".
2. Redeeming a code should redeem a specific product / SKU, not wallet balance.
3. Preferred implementation: redeeming creates a zero-price paid order and reuses the existing order fulfillment flow.
4. Add a public redeem entry on the home page.
5. Keep compatibility where reasonable; avoid breaking old API consumers abruptly.
6. Add a simple "subsite 1.0" model, preferring the least invasive implementation.

## Constraints
- Keep naming and coding style consistent with this repo.
- Preserve existing authentication / captcha / validation patterns.
- Do not remove old fields unless replacement compatibility is clearly handled.
- Add or update tests for changed behavior when the repo already has a test pattern.

- ## Repo-specific scope
- Focus on models, services, controllers, validators, database migrations, API responses, and tests.
- For redemption, prefer creating an order and reusing fulfillment.
