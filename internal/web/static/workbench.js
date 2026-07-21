const projectID = window.location.pathname.split('/')[2];
const candidates = JSON.parse(document.getElementById('workbench-data')?.textContent || '[]');

function candidateByKey(key){
  return candidates.find(c => c.GroupKey === key);
}

function netHostPort(ip, port){
  return ip + ':' + port;
}

// ---------- Filtering ----------
const filterForm = document.getElementById('workbench-filter-form');
const filterZone = document.getElementById('filter-zone');
const filterService = document.getElementById('filter-service');
const filterRun = document.getElementById('filter-run');
const filterPort = document.getElementById('filter-port');
const filterStatus = document.getElementById('filter-status');
const filterKeyword = document.getElementById('filter-keyword');
const filterReset = document.getElementById('filter-reset');

function populateFilters(){
  const services = new Set();
  const runs = new Set();
  candidates.forEach(c => {
    c.Services?.forEach(s => services.add(s));
    c.SourceRuns?.forEach(r => runs.add(r));
  });
  [...services].sort().forEach(s => {
    const opt = document.createElement('option');
    opt.value = s; opt.textContent = s;
    filterService.appendChild(opt);
  });
  [...runs].sort().forEach(r => {
    const opt = document.createElement('option');
    opt.value = r; opt.textContent = r;
    filterRun.appendChild(opt);
  });
}

function cardMatches(card){
  const zone = filterZone.value;
  if(zone && card.dataset.zone !== zone) return false;
  const severityBoxes = [...filterForm.querySelectorAll('.filter-severity:checked')].map(b => b.value);
  if(severityBoxes.length && !severityBoxes.includes(card.dataset.severity)) return false;
  const service = filterService.value;
  if(service && !(card.dataset.services || '').split(/\s+/).filter(Boolean).includes(service)) return false;
  const port = filterPort.value.trim();
  if(port && !(card.dataset.ports || '').split(/\s+/).filter(Boolean).includes(port)) return false;
  const run = filterRun.value;
  if(run && !(card.dataset.runs || '').split(/\s+/).filter(Boolean).includes(run)) return false;
  const status = filterStatus.value;
  if(status && card.dataset.status !== status) return false;
  const keyword = filterKeyword.value.trim().toLowerCase();
  if(keyword && !(card.dataset.title || '').toLowerCase().includes(keyword)) return false;
  return true;
}

function applyFilters(){
  document.querySelectorAll('.candidate-card').forEach(card => {
    card.style.display = cardMatches(card) ? '' : 'none';
  });
}

if(filterForm){
  populateFilters();
  filterForm.addEventListener('change', applyFilters);
  filterKeyword.addEventListener('input', applyFilters);
  filterPort.addEventListener('input', applyFilters);
  filterReset.addEventListener('click', () => {
    filterForm.reset();
    applyFilters();
  });
  applyFilters();
}

// ---------- Command generation ----------
const commandDialog = document.getElementById('command-dialog');
const commandDialogTitle = document.getElementById('command-dialog-title');
const commandDialogMessage = document.getElementById('command-dialog-message');
const commandDialogBody = document.getElementById('command-dialog-body');
const commandDialogTool = document.getElementById('command-dialog-tool');
const commandDialogCopy = document.getElementById('command-dialog-copy');
const commandDialogClose = document.getElementById('command-dialog-close');

function showCommandDialog(){
  commandDialogMessage.textContent = '正在生成，不会启动扫描。';
  commandDialogBody.textContent = '';
  commandDialogTool.hidden = true;
  commandDialog.showModal();
}

commandDialogClose?.addEventListener('click', () => commandDialog.close());

async function fetchCommand(key, tool, asset, verificationID){
  const url = '/projects/' + projectID + '/candidates/' + encodeURIComponent(key) + '/commands';
  const body = new URLSearchParams({tool});
  if(asset) body.set('asset', asset);
  if(verificationID) body.set('verification_id', verificationID);
  const returnPath = '/projects/' + projectID + '/workbench';
  body.set('return', returnPath);
  const res = await fetch(url, {method: 'POST', headers: {'Content-Type': 'application/x-www-form-urlencoded'}, body});
  if(!res.ok) throw new Error((await res.text()).trim() || '命令不可用');
  return res.json();
}

function renderCommandResult(tool, data){
  const commands = data.commands || [];
  commandDialogTitle.textContent = '生成 ' + tool + ' 命令';
  commandDialogMessage.textContent = (data.warning ? data.warning + '；' : '') + '共 ' + commands.length + ' 条命令；请人工确认后运行。';
  commandDialogBody.textContent = commands.map(c => c.full_command).join('\n\n');
  commandDialogCopy.onclick = () => navigator.clipboard.writeText(commandDialogBody.textContent);
  if(data.tool_link && (tool === 'nuclei' || tool === 'nmap')){
    commandDialogTool.href = data.tool_link;
    commandDialogTool.hidden = false;
  } else {
    commandDialogTool.hidden = true;
  }
}

