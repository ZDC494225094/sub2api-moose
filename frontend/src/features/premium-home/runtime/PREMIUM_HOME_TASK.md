# Premium Home Runtime Task

## Goal

Create a new production homepage for Sub2API without modifying the existing `HomeView.vue` implementation. The new homepage uses the approved premium style: clean white and blue, glass texture, enterprise tone, responsive navigation, dynamic subscription plan cards, and public announcement timeline.

## Scope

- Keep the legacy homepage at `/home` for rollback.
- Mount the new homepage at `/`.
- Put all new homepage runtime code under `frontend/src/features/premium-home/runtime/`.
- Use existing backend configuration data instead of hardcoded production plans.

## Design Rules

- White is pure white; blue is the main accent.
- Use thin borders, glass blur, controlled shadows, and large whitespace.
- Hero section uses left copy and right AI visual.
- Desktop announcements appear as a timeline card; small screens show an announcement button and modal.
- Plan cards include discount, heat score, buyer count, and an electric highlight to support conversion.
- Navigation collapses to a menu when width is not enough.

## Data Sources

- `GET /api/v1/payment/public/plans`
  - Returns active subscription plans configured for sale.
  - Includes group name/platform, rate multiplier, limits, model scopes, price, original price, validity, and parsed features.
- `GET /api/v1/announcements/public`
  - Returns active announcements whose targeting is empty, meaning "show to everyone".
  - Does not require login and does not return read state.
- Public site settings continue to come from the existing app store public settings loader.

## Implementation Steps

1. Add public backend endpoints for anonymous homepage rendering.
2. Add frontend API wrappers in the runtime folder.
3. Build `PremiumHomeView.vue` as a standalone homepage component.
4. Add isolated CSS in `premium-home.css`, scoped by `.premium-home`.
5. Copy the approved AI hero visual into `runtime/assets/`.
6. Route `/` to the new homepage while keeping `/home` on the legacy homepage.
7. Validate frontend typecheck/build and backend compile when local dependencies are available.

## Acceptance Criteria

- `/` loads without authentication.
- `/home` still opens the legacy homepage.
- Plan cards render from backend plan configuration when the API is available.
- Announcements configured for everyone render on the homepage.
- Small-screen menu and announcement modal do not overflow.
- Existing `frontend/src/views/HomeView.vue` remains untouched.

## Current Optimization Pass

- Header logo must render inside a fixed square mark without stretching or clipping rectangular site logos.
- Hero visual must match the approved design-preview positioning and light treatment.
- Announcement cards show title, date, and plain-text Markdown summary only; clicking opens a glass modal with sanitized Markdown full content.
- Subscription cards use a more compact promotional layout closer to the reference design, with discount, heat, buyers, and electric scan effect.
- Theme control supports three modes: light, dark, and system. System mode follows `prefers-color-scheme` and the app bootstrap understands the `system` stored value.

## Current Data Fidelity Pass

- Header logo image fills the mark container directly; the parent mark has no padding or background.
- Hero visual positioning is restored to the approved design-preview crop/light layout.
- The homepage renders every plan returned by `GET /api/v1/payment/public/plans`; there is no frontend slice limit for subscription cards.
- Discount labels use the API `discount_rate` field, derived from backend `price / original_price`.
- Purchase count uses the API `purchase_count` field, counted from paid/completed subscription orders instead of frontend mock numbers.
