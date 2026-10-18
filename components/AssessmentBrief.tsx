'use client';
import {useState} from 'react';
import {Icon, Tag} from './ui';

const TABS = [{id:'summary', label:'Ringkasan'}, {id:'task1', label:'Task 1 · Diagnosis'}, {id:'task2', label:'Task 2 · Journey'}, {id:'task3', label:'Task 3 · Arsitektur & otomasi'}, {id:'sales', label:'Sales dashboard'}, {id:'ai', label:'AI'}, {id:'roadmap', label:'Roadmap'}];
const OUTCOMES = [
  ['Follow-up tepat waktu', '70% (30% ≈ 3.000 inquiry terlambat)', '≥ 90% respons manusia ≤ 15 menit pada jam kerja'],
  ['Customer mengulang informasi', '20% (≈ 2.000 inquiry)', '< 5%, karena profil & ringkasan lintas channel'],
  ['Visibilitas status prospect', 'Tidak ada, berbasis laporan manual', 'Real-time, data per ≤ 15 menit'],
  ['Data customer di HP pribadi', 'Ada, tidak terukur', '0; 100% percakapan di nomor resmi perusahaan'],
  ['Standar follow-up', 'Bergantung individu sales', '1 playbook baku H+0/1/3/7/14 + nurture'],
  ['Broadcast marketing', 'Manual, tanpa segmentasi', 'Tersegmentasi, berbasis consent, terjadwal, ber-frequency cap'],
  ['Pembeli menerima prospecting', 'Terjadi', '0, suppression otomatis dari SPK/DO ERP']
];
const PROBLEMS = [
  ['30% inquiry tidak di-follow-up tepat waktu', 'Tidak ada antrean bersama, routing, SLA timer, dan eskalasi; inquiry malam tidak tertangani; beban sales tidak seimbang', 'Lead dingin dan pindah ke kompetitor; kecepatan respons pertama sangat menentukan peluang', 'Shared inbox, auto-routing cabang/produk/kapasitas, SLA 15 menit + eskalasi, bot kualifikasi di luar jam kerja'],
  ['20% customer mengulang informasi saat pindah channel', 'Identitas terpecah per channel dan per HP sales; tidak ada riwayat bersama', 'Pengalaman buruk, waktu sales terbuang, kebutuhan customer hilang', 'Identity resolution (nomor E.164, email, ID sosial) → satu profil; timeline lintas channel; ringkasan AI saat handover'],
  ['Management tidak melihat status prospect real-time', 'Status hidup di chat; definisi stage tidak baku; CRM tidak terhubung DMS/ERP', 'Forecast, alokasi stok, dan promosi berbasis asumsi', 'Pipeline baku, dashboard CRM, dan Sales dashboard dari MS SQL'],
  ['Sales menyimpan customer di HP pribadi', 'Nomor WA personal; tidak ada kebijakan kepemilikan data', 'Aset customer hilang saat sales resign; risiko UU PDP; tidak dapat diaudit', 'WhatsApp Business Platform per cabang, akses berbasis role, reassign massal'],
  ['Tidak ada standard follow-up journey', 'Tidak ada SOP, stage, dan cadence; follow-up bergantung individu', 'Konversi tidak konsisten antar sales dan cabang', 'Journey baku, cadence otomatis, task harian, stop rules'],
  ['Broadcast marketing manual', 'Tanpa segmentasi, consent, template, dan penjadwalan', 'Salah sasaran, boros waktu, risiko quality rating dan pemblokiran nomor WA', 'Campaign studio: segmen, template disetujui, consent, frequency cap, pengukuran'],
  ['Customer yang sudah membeli tetap menerima prospecting', 'Status pembelian di DMS/ERP tidak mengalir ke daftar broadcast', 'Pengalaman buruk dan menurunkan kepercayaan', 'Event SPK/DO dari MS SQL → suppression otomatis + pindah ke journey aftersales']
];
const ROOTS = [
  {title:'People', text:'Ownership customer melekat ke individu, bukan perusahaan. Insentif belum mendorong pencatatan.'},
  {title:'Process', text:'Tidak ada SOP, definisi stage, SLA, cadence, dan aturan eskalasi.'},
  {title:'Technology', text:'WhatsApp personal tanpa API, tanpa shared inbox, tanpa integrasi DMS/ERP.'},
  {title:'Data', text:'Identitas terpecah, tidak ada ID customer tunggal, tidak ada KPI baku.'},
  {title:'Governance', text:'Consent, kepemilikan data, dan audit belum diatur sesuai UU PDP.'}
];
const JOURNEY = [
  ['Awareness', 'Mengenal produk & promo', 'Ads IG/FB/TikTok, website, click-to-WA', 'UTM, ad ID, dan referral click-to-WA tersimpan', 'Marketing', 'Sumber lead tercatat'],
  ['Inquiry & capture', 'Mendapat jawaban cepat', 'WA, DM IG/FB, TikTok lead form, web form', 'Auto-ack < 1 menit, permintaan consent, dedupe & merge profil, lead dibuat', 'Sistem · < 1 menit', 'Lead + consent tersimpan'],
  ['Kualifikasi', 'Kebutuhan dipahami', 'Chat WA, bot di luar jam kerja', 'Pertanyaan model, budget, cash/kredit, waktu beli, lokasi; skor Hot/Warm/Cold', 'Sales · respons manusia ≤ 15 menit', 'Qualified atau Nurture'],
  ['Assignment & konsultasi', 'Dilayani orang yang tepat', 'WA, telepon', 'Routing cabang/produk/kapasitas; brosur, price list, simulasi kredit', 'Sales owner · eskalasi supervisor bila SLA lewat', 'Jadwal test drive/kunjungan'],
  ['Test drive / kunjungan', 'Mencoba unit', 'Showroom, home test drive', 'Booking, reminder H-1 & 2 jam, follow-up no-show', 'Sales · konfirmasi H-1', 'Test drive selesai'],
  ['Penawaran & negosiasi', 'Harga & skema terbaik', 'WA, showroom', 'Quotation, simulasi leasing, trade-in, approval diskon', 'Sales + supervisor', 'Proposal diterima atau Lost + alasan'],
  ['SPK & pembiayaan', 'Kepastian order', 'DMS/ERP (MS SQL)', 'Event SPK → stage Order, prospecting berhenti, checklist dokumen & status leasing', 'Sales admin', 'SPK aktif'],
  ['DO & serah terima', 'Unit diterima dengan baik', 'Showroom, WA', 'Event DO → Won, jadwal serah terima, survey CSAT/NPS H+1', 'Delivery / CRO', 'Won'],
  ['Aftersales & loyalty', 'Kendaraan terawat, nilai berlanjut', 'WA, bengkel resmi', 'Reminder servis, STNK & asuransi, referral, program trade-in', 'CRO / service', 'Repeat order & referral']
];
export const CADENCE = [['H+0', '≤ 15 menit', 'Respons personal + kualifikasi'], ['H+1', 'Info sesuai minat', 'Brosur, simulasi kredit'], ['H+3', 'Ajakan test drive', 'Slot jadwal terdekat'], ['H+7', 'Penawaran relevan', 'Promo/program yang berlaku'], ['H+14', 'Check-in terakhir', 'Belum siap → Nurture'], ['≤ 90 hari', 'Nurture bulanan', 'Konten edukasi, re-score']];
const AUTOMATIONS = [
  ['inquiry.created', 'Semua channel', 'Auto-ack, minta consent, merge profil, routing, mulai SLA timer', 'Waktu auto-ack < 1 menit'],
  ['sla.warning (10 menit)', 'Belum ada respons manusia', 'Push notifikasi ke owner', '% respons ≤ 15 menit'],
  ['sla.breached (15 menit)', 'Masih belum dibalas', 'Eskalasi supervisor / reassign ke sales tersedia', 'Jumlah breach per cabang'],
  ['Inquiry di luar jam kerja', '18.00–08.00, libur', 'Bot kualifikasi + janji waktu respons; antre shift pagi', '% siap ditindaklanjuti 08.00'],
  ['lead.scored = Hot', 'Skor ≥ ambang', 'Task prioritas + link booking test drive', 'Konversi lead Hot'],
  ['testdrive.booked', 'Jadwal tersimpan', 'Reminder H-1 dan 2 jam; no-show → task follow-up', 'Show rate test drive'],
  ['Tidak ada balasan', 'H+1 / H+3 / H+7 / H+14', 'Task + template sesuai cadence; H+14 → Nurture', 'Reply rate'],
  ['erp.spk.created', 'SPK baru di DMS/ERP', 'Stage Order, hentikan prospecting, mulai journey order', 'Pembeli di audience prospecting = 0'],
  ['erp.do.delivered', 'DO terbit', 'Stage Won, jadwal serah terima, survey H+1, mulai journey aftersales', 'CSAT/NPS'],
  ['Keyword opt-out', '"STOP", "BERHENTI"', 'Consent dicabut, suppression semua marketing', 'Opt-out diproses < 1 menit'],
  ['Sales resign / mutasi', 'Status karyawan berubah', 'Reassign massal lead & percakapan; riwayat tetap', 'Lead tanpa owner = 0'],
  ['Campaign terjadwal', 'Saat kirim', 'Cek ulang consent, stage, template, service window, frequency cap', 'Opt-out & quality rating']
];
const SOURCES = [
  {name:'WhatsApp', text:'WhatsApp Business Platform (Cloud API via BSP). Webhook pesan & status; referral click-to-WA ads membawa ID iklan. Template disetujui untuk pesan di luar 24 jam.'},
  {name:'Website', text:'Form, chat widget, dan tombol click-to-WA dengan UTM. Validasi server, anti-spam, checkbox consent.'},
  {name:'Instagram & Facebook', text:'Webhook Messenger/IG messaging dan Lead Ads (leadgen) → detail lead diambil via Graph API dengan token aplikasi.'},
  {name:'TikTok', text:'Lead Generation instant form → webhook/CRM integration atau Leads API sesuai entitlement akun.'},
  {name:'Walk-in & telepon', text:'Input cepat di aplikasi sales/tablet showroom atau QR di meja tamu; masuk pipeline yang sama.'}
];
const LAYERS = [
  {title:'Channel', items:['WhatsApp Business Platform', 'Instagram', 'Facebook', 'TikTok Lead Gen', 'Website', 'Walk-in / telepon']},
  {title:'Integrasi', items:['Webhook gateway + verifikasi signature', 'Queue / event bus', 'Idempotency & retry', 'Adapter per channel']},
  {title:'Core OneCRM · Go', items:['Identity resolution', 'Lead: dedupe, skor, routing, SLA', 'Shared inbox', 'Journey & automation engine', 'Consent & preferensi', 'Campaign', 'Task & eskalasi']},
  {title:'Data', items:['OneCRM DB (MySQL demo / MS SQL)', 'Audit & event log', 'DMS/ERP MS SQL · CT/CDC', 'Reporting MS SQL · rpt', 'Object storage media']},
  {title:'Pengguna', items:['Sales PWA', 'Konsol supervisor', 'Dashboard manajemen', 'Campaign studio']}
];
const ROADMAP = [
  {week:'Minggu 1–2', title:'Discovery & baseline', text:'Audit channel, ukur waktu respons, duplikasi, porsi inquiry malam, kontrak data DMS/ERP, consent.'},
  {week:'Minggu 3–5', title:'Core CRM pilot', text:'WA + website di 1–2 cabang: shared inbox, routing, SLA, Customer 360°, journey baku.'},
  {week:'Minggu 6–8', title:'MS SQL & lifecycle', text:'Change Tracking SPK/DO, suppression pembeli, journey aftersales, Sales dashboard.'},
  {week:'Minggu 9–12', title:'Ekspansi & evaluasi', text:'IG/FB/TikTok sesuai akses API, AI pilot (ekstraksi, ringkasan, draft), evaluasi KPI.'}
];
const RISKS = [
  ['Adopsi sales rendah', 'Libatkan sales sejak desain, aplikasi mobile yang cepat, KPI & insentif berbasis data CRM'],
  ['Kualitas data DMS/ERP', 'Profiling data saat discovery, rekonsiliasi harian, owner data per domain'],
  ['Akses API channel & approval Meta/TikTok', 'Mulai dari WA + website, ajukan akses paralel, pilih BSP resmi'],
  ['Nomor WA terblokir karena blasting', 'Template disetujui, consent, frequency cap, pantau quality rating'],
  ['Kepatuhan UU PDP', 'Consent tercatat, minimisasi data, akses berbasis role, retensi & audit'],
  ['Halusinasi AI', 'Grounding ke data resmi, review manusia, golden set evaluasi']
];

