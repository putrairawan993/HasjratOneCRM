# Hasjrat OneCRM assessment demo

Full assessment prototype by Putra Irawan:

- Frontend: Next.js 16, React 19 and TypeScript
- Backend: Go API in `backend/`
- CRM persistence: MySQL 8.4
- Sales reporting: Microsoft SQL Server 2022 (`rpt` star schema + `rpt.usp_SalesDashboard`), optional
- AI follow-up draft: DeepSeek through the Go backend only

```text
npm ci
npm run dev
```

Local URL: http://127.0.0.1:4173

For a static production build, run `npm run build`, then `npm start`. The static server serves `out/` on loopback only.

Leave `NEXT_PUBLIC_API_URL` unset for the standalone demo. To connect the Go API, set it to `http://127.0.0.1:8080` in `.env.local`, restart, then connect your local demo token in Integration Hub. No token or DeepSeek key belongs in public environment variables.

Run the backend locally with Docker:

```text
cd backend
copy .env.example .env
docker compose up --build
```

Add private values for `MYSQL_PASSWORD`, `MYSQL_ROOT_PASSWORD`, `DEMO_API_TOKEN`, and `DEEPSEEK_API_KEY` in `backend/.env`. The file is ignored by Git.

To serve the Sales Dashboard from SQL Server, also set `MSSQL_SA_PASSWORD`, `MSSQL_API_PASSWORD` and `MSSQL_DSN` (see `backend/.env.example`) and start the `sales` profile:

```text
docker compose --profile sales up --build
```

The one-shot `mssql-init` service applies `backend/sql/mssql/001`–`004`: schema, procedure, an EXECUTE-only login and synthetic seed data. The API then answers `GET /api/sales/dashboard?from=&to=&branch=`. Without `MSSQL_DSN` that endpoint returns 503 and the UI stays on its labelled simulation. The metric definitions live in `lib/sales.ts`; `node --experimental-strip-types scripts/export-sales-seed.mjs` regenerates the SQL seed and `backend/testdata/sales-golden.json`, and `backend/sql/mssql/README.md` shows how to run the golden test against a live SQL Server.

Vercel deploys the Next.js frontend from this repository. Without `NEXT_PUBLIC_API_URL`, the public deployment runs in clearly labelled standalone simulation mode. Live DeepSeek and MySQL require the Go API to be deployed separately over HTTPS, then its URL must be provided to the frontend at build time.

Ten views: CRM Overview, Sales Dashboard, Omnichannel Inbox, Lead Pipeline, Customer 360°, Journey Automation, Campaign Studio, AI Playbook, Integration Hub and Assessment Brief (Task 1–3, sales dashboard, AI, roadmap).

The written submission and slide deck are in `docs/`.

All customers, products, branches and figures are synthetic. Messages and ERP events are simulated. The local follow-up template is explicitly labeled and is not a real DeepSeek response. Default browser data can be reset in Integration Hub.

This app is a concept for Putra Irawan's assessment, not an official PT Hasjrat Abadi product. It contains no personal memory export or private CV files.
