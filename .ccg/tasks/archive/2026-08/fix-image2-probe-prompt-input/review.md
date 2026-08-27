# Review

## Root cause

The Prompt used Python `input()`, which reads one terminal line and is also
subject to terminal canonical input limits. Multiline clipboard content was
therefore split and later lines could be consumed by subsequent menu prompts.

## Fix

- Read Prompt text directly from the macOS clipboard through `/usr/bin/pbpaste`.
- Invoke `pbpaste` without a shell and preserve its exact UTF-8 stdout.
- Reject empty clipboard content and surface clipboard read failures.
- Display the captured character and line counts before continuing.

## Verification

- Red test failed because clipboard Prompt support did not exist.
- Green test preserves a multiline Prompt longer than 15,000 characters.
- Empty/whitespace-only clipboard test passes.
- Full suite: 11 tests passed.
- Python bytecode compilation and whitespace checks passed.
- No real clipboard contents or paid upstream requests were used during tests.
