const API='http://localhost:8080/api';
let contacts=[],mediaB64='',mediaType='',mediaMime='',mediaName='';
let currentJobId=null,pollTimer=null;
let phoneList=[];

// ── Toast ──────────────────────────────────────────────────────
function toast(msg,type='info'){
  const w=document.getElementById('toast-wrap');
  const t=document.createElement('div');
  t.className=`toast ${type}`;
  const icons={success:'✅',error:'❌',info:'💡'};
  t.innerHTML=`<span>${icons[type]||'ℹ️'}</span><span>${msg}</span>`;
  w.appendChild(t);setTimeout(()=>t.remove(),4500);
}

// ── Tabs ───────────────────────────────────────────────────────
document.querySelectorAll('.tab-btn').forEach(b=>{
  b.addEventListener('click',()=>{
    document.querySelectorAll('.tab-btn,.tab-pane').forEach(x=>x.classList.remove('active'));
    b.classList.add('active');
    document.getElementById(b.dataset.tab).classList.add('active');
    if(b.dataset.tab==='tab-jobs') refreshJobs();
    if(b.dataset.tab==='tab-login') syncLoginTab();
  });
});

// ── Status Polling ─────────────────────────────────────────────
async function checkStatus(){
  try{
    const d=await fetch(`${API}/status`).then(r=>r.json());
    const dot=document.getElementById('status-dot');
    const lbl=document.getElementById('status-lbl');
    const jidEl=document.getElementById('jid-text');
    const logoutBtn=document.getElementById('btn-logout');
    const loginBtn=document.getElementById('btn-login');
    dot.className='status-dot';
    if(d.status==='connected'){
      dot.classList.add('connected');lbl.textContent='Connected';
      jidEl.textContent=d.jid||'';jidEl.classList.remove('hidden');
      logoutBtn.classList.remove('hidden');loginBtn.classList.add('hidden');
    }else if(d.status==='awaiting_login'){
      dot.classList.add('awaiting');lbl.textContent='Scan QR';
      jidEl.classList.add('hidden');logoutBtn.classList.add('hidden');loginBtn.classList.remove('hidden');
    }else{
      lbl.textContent='Disconnected';
      jidEl.classList.add('hidden');logoutBtn.classList.add('hidden');loginBtn.classList.remove('hidden');
    }
    syncLoginTab(d);
  }catch(e){document.getElementById('status-lbl').textContent='Offline';}
}
setInterval(checkStatus,4000);checkStatus();

function syncLoginTab(d){
  if(!d) return;
  const cdot=document.getElementById('conn-dot-lg');
  const ctxt=document.getElementById('conn-status-text');
  const cjid=document.getElementById('conn-jid');
  if(!cdot) return;
  cdot.className='conn-dot-lg';
  if(d.status==='connected'){
    cdot.classList.add('connected');ctxt.textContent='Connected ✅';
    cjid.textContent=d.jid||'';
  }else{
    cdot.classList.add('awaiting');ctxt.textContent='API Not Configured ⏳';
    cjid.textContent='Check .env file';
  }
}

// Header Login button
document.getElementById('btn-login').addEventListener('click',()=>{
  document.getElementById('qr-modal').classList.remove('hidden');
});
document.getElementById('btn-close-qr').addEventListener('click',()=>document.getElementById('qr-modal').classList.add('hidden'));

// Header Logout (Refresh Status)
document.getElementById('btn-logout').addEventListener('click',async()=>{
  toast('Refreshing status...','info');checkStatus();
});

// Login tab buttons
if(document.getElementById('btn-hard-logout')) {
  document.getElementById('btn-hard-logout').addEventListener('click',async()=>{
    checkStatus();
    toast('Connection status refreshed','info');
  });
}

// ── Contact Mode Toggle ────────────────────────────────────────
document.querySelectorAll('.mode-btn').forEach(b=>{
  b.addEventListener('click',()=>{
    document.querySelectorAll('.mode-btn').forEach(x=>x.classList.remove('active'));
    b.classList.add('active');
    const mode=b.dataset.mode;
    document.getElementById('contacts-csv-mode').classList.toggle('hidden',mode==='phone');
    document.getElementById('contacts-phone-mode').classList.toggle('hidden',mode==='csv');
  });
});

