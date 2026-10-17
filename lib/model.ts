export interface Message {id:string; direction:string; text:string; at:string}
export interface Lead {id:string; customerId:string; name:string; channel:string; branch:string; owner:string|null; product:string; stage:string; value:number; createdAt:string; firstReplyMins:number|null; marketingConsent:boolean; optedOut:boolean; intent:string; score:number; inquiries:number; contact:string; closedAt:string|null; nextAction:string; messages:Message[]; history:{text:string;at:string}[]}
export interface State {version:number;leads:Lead[];activity:{text:string;at:string}[];ingestedEvents:string[];health:{erp:boolean};rules:{routing:boolean;sla:boolean;aftersales:boolean};campaigns:{id:string;name:string;ids:string[];status:string}[];lastSyncAt:string}
export interface Filters {days?:number;branch?:string;channel?:string;search?:string}
export const SNAPSHOT = new Date('2026-10-17T14:30:00+07:00');
export const CHANNELS = ['WhatsApp', 'Instagram', 'Facebook', 'TikTok', 'Website'];
export const BRANCHES = ['Manado', 'Palu', 'Kendari', 'Gorontalo'];
export const STAGES = ['New', 'Qualified', 'Test drive', 'Proposal', 'Order', 'Won'];
export const REPS = ['Nadia', 'Raka', 'Maya', 'Fajar'];
const names = ['Aditya Pratama', 'Citra Lestari', 'Bima Saputra', 'Dewi Anggraini', 'Eka Santoso', 'Farah Putri', 'Gilang Ramadhan', 'Hana Wijaya', 'Indra Permana', 'Julia Maharani', 'Kevin Setiawan', 'Laras Pertiwi', 'Maulana Arif', 'Nabila Sari', 'Oki Kurniawan', 'Putri Amanda', 'Rizky Firmansyah', 'Sinta Ayu', 'Teguh Wibowo', 'Utami Dewanti', 'Vino Saputra', 'Wulan Safitri', 'Yuda Prakoso', 'Zahra Anindya', 'Ardi Nugraha', 'Bella Kirana', 'Dimas Putra', 'Elsa Wati', 'Fikri Hidayat', 'Gita Rahayu', 'Hadi Susanto', 'Intan Melati', 'Joko Purnomo', 'Karina Dewi', 'Lukman Hakim', 'Mega Puspita'];
const products = ['MPV Family', 'SUV Urban', 'Scooter 155'];
const day = 86400000;
export function seedState():State {
  const leads = names.map((name, i) => {
    const ageDays = i < 6 ? 0 : 1 + ((i * 7) % 27);
    const createdAt = new Date(SNAPSHOT.getTime() - ageDays * day - (i < 6 ? [46, 35, 22, 18, 8, 3][i] : 40) * 60000).toISOString();
    const stage = i < 4 ? 'New' : i === 4 ? 'Qualified' : i === 5 ? 'New' : [...STAGES, 'Lost', 'Nurture'][i % 8];
    const firstReplyMins = i < 4 || i === 5 ? null : [4, 8, 11, 19, 7, 24][i % 6];
    return {
      id: `L-${String(1001 + i)}`, customerId: `C-${String(2001 + i)}`, name,
      channel: CHANNELS[i % 5], branch: BRANCHES[i % 4], owner: i < 2 ? null : REPS[i % 4],
      product: products[i % 3], stage, value: [285000000, 340000000, 32000000][i % 3],
      createdAt, firstReplyMins, marketingConsent: i % 6 !== 0, optedOut: i === 9,
      intent: ['Minta simulasi cicilan', 'Jadwalkan test drive', 'Cek stok dan warna'][i % 3],
      score: 52 + (i * 7) % 45, inquiries: 1 + i % 4,
      contact: `+62 8xx-xxxx-${String(1000 + i)}`, closedAt: stage === 'Won' ? new Date(new Date(createdAt).getTime() + day / 4).toISOString() : null,
      nextAction: stage === 'Won' ? 'Konfirmasi jadwal serah terima' : stage === 'Lost' ? 'Evaluasi alasan kehilangan' : 'Konfirmasi kebutuhan dan waktu dihubungi',
      messages: [{ id: `m-${i}-1`, direction: 'in', text: `Halo, saya tertarik ${products[i % 3]}. Bisa bantu ${['simulasi cicilan', 'jadwal test drive', 'cek stok dan warna'][i % 3]}?`, at: createdAt }, ...(firstReplyMins === null ? [] : [{ id: `m-${i}-2`, direction: 'out', text: `Selamat siang, Bapak/Ibu ${name.split(' ')[0]}. Saya ${REPS[i % 4]} dari tim sales. Boleh saya konfirmasi kebutuhan Anda?`, at: new Date(new Date(createdAt).getTime() + firstReplyMins * 60000).toISOString() }])],
      history: [{ text: `Inquiry dari ${CHANNELS[i % 5]} masuk`, at: createdAt }, { text: stage === 'Won' ? 'Contoh order ERP terverifikasi. Prospecting dihentikan.' : `Status saat snapshot: ${stage}`, at: createdAt }]
    };
  });
  return {version: 1, leads, activity: [], ingestedEvents: [], health: {erp: true}, rules: {routing: true, sla: true, aftersales: true}, campaigns: [], lastSyncAt: SNAPSHOT.toISOString()};
}
export function cohort(state:State, filters:Filters = {}) {
  const cutoff = SNAPSHOT.getTime() - Number(filters.days || 30) * day;
  return state.leads.filter(l => new Date(l.createdAt).getTime() >= cutoff && (!filters.branch || l.branch === filters.branch) && (!filters.channel || l.channel === filters.channel) && (!filters.search || `${l.name} ${l.id} ${l.product} ${l.owner || ''}`.toLowerCase().includes(filters.search.toLowerCase())));
}
export function slaStatus(lead:Lead) {
  if (lead.firstReplyMins !== null) return lead.firstReplyMins <= 15 ? 'met' : 'breached';
  return (SNAPSHOT.getTime() - new Date(lead.createdAt).getTime()) / 60000 > 15 ? 'breached' : 'waiting';
}
export function metrics(leads:Lead[]) {
  const evaluated = leads.filter(l => slaStatus(l) !== 'waiting');
  const met = evaluated.filter(l => slaStatus(l) === 'met').length;
  const won = leads.filter(l => l.stage === 'Won');
  return {leads: leads.length, inquiries: leads.reduce((s, l) => s + l.inquiries, 0), won: won.length, conversion: leads.length ? won.length / leads.length * 100 : 0, revenue: won.reduce((s, l) => s + l.value, 0), pipeline: leads.filter(l => STAGES.includes(l.stage) && l.stage !== 'Won').reduce((s,l) => s + l.value, 0), overdue: leads.filter(l => l.firstReplyMins === null && slaStatus(l) === 'breached').length, sla: evaluated.length ? met / evaluated.length * 100 : null, slaMet: met, slaEvaluated: evaluated.length};
}
export function campaignEligibility(lead:Lead, erpHealthy:boolean) {
  if (!erpHealthy) return {eligible: false, reason: 'ERP tidak segar'};
  if (lead.optedOut) return {eligible: false, reason: 'Opt-out'};
  if (!lead.marketingConsent) return {eligible: false, reason: 'Tanpa consent marketing'};
  if (['Won', 'Order'].includes(lead.stage)) return {eligible: false, reason: 'Sudah membeli / order aktif'};
  if (lead.stage === 'Lost') return {eligible: false, reason: 'Closed lost'};
  return {eligible: true, reason: 'Layak menerima prospecting'};
}
export function logEvent(state:State, text:string) {state.activity.unshift({text, at: SNAPSHOT.toISOString()});}
export function purchaseEvent(state:State, id:string) {
  if (!state.health.erp) return false;
  const lead = state.leads.find(l => l.id === id);
  if (!lead || lead.stage === 'Won') return false;
  lead.stage = 'Won'; lead.closedAt = SNAPSHOT.toISOString();
  lead.nextAction = 'Konfirmasi jadwal serah terima';
  lead.history.push({text: 'Simulasi order ERP terverifikasi. Status Won dan prospecting dihentikan.', at: SNAPSHOT.toISOString()});
  logEvent(state, `${lead.name}: event pembelian → Won. Antrean prospecting diperiksa ulang saat pengiriman.`);
  return true;
}
export function addLead(state:State, input:{name:string;branch:string;channel:string;product:string;marketingConsent:boolean}) {
  if (!input.name?.trim() || !BRANCHES.includes(input.branch) || !CHANNELS.includes(input.channel) || !['MPV Family','SUV Urban','Scooter 155'].includes(input.product)) throw new Error('Nama, cabang, channel, dan produk wajib valid.');
  const lead = {id: `L-${Math.max(1000,...state.leads.map(l => Number(l.id.slice(2)))) + 1}`, customerId: `C-demo-${state.leads.length+1}`, name: input.name.trim().slice(0,80), branch: input.branch, channel: input.channel, owner: state.rules.routing ? REPS[BRANCHES.indexOf(input.branch)] : null, product: input.product, stage: 'New', value: 285000000, createdAt: SNAPSHOT.toISOString(), firstReplyMins: null, marketingConsent: Boolean(input.marketingConsent), optedOut: false, intent: 'Inquiry baru', score: 60, inquiries: 1, contact: 'Identitas kontak simulasi', closedAt: null, nextAction: 'Konfirmasi kebutuhan dan waktu dihubungi', messages: [{id: `m-${state.leads.length}-new`, direction:'in',text:`Saya tertarik ${input.product}. Mohon informasinya.`,at:SNAPSHOT.toISOString()}], history:[{text:'Lead simulasi dibuat. SLA mulai dihitung.',at:SNAPSHOT.toISOString()}]};
  state.leads.push(lead); logEvent(state, `${lead.id}: inquiry baru dari ${lead.channel}, owner ${lead.owner || 'belum ditentukan'}.`);return lead;
}
export function ingestEvent(state:State, eventId:string) {
  if (state.ingestedEvents.includes(eventId)) return {duplicate:true};
  state.ingestedEvents.push(eventId);
  const lead = addLead(state,{name:'Customer Event Demo',branch:'Manado',channel:'Website',product:'MPV Family',marketingConsent:false});
  logEvent(state,`${eventId}: diterima sekali, replay berikutnya dilewati.`);return {duplicate:false,lead};
}
export function csvCell(value:string|number) {
  let text=String(value);
  if(typeof value==='string' && /^[\s]*[=+@-]/.test(text)) text="'"+text;
  return '"'+text.replace(/"/g,'""')+'"';
}