document.querySelectorAll('.command-btn').forEach(btn => btn.addEventListener('click', async () => {
  const card = btn.closest('.candidate-card');
  const key = card.dataset.key;
  const tool = btn.dataset.tool;
  const verificationID = card.querySelector('.verify-btn')?.dataset.verificationId || '';
  showCommandDialog();
  try {
    const data = await fetchCommand(key, tool, 'all', verificationID);
    renderCommandResult(tool, data);
  } catch(err){
    commandDialogMessage.textContent = err.message || String(err);
  }
}));

document.querySelectorAll('.command-asset-btn').forEach(btn => btn.addEventListener('click', async () => {
  const card = btn.closest('.candidate-card');
  const key = card.dataset.key;
  const tool = btn.dataset.tool;
  const asset = btn.dataset.asset;
  const verificationID = card.querySelector('.verify-btn')?.dataset.verificationId || '';
  showCommandDialog();
  try {
    const data = await fetchCommand(key, tool, asset, verificationID);
    renderCommandResult(tool, data);
  } catch(err){
    commandDialogMessage.textContent = err.message || String(err);
  }
}));

// ---------- Copy whole card ----------
document.querySelectorAll('.copy-card-btn').forEach(btn => btn.addEventListener('click', async () => {
  const card = btn.closest('.candidate-card');
  const key = card.dataset.key;
  const c = candidateByKey(key);
  if(!c) return;
  const lines = [
    '漏洞名\n' + c.Title,
    '漏洞简介\n' + (c.Description || ''),
    '漏洞资产\n' + c.Assets.map(a => netHostPort(a.IP, a.Port)).join('\n'),
    '修复建议\n' + (c.Remediation || ''),
  ];
  const text = lines.join('\n\n');
  await navigator.clipboard.writeText(text.trimEnd());
  const original = btn.textContent;
  btn.textContent = '已复制';
  setTimeout(() => btn.textContent = original, 1200);
}));

// ---------- Verification dialog ----------
const verifyDialog = document.getElementById('verify-dialog');
const verifyForm = document.getElementById('verify-form');
const verifyKey = document.getElementById('verify-key');
const verifyZoneId = document.getElementById('verify-zone-id');
const verifyId = document.getElementById('verify-id');
const verifyTitle = document.getElementById('verify-title');
const verifySeverity = document.getElementById('verify-severity');
const verifyOutcome = document.getElementById('verify-outcome');
const verifyIncluded = document.getElementById('verify-included');
const verifyDescription = document.getElementById('verify-description');
const verifyRemediation = document.getElementById('verify-remediation');
const verifyAssets = document.getElementById('verify-assets');
const verifyEvidenceList = document.getElementById('verify-evidence-list');
const verifyFileInput = document.getElementById('verify-evidence-file');
const verifyPasteHint = document.getElementById('verify-paste-hint');

let currentVerification = null;

function resetVerifyDialog(){
  verifyForm.reset();
  verifyAssets.innerHTML = '';
  verifyEvidenceList.innerHTML = '';
  currentVerification = null;
}

function buildVerificationPayload(c){
  const assets = (c.Assets || []).map((a, i) => ({
    ip: a.IP, port: a.Port, protocol: a.Protocol,
    asset_name: netHostPort(a.IP, a.Port), position: i,
  }));
  const sources = (c.Sources || []).map(s => ({
    run_id: s.RunID, source: s.Source, finding_id: s.FindingID,
    ip: s.IP, port: s.Port, protocol: s.Protocol,
  }));
  return {
    zone_id: verifyZoneId.value,
    vulnerability_key: c.GroupKey,
    outcome: verifyOutcome.value,
    title: verifyTitle.value.trim(),
    severity: verifySeverity.value,
    description: verifyDescription.value.trim(),
    remediation: verifyRemediation.value.trim(),
    included: verifyIncluded.checked,
    position: 0,
    assets,
    sources,
  };
}