// ── CSV / Excel ────────────────────────────────────────────────
const dropZone=document.getElementById('drop-zone');
const fileInput=document.getElementById('file-input');
const csvText=document.getElementById('csv-text');

dropZone.addEventListener('dragover',e=>{e.preventDefault();dropZone.classList.add('dragover')});
dropZone.addEventListener('dragleave',()=>dropZone.classList.remove('dragover'));
dropZone.addEventListener('drop',e=>{e.preventDefault();dropZone.classList.remove('dragover');handleFile(e.dataTransfer.files[0])});
fileInput.addEventListener('change',()=>handleFile(fileInput.files[0]));

function handleFile(file){
  if(!file) return;
  const ext=file.name.split('.').pop().toLowerCase();
  if(ext==='csv'||ext==='txt'){
    const r=new FileReader();
    r.onload=e=>{csvText.value=e.target.result;parseCSV(e.target.result);};
    r.readAsText(file);
  }else if(ext==='xlsx'||ext==='xls'){
    const r=new FileReader();
    r.onload=e=>{
      const wb=XLSX.read(e.target.result,{type:'array'});
      const csv=XLSX.utils.sheet_to_csv(wb.Sheets[wb.SheetNames[0]]);
      csvText.value=csv;parseCSV(csv);
    };
    r.readAsArrayBuffer(file);
  }else{toast('Use CSV or XLSX files','error');}
}
csvText.addEventListener('input',()=>parseCSV(csvText.value));

function splitCSVLine(line){
  const res=[];let cur='',inQ=false;
  for(const c of line){
    if(c==='"'){inQ=!inQ;}
    else if(c===','&&!inQ){res.push(cur);cur='';}
    else{cur+=c;}
  }
  res.push(cur);return res;
}

function parseCSV(text){
  const lines=text.trim().split('\n').filter(l=>l.trim());
  if(lines.length<2){contacts=[];renderTable([]);return;}
  const headers=lines[0].split(',').map(h=>h.trim().replace(/^"|"$/g,'').toLowerCase());
  contacts=lines.slice(1).map(line=>{
    const vals=splitCSVLine(line);
    const row={};
    headers.forEach((h,i)=>row[h]=(vals[i]||'').trim().replace(/^"|"$/g,''));
    return{phone:row.phone||row.number||row.mobile||'',variables:row,status:null};
  }).filter(c=>c.phone);
  renderTable(contacts);
  document.getElementById('contact-count').textContent=`${contacts.length} contacts loaded`;
  document.getElementById('btn-validate-all').classList.remove('hidden');
  updatePreview();
}

function renderTable(rows){
  const wrap=document.getElementById('csv-table-wrap');
  if(!rows.length){wrap.innerHTML='';return;}
  const headers=Object.keys(rows[0].variables);
  const statusCol='<th>✅ WA</th>';
  const waCol='<th>Open</th>';
  const delCol='<th>Action</th>';
  wrap.innerHTML=`<table><thead><tr>${statusCol}${headers.map(h=>`<th>${h}</th>`).join('')}${waCol}${delCol}</tr></thead>
  <tbody>${rows.slice(0,100).map((r,i)=>{
    const tick=r.status===true?'<span style="color:#90c74a">✅</span>':r.status===false?'<span style="color:#ef5350">❌</span>':'<span style="color:#3d5575">—</span>';
    const waNum=r.phone.replace(/\D/g,'');
    const waLink=`<a href="https://wa.me/${waNum}" target="_blank" class="wa-link-cell">💬 Open</a>`;
    const delBtn=`<button class="btn-del" onclick="removeCSVContact(${i})" style="color:var(--danger);background:none;border:none;cursor:pointer;font-size:14px;padding:2px 6px">✕</button>`;
    return `<tr>${`<td>${tick}</td>`}${headers.map(h=>`<td>${r.variables[h]||''}</td>`).join('')}<td>${waLink}</td><td>${delBtn}</td></tr>`;
  }).join('')}
  ${rows.length>100?`<tr><td colspan="${headers.length+3}" style="color:#3d5575;text-align:center">...and ${rows.length-100} more</td></tr>`:''}</tbody></table>`;
}

