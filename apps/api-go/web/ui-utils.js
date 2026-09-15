// VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
export function cents(text){if(!/^\d+(\.\d{1,2})?$/.test(String(text)))throw new Error('Use a nonnegative amount with at most two decimals.');const [a,b='']=String(text).split('.');const n=Number(a)*100+Number(b.padEnd(2,'0'));if(!Number.isSafeInteger(n))throw new Error('Amount too large.');return n;}
export function money(v){return typeof v==='number'&&Number.isFinite(v)?new Intl.NumberFormat('en-US',{style:'currency',currency:'USD',maximumFractionDigits:2}).format(v/100):'Unverified';}
export function percent(v){return typeof v==='number'?(v/100).toFixed(2)+'%':'N/A';}
export function safeLink(url){try{const u=new URL(url);return u.protocol==='https:'&&!u.username?u.href:null;}catch{return null;}}
export function dateOnly(){const d=new Date();return `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')}`;}
