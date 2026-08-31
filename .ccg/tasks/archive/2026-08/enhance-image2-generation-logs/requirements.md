# Requirements

- Enrich exact `gpt-image-2` common-log details with quality, background, output format, moderation, size, user, output count, and total duration.
- Use a designed, bordered metric layout rather than a flat content string.
- Preserve the OpenAI API Base64 response while saving a best-effort Molii-owned COS preview.
- Retain both image objects and lookup metadata for exactly 24 hours and then delete them.
- Add a `GPT Image 2` source under `/usage-logs/drawing`, modeled after the existing Grok Image source.
- Keep historical logs readable and leave all other image models unchanged.