function removeCSVContact(i){
  contacts.splice(i,1);
  renderTable(contacts);
  document.getElementById('contact-count').textContent=`${contacts.length} contacts loaded`;
  if(contacts.length===0) document.getElementById('btn-validate-all').classList.add('hidden');
  updatePreview();
}

// Validate all CSV contacts
document.getElementById('btn-validate-all').addEventListener('click',async()=>{
  if(!contacts.length){toast('No contacts loaded','error');return;}
  toast(`Validating ${contacts.length} numbers...`,'info');
  for(let i=0;i<contacts.length;i++){
    try{
      const d=await fetch(`${API}/check-number`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({phone:contacts[i].phone})}).then(r=>r.json());
      contacts[i].status=d.on_whatsapp===true;
    }catch(e){contacts[i].status=null;}
    if(i%5===0) renderTable(contacts);
    await new Promise(r=>setTimeout(r,500));
  }
  renderTable(contacts);
  const ok=contacts.filter(c=>c.status===true).length;
  toast(`Validation done: ${ok}✅ / ${contacts.length-ok}❌`,'success');
});

// ── Phone Number Mode ──────────────────────────────────────────
document.getElementById('btn-add-phone').addEventListener('click',addPhone);
document.getElementById('single-phone').addEventListener('keydown',e=>{if(e.key==='Enter') addPhone();});

function addPhone(){
  const inp=document.getElementById('single-phone');
  const num=inp.value.trim().replace(/\D/g,'');
  if(!num){toast('Enter a phone number','error');return;}
  if(phoneList.find(p=>p.phone===num)){toast('Already added','info');return;}
  phoneList.push({phone:num,status:null});
  inp.value='';
  renderPhoneList();
  document.getElementById('phone-count').textContent=`${phoneList.length} numbers`;
  syncContactsFromPhoneList();
}

function renderPhoneList(){
  const el=document.getElementById('phone-list');
  el.innerHTML=phoneList.map((p,i)=>{
    const tick=p.status===true?'<span class="status-tick" style="color:#90c74a">✅</span>':p.status===false?'<span class="status-tick" style="color:#ef5350">❌</span>':'<span class="status-tick" style="color:#3d5575">—</span>';
    const waLink=`<a href="https://wa.me/${p.phone}" target="_blank" class="wa-link">💬</a>`;
    return `<div class="phone-row">${tick}<span style="font-size:13px">${p.phone}</span>${waLink}<button class="btn-del" onclick="removePhone(${i})">✕</button></div>`;
  }).join('');
}

function removePhone(i){phoneList.splice(i,1);renderPhoneList();syncContactsFromPhoneList();}

function syncContactsFromPhoneList(){
  contacts=phoneList.map(p=>({phone:p.phone,variables:{phone:p.phone},status:p.status}));
}

document.getElementById('btn-validate-phones').addEventListener('click',async()=>{
  if(!phoneList.length){toast('No numbers added','error');return;}
  toast(`Validating ${phoneList.length} numbers...`,'info');
  for(let i=0;i<phoneList.length;i++){
    try{
      const d=await fetch(`${API}/check-number`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({phone:phoneList[i].phone})}).then(r=>r.json());
      phoneList[i].status=d.on_whatsapp===true;
    }catch(e){phoneList[i].status=null;}
    renderPhoneList();
    await new Promise(r=>setTimeout(r,500));
  }
  syncContactsFromPhoneList();
  const ok=phoneList.filter(p=>p.status===true).length;
  toast(`Validation done: ${ok}✅ / ${phoneList.length-ok}❌`,'success');
});

// ── Template Preview ───────────────────────────────────────────
document.getElementById('template').addEventListener('input',updatePreview);
function updatePreview(){
  const tmpl=document.getElementById('template').value;
  const sample=contacts[0]?.variables||{name:'Rahul',company:'Acme',offer:'20% off'};
  let preview=tmpl;
  Object.entries(sample).forEach(([k,v])=>{preview=preview.replaceAll(`{${k}}`,v);});
  document.getElementById('preview-bubble').textContent=preview||'Your message will appear here...';
}