async function openVerifyDialog(key){
  const c = candidateByKey(key);
  if(!c) return;
  resetVerifyDialog();
  verifyKey.value = c.GroupKey;
  verifyZoneId.value = c.ZoneID;
  verifyTitle.value = c.Title;
  verifySeverity.value = c.Severity || 'high';
  verifyDescription.value = c.Description || '';
  verifyRemediation.value = c.Remediation || '';
  verifyOutcome.value = 'confirmed';
  verifyIncluded.checked = false;

  verifyAssets.innerHTML = (c.Assets || []).map((a, i) => {
    return '<li><code>' + netHostPort(a.IP, a.Port) + '</code>' + (a.Target ? ' <a href="' + a.Target + '" target="_blank">' + a.Target + '</a>' : '') + '</li>';
  }).join('');

  const btn = document.querySelector('.verify-btn[data-key="' + key + '"]');
  const vid = btn?.dataset.verificationId;
  if(vid){
    verifyId.value = vid;
    try {
      const res = await fetch('/projects/' + projectID + '/verifications/' + vid);
      if(res.ok){
        const v = await res.json();
        currentVerification = v;
        verifyTitle.value = v.Verification.Title;
        verifySeverity.value = v.Verification.Severity;
        verifyOutcome.value = v.Verification.Outcome;
        verifyIncluded.checked = v.Verification.Included;
        verifyDescription.value = v.Verification.Description || '';
        verifyRemediation.value = v.Verification.Remediation || '';
        renderEvidenceList(v.Evidence || []);
      }
    } catch(e) {
      // ignore
    }
  }
  verifyDialog.showModal();
}

function renderEvidenceList(list){
  verifyEvidenceList.innerHTML = list.map(e => {
    return '<li class="evidence-item">' +
      '<img src="/projects/' + projectID + '/verifications/' + verifyId.value + '/evidence/' + e.ID + '" alt="" loading="lazy">' +
      '<span>' + (e.Caption || '无说明') + '</span>' +
      '<button class="button button-small" type="button" data-evidence-id="' + e.ID + '">删除</button>' +
      '</li>';
  }).join('');
  verifyEvidenceList.querySelectorAll('button[data-evidence-id]').forEach(btn => btn.addEventListener('click', async () => {
    if(!confirm('确定删除这张截图？')) return;
    const eid = btn.dataset.evidenceId;
    const res = await fetch('/projects/' + projectID + '/verifications/' + verifyId.value + '/evidence/' + eid, {
      method: 'POST',
      headers: {'Content-Type': 'application/x-www-form-urlencoded'},
      body: new URLSearchParams({_method: 'delete'}),
    });
    if(res.ok){
      btn.closest('li').remove();
    } else {
      alert('删除失败');
    }
  }));
}

async function uploadEvidenceFile(file, caption){
  if(!verifyId.value) return;
  const form = new FormData();
  form.append('file', file);
  form.append('caption', caption);
  const res = await fetch('/projects/' + projectID + '/verifications/' + verifyId.value + '/evidence', {
    method: 'POST',
    body: form,
  });
  if(!res.ok) throw new Error((await res.text()).trim() || '上传失败');
  return res.json();
}

async function handleFile(file){
  if(!file) return;
  const caption = prompt('截图说明（覆盖范围）：') || '';
  await uploadEvidenceFile(file, caption);
  if(verifyId.value){
    const res = await fetch('/projects/' + projectID + '/verifications/' + verifyId.value);
    if(res.ok){
      const v = await res.json();
      renderEvidenceList(v.Evidence || []);
    }
  }
}

verifyFileInput?.addEventListener('change', () => {
  handleFile(verifyFileInput.files[0]);
  verifyFileInput.value = '';
});

verifyPasteHint?.addEventListener('paste', async (e) => {
  e.preventDefault();
  const items = e.clipboardData?.items;
  if(!items) return;
  for(const item of items){
    if(item.type.startsWith('image/')){
      const file = item.getAsFile();
      if(file) await handleFile(file);
    }
  }
});

verifyForm?.addEventListener('submit', async (e) => {
  e.preventDefault();
  const c = candidateByKey(verifyKey.value);
  if(!c) return;
  const payload = buildVerificationPayload(c);
  const vid = verifyId.value;
  try {
    let res;
    if(vid){
      res = await fetch('/projects/' + projectID + '/verifications/' + vid, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
          zone_id: payload.zone_id,
          vulnerability_key: payload.vulnerability_key,
          outcome: payload.outcome,
          title: payload.title,
          severity: payload.severity,
          description: payload.description,
          remediation: payload.remediation,
          notes: '',
          included: payload.included,
          position: payload.position,
        }),
      });
    } else {
      res = await fetch('/projects/' + projectID + '/verifications', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(payload),
      });
      if(res.ok){
        const created = await res.json();
        verifyId.value = created.ID;
      }
    }
    if(!res.ok) throw new Error((await res.text()).trim() || '保存失败');
    window.location.reload();
  } catch(err){
    alert(err.message || String(err));
  }
});

document.querySelectorAll('.verify-btn').forEach(btn => btn.addEventListener('click', () => openVerifyDialog(btn.dataset.key)));

document.getElementById('verify-cancel')?.addEventListener('click', () => verifyDialog.close());
