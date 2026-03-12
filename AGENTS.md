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

## Persistent business rules for self-service paid subsite 1.0

- Goal: help the platform sell more products through subsites and referrals.
- Keep implementation lightweight and shippable. Do NOT do a full multi-tenant rewrite.
- One user can own at most one site in v1.
- Site opening is paid and automatic after successful payment.
- Site opening price and supported domain suffixes are configured in admin.
- User chooses:
  - site name
  - subdomain prefix
  - one supported suffix
- Final domain = prefix + selected suffix.
- Validate prefix format, reserved prefixes, prefix uniqueness, and full domain uniqueness.
- Site owners do NOT control product catalog, stock, fulfillment, or order detail content.
- Site owners can only:
  - configure site basic info
  - set site sale price for products
  - see order list summaries for their own site
  - see profit ledger and apply for withdraw
- Site sale price must be >= supply/base price.
- For site product orders:
  - use site price for checkout
  - snapshot base_price, site_price, and site_profit on order creation
  - do NOT combine site profit and affiliate commission on the same product order
- For site-opening orders:
  - affiliate commission is allowed
- No cookie-based site binding.
- Site attribution should come from host/domain or explicit referral flow only.
- Gift-card/redeem zero-price orders must not generate site profit.
- Reuse affiliate withdraw flow patterns where helpful, but use separate site profit and site withdraw tables.
- Keep admin and user accounts globally shared.