// ── Media ──────────────────────────────────────────────────────
document.querySelectorAll('.media-pill').forEach(p=>{
  p.addEventListener('click',()=>{
    document.querySelectorAll('.media-pill').forEach(x=>x.classList.remove('active'));
    p.classList.add('active');
    mediaType=p.dataset.type;
    const accept=mediaType==='image'?'image/*':mediaType==='video'?'video/*':'audio/*';
    document.getElementById('media-input').setAttribute('accept',accept);
  });
});
const mediaDrop=document.getElementById('media-drop');
mediaDrop.addEventListener('dragover',e=>{e.preventDefault();mediaDrop.classList.add('dragover')});
mediaDrop.addEventListener('dragleave',()=>mediaDrop.classList.remove('dragover'));
mediaDrop.addEventListener('drop',e=>{e.preventDefault();mediaDrop.classList.remove('dragover');handleMedia(e.dataTransfer.files[0])});
document.getElementById('media-input').addEventListener('change',e=>handleMedia(e.target.files[0]));

function handleMedia(file){
  if(!file) return;
  mediaName=file.name;mediaMime=file.type;
  if(!mediaType){
    if(file.type.startsWith('image')) mediaType='image';
    else if(file.type.startsWith('video')) mediaType='video';
    else if(file.type.startsWith('audio')) mediaType='audio';
    document.querySelectorAll('.media-pill').forEach(p=>p.classList.toggle('active',p.dataset.type===mediaType));
  }
  const reader=new FileReader();
  reader.onload=e=>{
    mediaB64=e.target.result.split(',')[1];
    document.getElementById('media-preview').innerHTML=
      mediaType==='image'?`<img src="${e.target.result}" style="max-height:120px;border-radius:8px;border:2px solid var(--border-bright)">`:
      mediaType==='video'?`<video src="${e.target.result}" controls style="max-height:120px;border-radius:8px">`:
      `<audio src="${e.target.result}" controls style="width:100%">`;
    document.getElementById('media-name').textContent=file.name;
    document.getElementById('btn-clear-media').classList.remove('hidden');
  };
  reader.readAsDataURL(file);
}
document.getElementById('btn-clear-media').addEventListener('click',()=>{
  mediaB64='';mediaType='';mediaMime='';mediaName='';
  document.getElementById('media-preview').innerHTML='';
  document.getElementById('media-name').textContent='';
  document.getElementById('btn-clear-media').classList.add('hidden');
  document.getElementById('media-input').value='';
  document.querySelectorAll('.media-pill').forEach(p=>p.classList.remove('active'));
});

// ── Schedule ───────────────────────────────────────────────────
document.querySelectorAll('.sched-btn').forEach(b=>{
  b.addEventListener('click',()=>{
    document.querySelectorAll('.sched-btn').forEach(x=>x.classList.remove('active'));
    b.classList.add('active');
    const mode=b.dataset.sched;
    document.getElementById('sched-custom-row').classList.toggle('hidden',mode==='now');
  });
});

function buildScheduleISO(){
  const mode=document.querySelector('.sched-btn.active')?.dataset.sched||'now';
  if(mode==='now') return '';
  const val=document.getElementById('schedule-dt').value;
  return val?new Date(val).toISOString():'';
}

// ── Send ───────────────────────────────────────────────────────
document.getElementById('btn-send').addEventListener('click',async()=>{
  if(!contacts.length){toast('No contacts loaded','error');return;}
  const tmpl=document.getElementById('template').value.trim();
  if(!tmpl){toast('Message template is empty','error');return;}

  const scheduleAt=buildScheduleISO();
  const delayMin=parseFloat(document.getElementById('delay-min').value)||8;
  const delayMax=parseFloat(document.getElementById('delay-max').value)||20;

  const payload={
    contacts,template:tmpl,
    media_base64:mediaB64,media_type:mediaType,media_mime:mediaMime,media_name:mediaName,
    schedule_at:scheduleAt,delay_min:delayMin,delay_max:delayMax
  };

  const btn=document.getElementById('btn-send');
  btn.disabled=true;btn.innerHTML='<span class="send-icon">⏳</span> Sending...';

  try{
    const d=await fetch(`${API}/bulk-send`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)}).then(r=>r.json());
    if(d.success){
      currentJobId=d.job_id;
      toast(`✅ Job created! Sending to ${d.total} contacts`+(scheduleAt?` at ${new Date(scheduleAt).toLocaleString()}`:''),'success');
      document.querySelectorAll('.tab-btn,.tab-pane').forEach(x=>x.classList.remove('active'));
      document.querySelector('[data-tab="tab-jobs"]').classList.add('active');
      document.getElementById('tab-jobs').classList.add('active');
      refreshJobs();startJobPoll(d.job_id);
    }else{toast(d.error||'Failed to send','error');}
  }catch(e){toast('Cannot reach server — is wa-server.exe running?','error');}
  btn.disabled=false;btn.innerHTML='<span class="send-icon">🚀</span> Send Messages';
});

