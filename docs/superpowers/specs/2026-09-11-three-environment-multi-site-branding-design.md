# Three-Environment Multi-Site Deployment and Branding Design

## Goal

Establish one promotion path from local development to a shared development site and then to two independent production sites:

1. Local development produces commits for `develop`.
2. A successful `develop` CI/CD run deploys to `https://dev.molii.co` for acceptance.
3. Accepted commits are merged from `develop` into `main`.
4. One successful `main` build deploys the same source revision to both `https://molii.co` and `https://aigc.ixiaozu.cn`.

The two production sites share application code but remain operationally independent. Each has its own domain, PostgreSQL database, Redis instance, user system, channel configuration, runtime secrets, branding, documentation site, health check, rollback result, and notification result.

## Environment Map

| Branch | Deployment target | Public application | Public documentation | Data boundary |
| --- | --- | --- | --- | --- |
| `develop` | development | `https://dev.molii.co` | `https://dev.molii.co/docs/` | Development PostgreSQL and Redis only |
| `main` | Molii production | `https://molii.co` | `https://molii.co/docs/` | Molii production PostgreSQL and Redis only |
| `main` | iXiaozu production | `https://aigc.ixiaozu.cn` | `https://aigc.ixiaozu.cn/docs/` | iXiaozu production PostgreSQL and Redis only |

Merging into `main` starts both production deployments. They run independently with matrix `fail-fast: false`: a failure on one site does not cancel or roll back the other site. The workflow reports each target separately and reports the overall release as partially successful when only one target succeeds.

## Application Build and Deployment

All application artifacts carry the same Git SHA and application version. Brand-dependent static assets require separate frontend builds:

- the development and Molii production builds use the Molii brand profile;
- the iXiaozu production build uses the iXiaozu brand profile;
- the Go backend and business behavior are built from the same revision;
- each deploy job uses the immutable digest produced for its brand profile rather than a mutable Docker tag.

The brand profile is public build configuration, not a secret. Database DSNs, Redis URLs, session secrets, encryption secrets, channel keys, and user data remain only in the target server's `.env.runtime` file.

The workflow uses GitHub Environments to isolate deployment variables and secrets:

- `development`
- `production-molii`
- `production-ixiaozu`

Each environment defines its own SSH connection, deployment directory, public health URL, brand profile, and Telegram destination if notification routing differs. Production environments may use GitHub environment protection rules; development remains automatic.

## Server and Data Isolation

Each target server directory contains its own Compose file, deployment lock, runtime environment file, generated deployment environment, and persistent application paths. No production target shares PostgreSQL roles, databases, Redis databases, session secrets, crypto secrets, object-storage credentials, or channel configuration with another target.

The deployment script accepts a target identifier rather than assuming one global production target. It validates the target's expected public URL and uses target-specific container names, ports, paths, and Compose project names. It records the previous immutable image digest and rolls back only the failing target.

Database migrations run inside each deployed application independently. A production deployment is considered successful only after the container is healthy and that target's public `/api/status` responds successfully. Migrations must remain backward-compatible because an application-image rollback cannot automatically reverse a database migration.

## Main-Site Brand Model

Branding has two layers:

### Build-time fallback

The build profile provides the correct initial browser metadata before the application loads:

- site title;
- meta title and description;
- favicon and Apple touch icon;
- default site brand name;
- default heading and body font selection;
- default brand assets.

This prevents the iXiaozu site, crawlers, link previews, or a slow first paint from exposing Molii's hard-coded title or favicon.

### Runtime site configuration

After `/api/status` loads, the existing independent database settings remain authoritative for system name, logo, footer, server address, documentation URL, and custom homepage content. Runtime values can override the matching build fallback without rebuilding the application.

The default homepage obtains its visible brand name and font defaults from the active site brand configuration. User-selected appearance settings remain domain-local and may override the site's default font in that browser.

## Homepage Banner and Colored Brand Text

The existing colored-letter brand component is retained. It is generalized from matching the literal word `Molii` to rendering the active site's configured banner brand name.

Visible brand characters are segmented as Unicode grapheme clusters and receive the existing colors in a repeating sequence:

1. pink;
2. blue;
3. pink;
4. blue;
5. continue alternating for the remaining characters.

This preserves the current `Molii` result while supporting Latin text, Chinese, punctuation, emoji, whitespace, and combined Unicode characters without breaking a visual character into multiple spans. Every grapheme advances the alternating color sequence; whitespace keeps its layout but has no visible paint. Surrounding sentence text keeps the normal banner color. Screen readers receive the complete unsplit sentence through one accessible label.

The homepage layout, responsive type scale, spacing, animation, search, and calls to action remain shared. Only the configured brand text, site copy, logo, favicon, and default font vary by site.

## Documentation Branding and Deployment

The Docusaurus documentation is built separately for Molii and iXiaozu because title, logo, favicon, canonical URL, sitemap host, social metadata, API base URL, footer branding, and font defaults are static build inputs.

Both documentation builds use the same Markdown/MDX and OpenAPI content revision. A typed documentation brand profile supplies all site-specific public values. The output is a complete static snapshot and does not fetch another site's branding or API URL at runtime.

The two documentation deployments are independent jobs with target-specific health checks and notifications. A documentation failure does not overwrite a previously healthy deployment.

## Promotion, Failure, and Notification Semantics

- A `develop` push verifies, builds, and deploys only the development target.
- Production is never deployed directly from `develop`.
- A `main` push verifies the revision once, builds required brand variants, and dispatches the two production targets.
- A failed target rolls back only itself.
- A successful target remains on the new revision even if its peer fails.
- Telegram messages identify the site, domain, Git SHA, deployment result, rollback result, and Actions run.
- The final workflow summary lists application and documentation outcomes for each target, including partial success.

## Security Boundaries

- Production secrets are GitHub Environment secrets or server-local runtime secrets, never repository-level shared values when they differ by target.
- SSH known-host entries are pinned for each server.
- Application ports remain bound to loopback behind each server's reverse proxy.
- Brand variables contain only public presentation data.
- No database content, API key, Redis credential, or signing secret is copied between sites by CI/CD.
- The release workflow does not create users, channels, or pricing records on either production system.

## Verification

Automated verification covers:

- branch-to-target mapping and the absence of production deployment from `develop`;
- two independent production matrix entries with `fail-fast: false`;
- per-target environment, SSH, directory, health URL, image digest, and rollback isolation;
- brand-profile validation with no Molii fallback in the generated iXiaozu HTML;
- dynamic document title and favicon replacement from runtime status;
- alternating grapheme-safe banner colors for `Molii`, arbitrary Latin names, Chinese names, spaces, and emoji;
- independent Docusaurus canonical URL, sitemap, title, assets, API base URL, footer, and fonts;
- production build, frontend type checking, Go tests, deployment contract tests, Compose validation, and workflow syntax;
- smoke checks for `/`, `/api/status`, and the documentation landing page on each target.

## Operator Inputs

Before the first iXiaozu release, the operator supplies its public brand values through the `production-ixiaozu` GitHub Environment and its private runtime values on the target server. Missing required brand values fail the build with a clear message; they never silently fall back to Molii. The Molii and development profiles retain the current Molii assets and appearance unless explicitly changed.

## Out of Scope

- Synchronizing users, channels, balances, logs, or model configuration between production sites.
- Deploying one production site's database backup into the other site.
- Automatically promoting `develop` to `main` without review.
- Automatically reversing database migrations during image rollback.
- Creating independent forks of application business logic for each brand.
