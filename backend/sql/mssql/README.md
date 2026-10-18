# MS SQL sales reporting mart

The Sales Dashboard reads Microsoft SQL Server through one stored procedure. The metric definitions match `lib/sales.ts` exactly, so the standalone simulation and SQL Server return the same numbers for the same filter.

| Script | Purpose |
| --- | --- |
| `001_reporting_schema.sql` | Database `onecrm_reporting`, schema `rpt`: branch, product, salesperson and lead-source dimensions; `FactSpk` (SPK → DO), `FactLeadDaily` (CRM leads for attribution), `FactTarget`, `EtlWatermark`; date indexes. |
| `002_procedures.sql` | `rpt.usp_SalesDashboard @DateFrom, @DateTo, @Branch` → 6 result sets: kpi, trend, branches, products, salespeople, channels. Validates its parameters with `THROW`. |
| `003_security.sql` | Login `svc_onecrm_dashboard` with `EXECUTE` on the procedure only (table access denied), a Row-Level Security policy for branch managers (created `OFF`), and a Change Tracking template for the incremental sync from the DMS/ERP. |
| `004_seed_demo.sql` | **Generated** synthetic data. Do not edit; regenerate with `node --experimental-strip-types scripts/export-sales-seed.mjs` from the repo root (also refreshes `backend/testdata/sales-golden.json`). |

All scripts are idempotent. Run them in order with `sqlcmd` (`-I` enables `QUOTED_IDENTIFIER`, `-C` trusts the local dev certificate):

```text
sqlcmd -C -b -I -S 127.0.0.1,14333 -U sa -i 001_reporting_schema.sql
sqlcmd -C -b -I -S 127.0.0.1,14333 -U sa -i 002_procedures.sql
sqlcmd -C -b -I -S 127.0.0.1,14333 -U sa -v ApiPassword="<api password>" -i 003_security.sql
sqlcmd -C -b -I -S 127.0.0.1,14333 -U sa -i 004_seed_demo.sql
```

With Docker, `docker compose --profile sales up --build` in `backend/` starts SQL Server 2022 and applies the four scripts through the one-shot `mssql-init` service.

Verify SQL Server against the TypeScript definitions (9 golden cases) using the least-privilege login:

```text
cd backend
MSSQL_TEST_EXPECT_NO_SELECT=1 MSSQL_TEST_DSN="sqlserver://svc_onecrm_dashboard:<url-encoded password>@127.0.0.1:14333?database=onecrm_reporting&encrypt=true&TrustServerCertificate=true" go test -run MSSQL -v ./...
```

Production notes: point the procedure at a reporting database fed by Change Tracking from a readable secondary, never at the transactional DMS; keep `encrypt=true` with a trusted certificate; store the API password in a secret manager.
