# Review

- Root cause: `object-cover` intentionally crops an image to fill its fixed thumbnail box.
- Fix: use `object-contain` so the complete image is scaled proportionally and centered inside the existing neutral thumbnail background.
- Scope: one presentation class and its regression test; request payload and asset selection behavior are unchanged.
- Verification: focused tests 5/5, typecheck, focused lint, focused format check, and production build all passed.
