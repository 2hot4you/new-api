# PostgreSQL and Redis deployment verification gate

- The application runtime continues to use external PostgreSQL and Redis only.
- The deployment verification job must provision isolated PostgreSQL 15 and Redis 7 services.
- The verification job must create every dedicated PostgreSQL database used by the test suite.
- The redemption batch database matrix must use its own PostgreSQL database so repeated full and race suites cannot inherit tables from unrelated controller tests.
- Backend tests must receive explicit PostgreSQL and Redis test connection strings so they do not fall back to an implicit local database.
- Production database and Redis credentials must remain isolated in each server's runtime file and must not enter GitHub Actions.
