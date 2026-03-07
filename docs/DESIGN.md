# DESIGN.md

UI provides a consistent visual system across Auth, Document, QA, and Settings pages.

## Implemented design language
- Typography: `Space Grotesk`-first stack with strong heading contrast.
- Color system: teal/graphite palette via CSS variables (no default purple scheme).
- Background: layered radial gradients + subtle glass panel treatment.
- Motion:
  - page panel rise/fade entry animations,
  - button hover lift for key actions.
- Components:
  - pill tabs,
  - status chips (`ready/indexing/failed`),
  - citation cards and provider chips.

## UX requirements covered
- Clear indexing/upload feedback messages.
- Citations always visible in QA turn cards.
- Scope controls (`@all` / `@doc`) placed in query composer.
- Ownership-visible document table tied to current authenticated user.
- Deployed app uses same-origin API route (`/api`) through frontend gateway proxy, avoiding localhost-target confusion on client devices.

## Constraint compliance
- Prioritizes clarity over ornament.
- Responsive behavior included for <=768px viewports.
