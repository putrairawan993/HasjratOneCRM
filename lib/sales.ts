import {BRANCHES, CHANNELS} from './model.ts';
// Sales analytics contract shared by the standalone simulation, the MS SQL procedure rpt.usp_SalesDashboard and the Go API.
// The dashboard carries counts only; every ratio is derived in the UI so SQL Server and TypeScript cannot drift on rounding.
export interface Spk {spkNo:string; spkDate:string; doDate:string|null; status:'Open'|'Delivered'|'Cancelled'; branch:string; product:string; salesperson:string; source:string; payment:'Cash'|'Kredit'; amount:number; leadId:string|null}
export interface LeadDay {date:string; branch:string; channel:string; leads:number}
export interface Target {month:string; branch:string; product:string; units:number}
export interface SalesData {spk:Spk[]; leads:LeadDay[]; targets:Target[]; asOf:string}
export interface SalesFilter {from:string; to:string; branch?:string}
export interface SalesDashboard {
  source:'mssql'|'simulation'; asOf:string; from:string; to:string; branch:string|null;
  kpi:{spk:number; cancelled:number; delivered:number; revenue:number; target:number; outstanding:number; credit:number; leads:number};
  trend:{month:string; spk:number; delivered:number; target:number}[];
  branches:{branch:string; spk:number; delivered:number; revenue:number; target:number}[];
  products:{product:string; spk:number; delivered:number; revenue:number; target:number}[];
  salespeople:{name:string; branch:string; spk:number; delivered:number; revenue:number}[];
  channels:{channel:string; leads:number; spk:number; delivered:number}[];
}
export const SALES_AS_OF = '2026-10-17';
export const SALES_START = '2026-05-01';
export const SALES_SOURCES = [...CHANNELS, 'Walk-in'];
export const SALES_PRODUCTS = [{name:'MPV Family', segment:'Mobil', price:285000000, monthly:24, leadDays:[7, 30], credit:.72}, {name:'SUV Urban', segment:'Mobil', price:340000000, monthly:14, leadDays:[10, 35], credit:.68}, {name:'Scooter 155', segment:'Motor', price:32000000, monthly:55, leadDays:[2, 10], credit:.62}];
export const SALESPEOPLE = [{name:'Nadia', branch:'Manado'}, {name:'Rani', branch:'Manado'}, {name:'Raka', branch:'Palu'}, {name:'Bayu', branch:'Palu'}, {name:'Maya', branch:'Kendari'}, {name:'Sari', branch:'Kendari'}, {name:'Fajar', branch:'Gorontalo'}, {name:'Dodi', branch:'Gorontalo'}];
export const SALES_PERIODS = [{id:'mtd', label:'Bulan berjalan (MTD)', from:'2026-10-01'}, {id:'3m', label:'3 bulan terakhir', from:'2026-08-01'}, {id:'6m', label:'6 bulan (Mei–Okt)', from:SALES_START}];
const BRANCH_WEIGHT = [1.3, 1, .85, .75], TARGET_FACTOR = [.92, 1, .9, 1.1];
const LEAD_SHARE = [.55, .13, .1, .08, .08, .06], SPK_SHARE = [.42, .11, .08, .06, .1, .23];
const day = 86400000;
export const addDays = (date:string, n:number) => new Date(Date.parse(`${date}T00:00:00Z`) + n * day).toISOString().slice(0, 10);
export const monthOf = (date:string) => date.slice(0, 7);
const nextMonth = (month:string) => {const [y, m] = month.split('-').map(Number); return m === 12 ? `${y + 1}-01` : `${y}-${String(m + 1).padStart(2, '0')}`;};
export function monthsBetween(from:string, to:string) {const months = [monthOf(from)]; while (months.at(-1)! < monthOf(to)) months.push(nextMonth(months.at(-1)!)); return months;}
function mulberry32(seed:number) {return () => {seed = (seed + 0x6D2B79F5) | 0; let t = Math.imul(seed ^ (seed >>> 15), 1 | seed); t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t; return ((t ^ (t >>> 14)) >>> 0) / 4294967296;};}
function pick<T>(items:T[], weights:number[], r:number) {let acc = 0; for (let i = 0; i < items.length; i++) {acc += weights[i]; if (r < acc) return items[i];} return items[items.length - 1];}
export function seedSales():SalesData {
  const rand = mulberry32(20261017), spk:Spk[] = [], leads:LeadDay[] = [], targets:Target[] = [];
  for (let date = SALES_START; date <= SALES_AS_OF; date = addDays(date, 1)) {
    const dow = new Date(`${date}T00:00:00Z`).getUTCDay(), season = (dow === 0 ? .6 : dow === 6 ? 1.15 : 1) * (.94 + .03 * (Number(date.slice(5, 7)) - 5));
    BRANCHES.forEach((branch, b) => {
      SALES_SOURCES.forEach((channel, c) => {const n = Math.round(60 * BRANCH_WEIGHT[b] * LEAD_SHARE[c] * season * (.8 + .4 * rand())); if (n) leads.push({date, branch, channel, leads:n});});
      SALES_PRODUCTS.forEach(p => {
        const expected = p.monthly * BRANCH_WEIGHT[b] * season / 30.4;
        for (let n = Math.floor(expected) + (rand() < expected % 1 ? 1 : 0); n > 0; n--) {
          const seq = spk.length + 1, source = pick(SALES_SOURCES, SPK_SHARE, rand()), people = SALESPEOPLE.filter(s => s.branch === branch);
          const salesperson = rand() < .56 ? people[0].name : people[1].name, doDate = addDays(date, p.leadDays[0] + Math.floor(rand() * (p.leadDays[1] - p.leadDays[0] + 1)));
          const cancelled = rand() < .06, delivered = !cancelled && doDate <= SALES_AS_OF, payment = rand() < p.credit ? 'Kredit' as const : 'Cash' as const;
          spk.push({spkNo:`SPK-${date.slice(2, 4)}${date.slice(5, 7)}-${String(seq).padStart(5, '0')}`, spkDate:date, doDate:delivered ? doDate : null, status:cancelled ? 'Cancelled' : delivered ? 'Delivered' : 'Open', branch, product:p.name, salesperson, source, payment, amount:Math.round(p.price * (1 - .03 * rand()) / 100000) * 100000, leadId:source === 'Walk-in' ? null : `L-${50000 + seq}`});
        }
      });
    });
  }
  monthsBetween(SALES_START, SALES_AS_OF).forEach((month, m) => BRANCHES.forEach((branch, b) => SALES_PRODUCTS.forEach(p => targets.push({month, branch, product:p.name, units:Math.round(p.monthly * BRANCH_WEIGHT[b] * (.94 + .03 * m) * TARGET_FACTOR[b] * 1.04)}))));
  return {spk, leads, targets, asOf:'2026-10-17T07:30:00.000Z'};
}
export function validSalesFilter(f:SalesFilter) {
  const iso = (d:string) => /^\d{4}-\d{2}-\d{2}$/.test(d) && addDays(d, 0) === d;
  return iso(f.from) && iso(f.to) && f.from <= f.to && Date.parse(f.to) - Date.parse(f.from) <= 366 * day && (!f.branch || BRANCHES.includes(f.branch));
}
// Metric definitions (mirrored 1:1 in backend/sql/mssql/002_procedures.sql):
// spk = SPK created in the period, all statuses · cancelled = of those, status Cancelled · delivered/revenue = DO dated in the period
// target = monthly targets of every month the period touches · outstanding = SPK backlog at period end (not cancelled, not yet delivered by `to`)
// credit = delivered units paid through leasing · leads = unique CRM leads created in the period (lead source attribution)
export function salesDashboard(data:SalesData, f:SalesFilter, source:'mssql'|'simulation' = 'simulation'):SalesDashboard {
  if (!validSalesFilter(f)) throw new Error('Periode atau cabang tidak valid.');
  const inBranch = (b:string) => !f.branch || b === f.branch, inRange = (d:string|null) => !!d && d >= f.from && d <= f.to, months = monthsBetween(f.from, f.to);
  const spk = data.spk.filter(s => inBranch(s.branch)), leads = data.leads.filter(l => inBranch(l.branch)), targets = data.targets.filter(t => inBranch(t.branch));
  const tally = (rows:Spk[]) => {const done = rows.filter(s => inRange(s.doDate)); return {spk:rows.filter(s => inRange(s.spkDate)).length, delivered:done.length, revenue:done.reduce((a, s) => a + s.amount, 0)};};
  const target = (rows:Target[]) => rows.filter(t => months.includes(t.month)).reduce((a, t) => a + t.units, 0);
  const done = spk.filter(s => inRange(s.doDate));
  const trendStart = monthsBetween(addDays(`${monthOf(f.to)}-01`, -150), f.to).slice(-6)[0];
  return {
    source, asOf:data.asOf, from:f.from, to:f.to, branch:f.branch || null,
    kpi:{...tally(spk), cancelled:spk.filter(s => inRange(s.spkDate) && s.status === 'Cancelled').length, target:target(targets), outstanding:spk.filter(s => s.spkDate <= f.to && s.status !== 'Cancelled' && (!s.doDate || s.doDate > f.to)).length, credit:done.filter(s => s.payment === 'Kredit').length, leads:leads.filter(l => inRange(l.date)).reduce((a, l) => a + l.leads, 0)},
    trend:monthsBetween(trendStart, f.to).map(month => ({month, spk:spk.filter(s => monthOf(s.spkDate) === month && s.spkDate <= f.to).length, delivered:spk.filter(s => s.doDate && monthOf(s.doDate) === month && s.doDate <= f.to).length, target:targets.filter(t => t.month === month).reduce((a, t) => a + t.units, 0)})),
    branches:BRANCHES.filter(inBranch).map(branch => ({branch, ...tally(spk.filter(s => s.branch === branch)), target:target(targets.filter(t => t.branch === branch))})),
    products:SALES_PRODUCTS.map(p => ({product:p.name, ...tally(spk.filter(s => s.product === p.name)), target:target(targets.filter(t => t.product === p.name))})),
    salespeople:SALESPEOPLE.filter(s => inBranch(s.branch)).map(s => ({name:s.name, branch:s.branch, ...tally(spk.filter(x => x.salesperson === s.name))})).sort((a, b) => b.delivered - a.delivered || b.revenue - a.revenue || a.name.localeCompare(b.name)),
    channels:SALES_SOURCES.map(channel => ({channel, leads:leads.filter(l => l.channel === channel && inRange(l.date)).reduce((a, l) => a + l.leads, 0), spk:spk.filter(s => s.source === channel && inRange(s.spkDate)).length, delivered:spk.filter(s => s.source === channel && inRange(s.doDate)).length}))
  };
}
export const ratio = (part:number, whole:number) => whole ? part / whole * 100 : null;
// Share of the period's full-month targets that should be reached by `to`, for an on-pace reading of a partial month.
export function paceShare(from:string, to:string) {
  const start = Date.parse(`${monthOf(from)}-01T00:00:00Z`), end = Date.parse(`${nextMonth(monthOf(to))}-01T00:00:00Z`);
  return Math.min(1, (Date.parse(`${to}T00:00:00Z`) - start + day) / (end - start));
}
