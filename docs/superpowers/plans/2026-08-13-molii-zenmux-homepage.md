# Molii Model Platform Homepage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the default Molii home page with ZenMux-inspired information density and motion while presenting only Molii's real LLM, image, and video capabilities.

**Architecture:** Keep the existing custom `HomePageContent` override untouched. The default page consumes the public pricing catalog once, converts it through a pure home-catalog adapter, and passes that result to focused sections for search, live counts, vendor marquees, capabilities, latest models, and getting-started content.

**Tech Stack:** React 19, TypeScript, TanStack Router/Query, Tailwind CSS, i18next, Bun test, Rsbuild type checking.

## Global Constraints

- Use the current Serif typography and charcoal design tokens.
- Do not restore the theme switcher.
- Do not change `/playground` or send paid requests from the home page.
- Use only enabled models returned by public `/api/pricing`; do not maintain a second static model/vendor catalog.
- Preserve custom URL/HTML/Markdown home page overrides exactly.
- Do not add testimonials, insurance claims, fabricated usage metrics, or unsupported routing claims.
- Respect `prefers-reduced-motion` for all looping and entrance animation.
- Local development only: do not run a production build or add deployment configuration.

---

### Task 1: Public home catalog adapter

**Files:**
- Create: `web/src/features/home/lib/home-model-catalog.ts`
- Create: `web/src/features/home/lib/__tests__/home-model-catalog.test.ts`

**Interfaces:**
- Consumes: `PricingModel[]` and `PricingVendor[]` from `usePricingData()`.
- Produces: `buildHomeModelCatalog(models, vendors)` and `searchHomeModels(models, query, limit)`.

- [ ] **Step 1: Write failing tests for vendor deduplication, live counts, capability categories, release-date ordering, missing-date exclusion, and normalized search.**
- [ ] **Step 2: Run `cd web && bun test src/features/home/lib/__tests__/home-model-catalog.test.ts` and confirm the missing module failure.**
- [ ] **Step 3: Implement the smallest pure adapter that derives vendors, counts, capability categories, and the latest three dated models.**
- [ ] **Step 4: Implement normalized model/vendor/description/capability search and cap suggestions at the requested limit.**
- [ ] **Step 5: Run the focused test and confirm it passes.**

### Task 2: Search-first hero and live catalog overview

**Files:**
- Modify: `web/src/features/home/components/sections/hero.tsx`
- Modify: `web/src/features/home/components/sections/stats.tsx`
- Create: `web/src/features/home/components/model-search.tsx`
- Create: `web/src/features/home/components/vendor-marquee.tsx`
- Create: `web/src/features/home/components/__tests__/home-interactions.test.tsx`

**Interfaces:**
- Consumes: `HomeModelCatalog`, model list, loading/error state, and docs URL.
- Produces: keyboard-accessible model search, live counts, and dual opposite vendor marquee.

- [ ] **Step 1: Write failing behavior tests for search query generation, suggestion matching, two marquee rows, and reduced-motion class contracts.**
- [ ] **Step 2: Run the focused tests and confirm the expected failures.**
- [ ] **Step 3: Replace the old terminal-first hero with the Molii heading, non-billing search form, model suggestions, model-square CTA, and docs CTA.**
- [ ] **Step 4: Replace fabricated counters with model/vendor/capability counts and neutral loading/error placeholders.**
- [ ] **Step 5: Add a dynamic two-row vendor marquee with reverse directions, hover pause, static reduced-motion layout, and `/pricing?vendor=...` links.**
- [ ] **Step 6: Run focused tests and type checking for the changed interfaces.**

### Task 3: Real capability and product-principle sections

**Files:**
- Modify: `web/src/features/home/components/sections/features.tsx`
- Modify: `web/src/features/home/components/sections/how-it-works.tsx`
- Create: `web/src/features/home/components/sections/latest-models.tsx`
- Create: `web/src/features/home/components/__tests__/latest-models.test.tsx`

**Interfaces:**
- Consumes: latest models and public metadata from `HomeModelCatalog`.
- Produces: five real capability cards, four Molii principles, and three latest-model cards.

- [ ] **Step 1: Write failing rendering tests for the five approved capabilities and latest dated models.**
- [ ] **Step 2: Run focused tests and confirm failure against the old marketing sections.**
- [ ] **Step 3: Implement the five capability cards without unsupported claims.**
- [ ] **Step 4: Replace the three-step installer copy with four Molii product principles and restrained icon motion.**
- [ ] **Step 5: Render the latest three models with vendor, description, modalities, release date, and model detail link; hide the section when no dated models exist.**
- [ ] **Step 6: Run focused tests and confirm green.**

### Task 4: Getting started, composition, localization, and motion

**Files:**
- Modify: `web/src/features/home/components/sections/cta.tsx`
- Modify: `web/src/features/home/components/index.ts`
- Modify: `web/src/features/home/index.tsx`
- Modify: `web/src/styles/index.css`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Modify: `web/src/i18n/static-keys.ts`

**Interfaces:**
- Consumes: `usePricingData()`, `HomeModelCatalog`, current status docs URL, and authentication state.
- Produces: completed default homepage while leaving custom-home branches unchanged.

- [ ] **Step 1: Add contract tests that assert custom-home branches remain before the default catalog composition.**
- [ ] **Step 2: Implement API and model-square getting-started cards with display-only curl/Python tabs.**
- [ ] **Step 3: Compose all sections from one memoized catalog result in `Home`; retain existing custom URL/HTML/Markdown paths byte-for-byte where practical.**
- [ ] **Step 4: Add neutral rotating highlight, seamless marquee, hover pause, mobile speed, and reduced-motion CSS.**
- [ ] **Step 5: Add English, Simplified Chinese, and Traditional Chinese copy for every new visible string and register static keys.**
- [ ] **Step 6: Run focused tests, `bun run typecheck`, scoped lint, and format check; do not run `bun run build`.**

### Task 5: Local visual and interaction verification

**Files:**
- Modify only files from Tasks 1–4 when a verified regression requires a fix.
- Update: `.ccg/tasks/molii-zenmux-homepage/review.md`

**Interfaces:**
- Consumes: the local backend at `127.0.0.1:3000` and frontend dev server at `127.0.0.1:3001`.
- Produces: verified desktop/mobile homepage behavior.

- [ ] **Step 1: Confirm local backend and frontend development processes are running; restart only the frontend when code changes are not picked up.**
- [ ] **Step 2: Verify desktop layout, dynamic counts, vendor logos, search suggestions, search navigation, latest models, API tabs, and footer.**
- [ ] **Step 3: Verify mobile layout and absence of horizontal overflow.**
- [ ] **Step 4: Emulate reduced motion and verify marquees/entrance animations stop while content remains visible.**
- [ ] **Step 5: Run the complete Bun test suite, type check, format check, scoped lint, and `git diff --check`; do not run a production build.**
- [ ] **Step 6: Review the diff against the design, write findings to the CCG review, archive the completed task, and leave the working tree ready for user inspection without pushing.**
