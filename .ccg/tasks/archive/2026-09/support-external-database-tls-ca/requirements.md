# External database TLS CA support

- Mount the deployment directory's `certs/` directory read-only into the application container.
- Allow Redis TLS connections to load a configured CA file without disabling certificate verification.
- Keep existing deployments compatible when no custom Redis CA is configured.
- Document the verified PostgreSQL and Redis TLS settings for the iXiaozu production environment.
- Fail with a clear startup error when a configured Redis CA cannot be read or parsed.
- Cover the Redis TLS configuration and Compose deployment contract with automated tests.
