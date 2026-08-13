# Molii Neutral Gray System Theme Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the frontend's blue brand interaction tokens with an accessible charcoal-gray palette without restoring any theme-switching UI or behavior.

**Architecture:** Keep the existing semantic-token architecture and change only the base light/dark brand interaction variables in `theme.css`. Add a Happy DOM behavior test that loads the real stylesheet and verifies the browser-computed custom properties; verify the existing disabled switch/config controls in the local browser.

**Tech Stack:** CSS OKLCH tokens, React 19, TypeScript, Bun `node:test`, Rsbuild.

## Global Constraints

- Preserve the Serif body font, radius, density, and system-driven light/dark mode.
- Do not mount or re-enable `ThemeSwitch`, `ThemeQuickSwitcher`, or `ConfigDrawer`.
- Keep success, warning, destructive, info, and chart colors semantically distinct.
- Do not add a new theme preset or theme preference persistence.
- Do not run a production build; verify in the local development environment only.

---

### Task 1: Lock the charcoal brand-token contract

**Files:**
- Create: `web/src/styles/__tests__/theme-contract.test.ts`
- Test: `web/src/styles/__tests__/theme-contract.test.ts`

**Interfaces:**
- Consumes: the real stylesheet in `web/src/styles/theme.css` through Happy DOM's CSS engine.
- Produces: a regression contract for the browser-computed light/dark charcoal tokens.

- [ ] **Step 1: Write the failing color-token test**

Create a `node:test` test that reads `theme.css`, attaches it to a `Window` from `happy-dom`, and asserts the computed custom-property values on `document.documentElement` before and after adding the `.dark` class:

```ts
const LIGHT_CHARCOAL_TOKENS = {
  '--primary': 'oklch(0.28 0 0)',
  '--primary-foreground': 'oklch(0.985 0 0)',
  '--secondary-foreground': 'oklch(0.28 0 0)',
  '--ring': 'oklch(0.5 0 0)',
  '--sidebar-primary': 'oklch(0.28 0 0)',
  '--sidebar-primary-foreground': 'oklch(0.985 0 0)',
  '--sidebar-accent-foreground': 'oklch(0.22 0 0)',
  '--sidebar-ring': 'oklch(0.5 0 0)',
}

const DARK_CHARCOAL_TOKENS = {
  '--primary': 'oklch(0.82 0 0)',
  '--primary-foreground': 'oklch(0.2 0 0)',
  '--secondary-foreground': 'oklch(0.9 0 0)',
  '--ring': 'oklch(0.7 0 0)',
  '--sidebar-primary': 'oklch(0.82 0 0)',
  '--sidebar-primary-foreground': 'oklch(0.2 0 0)',
  '--sidebar-accent-foreground': 'oklch(0.95 0 0)',
  '--sidebar-ring': 'oklch(0.7 0 0)',
}
```

Also read the computed `--success`, `--warning`, `--info`, and `--chart-1` values, parse their OKLCH chroma component, and assert it is greater than zero so the implementation cannot accidentally turn semantic states and charts monochrome.

- [ ] **Step 2: Run the focused test and verify RED**

Run: `cd web && bun test src/styles/__tests__/theme-contract.test.ts`

Expected: the charcoal token test fails because the browser computes the existing blue values.

---

### Task 2: Apply the accessible charcoal palette

**Files:**
- Modify: `web/src/styles/theme.css:112-148`
- Modify: `web/src/styles/theme.css:189-222`
- Test: `web/src/styles/__tests__/theme-contract.test.ts`

**Interfaces:**
- Consumes: the exact token contract from Task 1.
- Produces: the neutral brand colors used by Tailwind `primary`, `accent`, `ring`, and sidebar utilities throughout the frontend.

- [ ] **Step 1: Replace light-mode brand interaction tokens**

Set the light variables to:

```css
--primary: oklch(0.28 0 0);
--primary-foreground: oklch(0.985 0 0);
--secondary-foreground: oklch(0.28 0 0);
--ring: oklch(0.5 0 0);
--sidebar-primary: oklch(0.28 0 0);
--sidebar-primary-foreground: oklch(0.985 0 0);
--sidebar-accent-foreground: oklch(0.22 0 0);
--sidebar-ring: oklch(0.5 0 0);
```

Keep `accent` and `sidebar-accent` as `color-mix` expressions derived from `primary`.

- [ ] **Step 2: Replace dark-mode brand interaction tokens**

Set the dark variables to:

```css
--primary: oklch(0.82 0 0);
--primary-foreground: oklch(0.2 0 0);
--secondary-foreground: oklch(0.9 0 0);
--ring: oklch(0.7 0 0);
--sidebar-primary: oklch(0.82 0 0);
--sidebar-primary-foreground: oklch(0.2 0 0);
--sidebar-accent-foreground: oklch(0.95 0 0);
--sidebar-ring: oklch(0.7 0 0);
```

- [ ] **Step 3: Run the focused test and verify GREEN**

Run: `cd web && bun test src/styles/__tests__/theme-contract.test.ts`

Expected: all tests pass.

- [ ] **Step 4: Run frontend verification**

Run:

```bash
cd web
bun test
bun run typecheck
bun run format:check
bunx oxlint -c .oxlintrc.json src/styles/__tests__/theme-contract.test.ts
cd ..
git diff --check
```

Expected: 0 test failures and all commands exit 0.

- [ ] **Step 5: Commit the implementation**

```bash
git add web/src/styles/theme.css web/src/styles/__tests__/theme-contract.test.ts .ccg/tasks/neutral-gray-system-theme
git commit -m "feat: adopt charcoal system accent"
```

---

### Task 3: Verify the local light/dark experience

**Files:**
- Modify: `.ccg/tasks/neutral-gray-system-theme/review.md`
- Verify: `web/src/styles/theme.css`

**Interfaces:**
- Consumes: the running local frontend on `127.0.0.1:3001` and backend entry on `127.0.0.1:3000`.
- Produces: visual verification evidence and the completed task record.

- [ ] **Step 1: Verify the development services**

Run health checks for `http://127.0.0.1:3000/api/status`, `http://127.0.0.1:3000/pricing`, and `http://127.0.0.1:3001/pricing`. Restart only the local development process if needed.

- [ ] **Step 2: Inspect light mode in the browser**

Verify a public page and an authenticated page. Confirm primary buttons, links, active filters, focus rings, and sidebar selection use charcoal/neutral gray; confirm status colors and model/vendor logos remain colored; confirm no theme/config switch is visible.

- [ ] **Step 3: Inspect dark mode in the browser**

Emulate `prefers-color-scheme: dark` without adding a UI switch. Confirm the lighter neutral primary remains legible on the charcoal canvas and that focus/selected states remain distinguishable.

- [ ] **Step 4: Record review and archive the task**

Write the verification results to `.ccg/tasks/neutral-gray-system-theme/review.md`, set `task.json` to `completed`, move the task to `.ccg/tasks/archive/2026-08/neutral-gray-system-theme`, and commit:

```bash
git add .ccg/tasks
git commit -m "chore: archive ccg task neutral-gray-system-theme"
```