// ── Jobs ───────────────────────────────────────────────────────
async function refreshJobs(){
  try{
    const jobs=await fetch(`${API}/jobs`).then(r=>r.json());
    renderJobs(Array.isArray(jobs)?jobs:[]);
    const badge=document.getElementById('jobs-badge');
    if(jobs.length){badge.textContent=jobs.length;badge.style.display='inline-block';}
    else badge.style.display='none';
  }catch(e){document.getElementById('jobs-list').innerHTML='<p style="color:var(--text-dim);padding:24px;text-align:center">Cannot reach server</p>';}
}

function renderJobs(jobs){
  const el=document.getElementById('jobs-list');
  if(!jobs.length){
    el.innerHTML='<div class="empty-state"><div class="empty-icon">📭</div><p>No jobs yet. Go to Compose to send messages.</p></div>';
    return;
  }
  jobs.sort((a,b)=>new Date(b.created_at)-new Date(a.created_at));
  el.innerHTML=jobs.map(j=>{
    const pct=j.total?Math.round((j.sent+j.failed)/j.total*100):0;
    const log=(j.log||[]).slice(-20).map(l=>`<p>${l}</p>`).join('');
    const sentColor=j.sent>0?'var(--green)':'var(--text-dim)';
    const failColor=j.failed>0?'var(--danger)':'var(--text-dim)';
    return `<div class="job-item">
      <div class="job-header">
        <span class="job-status ${j.status}">${j.status.toUpperCase()}</span>
        <span class="job-id">${j.id}</span>
        <span class="job-time">${new Date(j.created_at).toLocaleString()}</span>
        <button class="btn-delete-job" onclick="deleteJob('${j.id}')">🗑 Delete</button>
      </div>
      <div class="job-stats">
        <div class="job-stat"><div class="job-stat-val">${j.total}</div><div class="job-stat-lbl">Total</div></div>
        <div class="job-stat"><div class="job-stat-val" style="color:${sentColor}">${j.sent}</div><div class="job-stat-lbl">Sent ✅</div></div>
        <div class="job-stat"><div class="job-stat-val" style="color:${failColor}">${j.failed}</div><div class="job-stat-lbl">Failed ❌</div></div>
      </div>
      <div class="progress-bar"><div class="progress-fill" style="width:${pct}%"></div></div>
      <div style="font-size:11px;color:var(--text-dim);margin-bottom:6px">${pct}% complete</div>
      <div class="job-log">${log||'<p>Waiting to start...</p>'}</div>
    </div>`;
  }).join('');
}

async function deleteJob(id){
  if(!confirm('Delete this job?')) return;
  try{
    const d=await fetch(`${API}/jobs?id=${id}`,{method:'DELETE'}).then(r=>r.json());
    if(d.success){toast('Job deleted','info');refreshJobs();}
    else toast(d.error||'Delete failed','error');
  }catch(e){toast('Cannot reach server','error');}
}

function startJobPoll(jobId){
  if(pollTimer) clearInterval(pollTimer);
  pollTimer=setInterval(async()=>{
    try{
      const j=await fetch(`${API}/jobs?id=${jobId}`).then(r=>r.json());
      refreshJobs();
      if(j.status==='done'||j.status==='failed'){
        clearInterval(pollTimer);pollTimer=null;
        toast(`Job ${j.status}: ${j.sent} sent, ${j.failed} failed`,j.status==='done'?'success':'error');
      }
    }catch(e){}
  },2000);
}

document.getElementById('btn-refresh-jobs').addEventListener('click',refreshJobs);

// ── Init ───────────────────────────────────────────────────────
updatePreview();
// Default date/time = now+5min
const dt=new Date(Date.now()+5*60000);
dt.setSeconds(0,0);
document.getElementById('schedule-dt').value=dt.toISOString().slice(0,16);
