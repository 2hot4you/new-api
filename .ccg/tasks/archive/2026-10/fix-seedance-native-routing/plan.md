# Scope and evidence
Read-only ixiaozu logs matched both supplied request IDs: actual POST /v1/videos and POST /api/video-studio/estimate; both failed at endpoint prepare_rejected/plugin doubao. No upstream request, container mutation or database write performed.

Independent analysis approves narrow union precheck fix. Existing service already bridges 4 Seedance 2.x IDs to native 61/64; relay chooses actual native adaptor before JS driver. Only shared decoder blocks -1.

## Plan
1. Add real-plugin HTTP regressions in middleware/seedance_smart_duration_test.go, covering public and studio paths, native validation/body and positive StarAI usage facts; legacy plugin final rejection; invalid bounds and unbridged models.
2. Permit exact -1 sentinel only for the four already-bridged models in Doubao openai_video JSON/multipart precheck. Preserve original JSON payload and seconds precedence; do not change driver, quota checks, channel selection or provider APIs.
3. Register both tests in the explicit CI inventory so normal/race cannot omit them. Run focused normal/race, affected package suites, project test command and PostgreSQL acceptance where applicable; independent review. Record failures honestly.
4. Archive CCG task; push feature branch, PR to develop. Do not merge or deploy without release authorization.

## Boundary
This restores native61/64 support, not JS54/45 automatic duration. A mixed pool may still select an unsupported legacy channel and correctly reject before submission. Channel priority and permissions are not changed. Alias support is not expanded. Existing settlement/refund untouched.
