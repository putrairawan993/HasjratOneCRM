# OneCRM frontend

Next.js + TypeScript assessment prototype with synthetic data. Parent project includes a Go/MySQL API and architecture/presentation documents.

```text
npm ci
npm run dev
```

Local URL: http://127.0.0.1:4173

For a static production build, run `npm run build`, then `npm start`. The static server serves `out/` on loopback only.

Leave `NEXT_PUBLIC_API_URL` unset for the standalone demo. To connect the Go API, set it to `http://127.0.0.1:8080` in `.env.local`, restart, then connect your local demo token in Integration Hub. No token or DeepSeek key belongs in public environment variables.

Eight views: Sales Overview, Omnichannel Inbox, Lead Pipeline, Customer 360°, Journey Automation, Campaign Studio, Integration Hub and Assessment Brief.

All customers, products, branches and figures are synthetic. Messages and ERP events are simulated. The local follow-up template is explicitly labeled and is not a real DeepSeek response. Default browser data can be reset in Integration Hub.

This app is a concept for Putra Irawan's assessment, not an official PT Hasjrat Abadi product. It contains no personal memory export or private CV files.