function Rows({head, rows}:{head:string[]; rows:string[][]}) {return <div className="table-scroll"><table className="wrap"><thead><tr>{head.map(h => <th key={h}>{h}</th>)}</tr></thead><tbody>{rows.map(r => <tr key={r[0]}>{r.map((c, i) => <td key={i}>{i === 0 ? <b>{c}</b> : c}</td>)}</tr>)}</tbody></table></div>;}

export default function AssessmentBrief({go}:{go:(view:string)=>void}) {
  const [tab, setTab] = useState('summary');
  return <>
    <section className="assessment-hero"><div className="eyebrow">TECHNICAL ASSESSMENT / PUTRA IRAWAN</div><h2>Hasjrat OneCRM</h2><h3>Mengubah WhatsApp dari sekadar communication channel menjadi customer management platform yang terintegrasi.</h3><p>One Customer. One Journey. Every Channel.</p><Tag>Usulan konsep aplikasi · bukan produk resmi PT Hasjrat Abadi · seluruh data sintetis</Tag></section>
    <div className="brief-tabs" role="tablist" aria-label="Bagian assessment">{TABS.map(t => <button key={t.id} role="tab" aria-selected={tab === t.id} onClick={() => setTab(t.id)}>{t.label}</button>)}</div>
    {tab === 'summary' && <>
      <div className="brief-grid">{[{title:'Satu sumber data customer', text:'Nomor WhatsApp milik perusahaan, shared inbox, dan profil 360° yang menyatukan WA, website, IG, FB, TikTok, dan walk-in.'}, {title:'Satu standar journey', text:'SLA, routing, cadence follow-up, dan stop rules dijalankan sistem, bukan bergantung ingatan individu.'}, {title:'Satu kebenaran penjualan', text:'Status SPK/DO dari DMS/ERP MS SQL mengalir ke CRM dan dashboard, sehingga pembeli tidak lagi diprospek.'}].map(s => <section className="panel panel-body" key={s.title}><h3>{s.title}</h3><p>{s.text}</p></section>)}</div>
      <section className="panel"><div className="panel-heading"><div><h2>Target hasil</h2><p>Target pilot diusulkan, dikonfirmasi setelah baseline minggu 1–2</p></div></div><Rows head={['Indikator', 'Kondisi skenario', 'Target pilot 12 minggu']} rows={OUTCOMES}/></section>
      <section className="panel panel-body"><h2>Panduan demo · 5 menit</h2><ol className="demo-guide">{[['dashboard', 'CRM Overview: filter cohort, lihat lead yang melewati SLA.'], ['inbox', 'Omnichannel Inbox: buat draft follow-up (DeepSeek/template), tinjau, catat balasan.'], ['customers', 'Customer 360°: ubah owner/status, simulasikan pembelian ERP → prospecting berhenti.'], ['campaigns', 'Campaign Studio: buat audience, jalankan send-time check.'], ['sales', 'Sales Dashboard: SPK/DO/target per cabang dari kontrak MS SQL rpt.'], ['ai', 'AI Playbook: yang sudah diterapkan dan roadmap use case.'], ['integration', 'Integration Hub: replay event dua kali, simulasi ERP outage.']].map(([id, text]) => <li key={id}><button className="text-button" onClick={() => go(id)}>{text}<Icon name="arrow" size={13}/></button></li>)}</ol></section>
    </>}
    {tab === 'task1' && <>
      <section className="panel panel-body"><div className="eyebrow">TASK 1 · PROBLEM DIAGNOSIS</div><h2 className="integration-title">WhatsApp dipakai sebagai alat chat personal, bukan sebagai sistem.</h2><p>Ketujuh gejala bersumber dari empat kekosongan: tidak ada system of record dan ownership perusahaan, tidak ada proses lead (routing, SLA, eskalasi), identitas customer terpecah, dan marketing tidak terhubung dengan status penjualan di DMS/ERP. Akibatnya juga tidak ada lapisan data untuk manajemen.</p></section>
      <section className="panel"><div className="panel-heading"><div><h2>Gejala → akar masalah → dampak → solusi</h2><p>Tujuh masalah pada skenario</p></div></div><Rows head={['Gejala', 'Akar masalah', 'Dampak bisnis', 'Komponen solusi']} rows={PROBLEMS}/></section>
      <div className="root-grid">{ROOTS.map(r => <section className="panel panel-body" key={r.title}><Tag tone="blue">{r.title}</Tag><p>{r.text}</p></section>)}</div>
      <div className="dashboard-grid"><section className="panel panel-body"><h2>Ukuran dampak · ilustrasi</h2><div className="formula"><span>10.000 inquiry ÷ 2,5 inquiry per lead</span><b>≈ 4.000 lead unik / bulan</b><span>× 30% terlambat</span><b>≈ 1.200 lead / bulan</b><span>× selisih konversi (5% tepat waktu − 2,5% terlambat)</span><b>≈ 30 SPK hilang / bulan</b></div><p className="fine">Rasio 2,5 inquiry/lead dan konversi 5% vs 2,5% adalah asumsi ilustratif untuk divalidasi dengan data baseline. Tujuannya menunjukkan cara menghitung, bukan klaim angka.</p></section>
        <section className="panel panel-body"><h2>Validasi saat discovery</h2><ul className="check-list">{['Distribusi waktu respons pertama per cabang, sales, dan jam', 'Porsi inquiry di luar jam kerja', 'Rasio inquiry per lead dan tingkat duplikasi', 'Porsi percakapan di nomor pribadi', 'Konversi lead → SPK menurut kecepatan respons', 'Jumlah pembeli yang masih menerima broadcast'].map(x => <li key={x}><Icon name="check" size={15}/>{x}</li>)}</ul></section></div>
      <section className="panel panel-body"><h2>Prioritas</h2><div className="brief-columns">{[{title:'Quick wins · minggu 1–5', items:['Nomor WA resmi + shared inbox', 'Auto-ack + SLA 15 menit + eskalasi', 'Routing cabang/produk', 'Stop rule pembeli & opt-out']}, {title:'Fondasi · minggu 6–8', items:['Identity resolution lintas channel', 'Integrasi SPK/DO MS SQL', 'Dashboard CRM & penjualan']}, {title:'Skala · minggu 9–12+', items:['IG/FB/TikTok lead ads', 'Campaign tersegmentasi', 'AI ekstraksi, ringkasan, scoring']}].map(c => <div key={c.title}><h3>{c.title}</h3><ul>{c.items.map(x => <li key={x}>{x}</li>)}</ul></div>)}</div></section>
    </>}
    {tab === 'task2' && <>
      <section className="panel"><div className="panel-heading"><div><h2>Standard customer journey · dealer otomotif</h2><p>Satu profil customer; opportunity, status order, dan izin komunikasi dikelola terpisah</p></div><button className="button subtle" onClick={() => go('journey')}>Lihat otomasi<Icon name="arrow" size={15}/></button></div><Rows head={['Tahap', 'Tujuan customer', 'Touchpoint', 'Otomasi sistem', 'Owner & SLA', 'Exit']} rows={JOURNEY}/></section>
      <section className="panel panel-body"><h2>Cadence follow-up prospecting</h2><div className="cadence">{CADENCE.map(([when, title, text]) => <div key={when}><b>{when}</b><span>{title}</span><small>{text}</small></div>)}</div><div className="info-note"><Icon name="shield"/><span>Stop rules: customer membalas (kembali ke percakapan), SPK/DO terbit, opt-out, status Lost, atau frequency cap 2 pesan marketing per minggu tercapai.</span></div></section>
      <div className="root-grid">{[{t:'Lost', x:'Wajib alasan (harga, kompetitor, kredit ditolak, tidak jadi beli). Masuk analisis win/loss.'}, {t:'Nurture', x:'Belum siap beli. Konten bulanan s.d. 90 hari, skor dihitung ulang saat ada respons.'}, {t:'Opt-out', x:'Semua komunikasi marketing berhenti seketika; layanan transaksional tetap.'}, {t:'Purchased', x:'SPK/DO dari ERP menghentikan prospecting dan memulai journey order & aftersales.'}, {t:'Kontinuitas', x:'Pindah channel atau sales tidak mengulang: profil, timeline, dan ringkasan ikut berpindah.'}].map(c => <section className="panel panel-body" key={c.t}><Tag tone={c.t === 'Purchased' || c.t === 'Kontinuitas' ? 'positive' : c.t === 'Lost' ? 'danger' : 'neutral'}>{c.t}</Tag><p>{c.x}</p></section>)}</div>
    </>}
    {tab === 'task3' && <>
      <section className="panel panel-body"><div className="eyebrow">TASK 3 · HIGH-LEVEL ARCHITECTURE</div><h2 className="integration-title">Event-driven, API-first, dengan DMS/ERP MS SQL tetap sebagai sumber kebenaran transaksi</h2><div className="layers">{LAYERS.map(l => <div key={l.title} className="layer"><b>{l.title}</b><div>{l.items.map(i => <span key={i}>{i}</span>)}</div></div>)}</div><div className="layer-side"><span><b>AI gateway</b>masking, policy, log, biaya</span><span><b>Keamanan</b>SSO Entra ID, RBAC (sales/cabang/manajemen), audit, enkripsi, backup</span><span><b>Stack</b>Next.js + TypeScript, Go API & worker, MySQL/MS SQL, queue, Docker, CI/CD</span></div></section>
      <section className="panel panel-body"><h2>Otomasi lead dari semua channel</h2><div className="flow-chips">{['Terima webhook', 'Verifikasi signature', 'Dedupe event', 'Normalisasi (E.164, produk)', 'Identity resolution', 'Enrich sumber & consent', 'Skor', 'Routing', 'SLA timer', 'Notifikasi sales'].map((s, i) => <span key={s}><small>{String(i + 1).padStart(2, '0')}</small>{s}</span>)}</div><div className="source-grid">{SOURCES.map(s => <div key={s.name}><b>{s.name}</b><p>{s.text}</p></div>)}</div></section>
      <section className="panel"><div className="panel-heading"><div><h2>Katalog otomasi</h2><p>Trigger → kondisi → aksi → KPI. Semua aksi tercatat di audit log</p></div></div><Rows head={['Trigger', 'Kondisi', 'Aksi otomatis', 'KPI']} rows={AUTOMATIONS}/></section>
    </>}
    {tab === 'sales' && <>
      <div className="dashboard-grid"><section className="panel panel-body"><div className="eyebrow">IDE</div><h2 className="integration-title">Satu layar untuk menjawab: berapa yang terjual, dari mana, oleh siapa, dan apakah on-target</h2><Rows head={['Pengguna', 'Keputusan yang didukung']} rows={[['Direksi', 'Pencapaian vs target, tren, mix produk, kontribusi channel digital'], ['Kepala cabang', 'Pace target harian, backlog SPK, performa sales'], ['Supervisor sales', 'Leaderboard, SPK batal, konversi lead → SPK per sales'], ['Marketing', 'ROI channel: lead → SPK → DO per sumber']]}/></section>
        <section className="panel panel-body"><h2>KPI utama</h2><ul className="check-list">{['SPK (gross) dan SPK batal', 'DO / retail unit dan revenue DO', 'Pencapaian vs target bulanan + pace', 'Outstanding SPK (backlog) per akhir periode', 'Porsi kredit vs cash', 'Konversi lead → SPK per sumber', 'Leaderboard sales dan mix produk', 'Tren 6 bulan'].map(x => <li key={x}><Icon name="check" size={15}/>{x}</li>)}</ul><button className="button primary" onClick={() => go('sales')}>Buka Sales Dashboard<Icon name="arrow" size={16}/></button></section></div>
      <section className="panel panel-body"><h2>Arsitektur & integrasi MS SQL</h2><div className="architecture-flow"><div><b>DMS/ERP · MS SQL</b><span>SPK, DO, faktur, stok · OLTP tidak dibebani query dashboard</span></div><Icon name="arrow"/><div><b>Change Tracking + Go worker</b><span>Inkremental ±5 menit, watermark, rekonsiliasi malam</span></div><Icon name="arrow"/><div><b>onecrm_reporting.rpt</b><span>Star schema + rpt.usp_SalesDashboard (6 result set)</span></div><Icon name="arrow"/><div><b>Go API → Next.js</b><span>Login EXECUTE-only, TLS, RLS cabang, label kesegaran data</span></div></div><p className="fine">Source code: backend/sql/mssql (skema, prosedur, keamanan, seed sintetis) dan backend/sales.go. Prosedur diuji terhadap SQL Server 2022 dengan angka yang identik dengan simulasi di browser.</p></section>
    </>}
    {tab === 'ai' && <section className="panel panel-body"><div className="eyebrow">PENGGUNAAN AI</div><h2 className="integration-title">Sudah diterapkan: draft follow-up DeepSeek melalui Go API dengan data minimal dan review manusia</h2><p>Roadmap berikutnya: ekstraksi intent, ringkasan handover, draft berbasis knowledge base resmi (pilot); bot di luar jam kerja, lead scoring, next-best-action, deteksi komplain dan anomali (fase 2); forecast, "tanya data", OCR dokumen kredit, voice note (fase 3).</p><button className="button primary" onClick={() => go('ai')}>Buka AI Playbook<Icon name="arrow" size={16}/></button></section>}
    {tab === 'roadmap' && <>
      <section className="panel panel-body"><h2>Delivery plan & pilot gates</h2><div className="roadmap">{ROADMAP.map(s => <div key={s.week}><small>{s.week}</small><h3>{s.title}</h3><p>{s.text}</p></div>)}</div><p className="metric-note">Estimasi usulan, bergantung akses API, kesiapan DMS/ERP, kapasitas tim, dan hasil pilot. Gate: replay event tidak menggandakan lead, pembeli/opt-out tidak menerima prospecting, audit SLA valid, angka dashboard cocok dengan sumber MS SQL.</p></section>
      <section className="panel"><div className="panel-heading"><div><h2>Risiko & mitigasi</h2></div></div><Rows head={['Risiko', 'Mitigasi']} rows={RISKS}/></section>
      <section className="panel panel-body source-list"><h2>Referensi teknis</h2>{[['WhatsApp Business Messaging Policy', 'https://whatsappbusiness.com/policy/'], ['Microsoft SQL Server · Change Tracking & CDC', 'https://learn.microsoft.com/en-us/sql/relational-databases/track-changes/track-data-changes-sql-server'], ['Microsoft SQL Server · Row-Level Security', 'https://learn.microsoft.com/en-us/sql/relational-databases/security/row-level-security'], ['Microsoft go-mssqldb driver', 'https://github.com/microsoft/go-mssqldb'], ['TikTok for Business · Leads API', 'https://business-api.tiktok.com/portal/docs/leads/v1.3'], ['DeepSeek API · Server-side integration', 'https://api-docs.deepseek.com/']].map(([t, href]) => <a key={href} href={href} target="_blank" rel="noreferrer">{t} <Icon name="arrow" size={14}/></a>)}</section>
    </>}
  </>;
}
