const projectID = window.location.pathname.split('/')[2];
const candidates = JSON.parse(document.getElementById('workbench-data')?.textContent || '[]');
const negativeGroups = JSON.parse(document.getElementById('negative-groups-data')?.textContent || '[]');

function candidateByKey(key){
  return candidates.find(c => c.GroupKey === key);
}

function negativeGroupByKey(key){
  return negativeGroups.find(g => g.Key === key);
}

function netHostPort(ip, port){
  return ip + ':' + port;
}

// ---------- Queue tabs ----------
document.querySelectorAll('.queue-tab').forEach(tab => {
  tab.addEventListener('click', () => {
    document.querySelectorAll('.queue-tab').forEach(t => {
      t.classList.remove('active');
      t.setAttribute('aria-selected', 'false');
    });
    tab.classList.add('active');
    tab.setAttribute('aria-selected', 'true');

    const panelId = tab.getAttribute('aria-controls');
    document.querySelectorAll('[role="tabpanel"]').forEach(p => {
      p.hidden = p.id !== panelId;
    });
  });
});

// ---------- Filtering (positive) ----------
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
  document.querySelectorAll('.candidate-card:not(.negative-card):not(.incomplete-card)').forEach(card => {
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

// ---------- Verification dialog (positive) ----------
const verifyDialog = document.getElementById('verify-dialog');
const verifyForm = document.getElementById('verify-form');
const verifyKey = document.getElementById('verify-key');
const verifyZoneId = document.getElementById('verify-zone-id');
const verifyId = document.getElementById('verify-id');
const verifyTitle = document.getElementById('verify-title');
const verifySeverity = document.getElementById('verify-severity');
const verifyOutcome = document.getElementById('verify-outcome');
const verifyDescription = document.getElementById('verify-description');
const verifyRemediation = document.getElementById('verify-remediation');
const verifyAssets = document.getElementById('verify-assets');
const verifyEvidenceList = document.getElementById('verify-evidence-list');
const verifyFileInput = document.getElementById('verify-evidence-file');

let currentVerification = null;
let verifyPendingFiles = [];
let verifyVerificationID = null;

function resetVerifyDialog(){
  verifyForm.reset();
  verifyAssets.innerHTML = '';
  verifyEvidenceList.innerHTML = '';
  verifyPendingFiles = [];
  verifyVerificationID = null;
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
    included: verifyOutcome.value === 'confirmed' || verifyOutcome.value === 'not_observed',
    position: 0,
    assets,
    sources,
  };
}

function stageVerifyFile(file){
  if(!file) return;
  verifyPendingFiles.push({file, caption: ''});
  renderVerifyEvidenceList();
}

function renderVerifyEvidenceList(){
  const items = [];
  verifyPendingFiles.forEach((f, i) => {
    items.push({type: 'pending', idx: i, url: URL.createObjectURL(f.file), caption: f.caption || '待上传'});
  });
  const vid = verifyVerificationID || verifyId.value;
  if(vid && currentVerification){
    (currentVerification.Evidence || []).forEach(e => {
      items.push({type: 'server', id: e.ID, url: '/projects/' + projectID + '/verifications/' + vid + '/evidence/' + e.ID, caption: e.Caption || '无说明'});
    });
  }
  verifyEvidenceList.innerHTML = items.map(item => {
    if(item.type === 'pending'){
      return '<li class="evidence-item">' +
        '<img src="' + item.url + '" alt="" style="max-height:80px;object-fit:cover">' +
        '<span>' + item.caption + '</span>' +
        '<button class="button button-small" type="button" data-verify-pending-idx="' + item.idx + '">删除</button>' +
        '</li>';
    }
    return '<li class="evidence-item">' +
      '<img src="' + item.url + '" alt="" loading="lazy">' +
      '<span>' + item.caption + '</span>' +
      '<button class="button button-small" type="button" data-evidence-id="' + item.id + '">删除</button>' +
      '</li>';
  }).join('');
  verifyEvidenceList.querySelectorAll('button[data-verify-pending-idx]').forEach(btn => {
    btn.addEventListener('click', () => {
      const idx = parseInt(btn.dataset.verifyPendingIdx, 10);
      verifyPendingFiles.splice(idx, 1);
      renderVerifyEvidenceList();
    });
  });
  verifyEvidenceList.querySelectorAll('button[data-evidence-id]').forEach(btn => btn.addEventListener('click', async () => {
    if(!confirm('确定删除这张截图？')) return;
    const eid = btn.dataset.evidenceId;
    const delVid = verifyVerificationID || verifyId.value;
    const res = await fetch('/projects/' + projectID + '/verifications/' + delVid + '/evidence/' + eid, {
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

  verifyAssets.innerHTML = (c.Assets || []).map((a) => {
    return '<li><code>' + netHostPort(a.IP, a.Port) + '</code>' + (a.Target ? ' <a href="' + a.Target + '" target="_blank">' + a.Target + '</a>' : '') + '</li>';
  }).join('');

  const btn = document.querySelector('.verify-btn[data-key="' + key + '"]');
  const vid = btn?.dataset.verificationId;
  if(vid){
    verifyId.value = vid;
    verifyVerificationID = vid;
    try {
      const res = await fetch('/projects/' + projectID + '/verifications/' + vid);
      if(res.ok){
        const v = await res.json();
        currentVerification = v;
        verifyTitle.value = v.Verification.Title;
        verifySeverity.value = v.Verification.Severity;
        verifyOutcome.value = v.Verification.Outcome;
        verifyDescription.value = v.Verification.Description || '';
        verifyRemediation.value = v.Verification.Remediation || '';
        renderVerifyEvidenceList();
      }
    } catch(e) {
      // ignore
    }
  }
  verifyDialog.showModal();
}

async function uploadEvidenceFile(file, caption, verificationIDOverride){
  const vid = verificationIDOverride || verifyId.value;
  if(!vid) return;
  const form = new FormData();
  form.append('file', file);
  form.append('caption', caption);
  const res = await fetch('/projects/' + projectID + '/verifications/' + vid + '/evidence', {
    method: 'POST',
    body: form,
  });
  if(!res.ok) throw new Error((await res.text()).trim() || '上传失败');
  return res.json();
}

verifyFileInput?.addEventListener('change', () => {
  stageVerifyFile(verifyFileInput.files[0]);
  verifyFileInput.value = '';
});

verifyDialog?.addEventListener('paste', async (e) => {
  const files = imagesFromClipboardData(e.clipboardData?.items);
  if(files.length === 0) return;
  e.preventDefault();
  files.forEach(stageVerifyFile);
});

verifyDialog?.addEventListener('dragover', e => e.preventDefault());
verifyDialog?.addEventListener('drop', e => {
  e.preventDefault();
  const files = e.dataTransfer?.files;
  if(!files) return;
  for(const file of files){
    if(file.type.startsWith('image/')) stageVerifyFile(file);
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
        verifyVerificationID = created.ID;
      }
    }
    if(!res.ok) throw new Error((await res.text()).trim() || '保存失败');
    const uploadVid = verifyVerificationID || verifyId.value;
    for(const f of verifyPendingFiles){
      await uploadEvidenceFile(f.file, f.caption, uploadVid);
    }
    window.location.reload();
  } catch(err){
    alert(err.message || String(err));
  }
});

document.querySelectorAll('.command-copy-btn').forEach(btn => btn.addEventListener('click', async () => {
  const text = btn.dataset.command || '';
  await navigator.clipboard.writeText(text);
  const original = btn.textContent;
  btn.textContent = '已复制';
  setTimeout(() => btn.textContent = original, 1200);
}));

document.querySelectorAll('.verify-btn').forEach(btn => btn.addEventListener('click', () => openVerifyDialog(btn.dataset.key)));

document.getElementById('verify-cancel')?.addEventListener('click', () => verifyDialog.close());

// ---------- Negative multi-select ----------
const negativeSubmitBtn = document.getElementById('negative-submit-btn');
const negativeSelectedCount = document.getElementById('negative-selected-count');

function updateNegativeSelection(){
  const checked = document.querySelectorAll('.negative-select:checked');
  const n = checked.length;
  negativeSelectedCount.textContent = n === 1 ? '已选 1 组' : '请选择 1 组';
  if(negativeSubmitBtn) negativeSubmitBtn.disabled = n === 0;
}

document.querySelectorAll('.negative-select').forEach(cb => {
  cb.addEventListener('change', updateNegativeSelection);
});

// ---------- Negative dialog ----------
const negativeDialog = document.getElementById('negative-dialog');
const negativeForm = document.getElementById('negative-form');
const negZoneId = document.getElementById('neg-zone-id');
const negTitle = document.getElementById('neg-title');
const negSeverity = document.getElementById('neg-severity');
const negDescription = document.getElementById('neg-description');
const negAssetsList = document.getElementById('neg-assets-list');
const negEvidenceList = document.getElementById('neg-evidence-list');
const negEvidenceFile = document.getElementById('neg-evidence-file');

// Pending evidence for negative dialog before verification is created
let negPendingFiles = [];  // [{file, caption}]
let negVerificationID = null;
let selectedNegativeGroup = null;

function resetNegDialog(){
  negativeForm.reset();
  negAssetsList.innerHTML = '';
  negEvidenceList.innerHTML = '';
  negPendingFiles = [];
  negVerificationID = null;
  selectedNegativeGroup = null;
}

function renderNegEvidenceList(){
  negEvidenceList.innerHTML = negPendingFiles.map((f, i) => {
    const url = URL.createObjectURL(f.file);
    return '<li class="evidence-item">' +
      '<img src="' + url + '" alt="" style="max-height:80px;object-fit:cover">' +
      '<span>' + (f.caption || '无说明') + '</span>' +
      '<button class="button button-small" type="button" data-neg-idx="' + i + '">删除</button>' +
      '</li>';
  }).join('');
  negEvidenceList.querySelectorAll('button[data-neg-idx]').forEach(btn => {
    btn.addEventListener('click', () => {
      const idx = parseInt(btn.dataset.negIdx, 10);
      negPendingFiles.splice(idx, 1);
      renderNegEvidenceList();
    });
  });
}

function stageNegFile(file){
  if(!file) return;
  negPendingFiles.push({file, caption: ''});
  renderNegEvidenceList();
}

negEvidenceFile?.addEventListener('change', () => {
  stageNegFile(negEvidenceFile.files[0]);
  negEvidenceFile.value = '';
});

negativeDialog?.addEventListener('paste', async (e) => {
  const files = imagesFromClipboardData(e.clipboardData?.items);
  if(files.length === 0) return;
  e.preventDefault();
  files.forEach(stageNegFile);
});

negativeDialog?.addEventListener('dragover', e => e.preventDefault());
negativeDialog?.addEventListener('drop', e => {
  e.preventDefault();
  const files = e.dataTransfer?.files;
  if(!files) return;
  for(const file of files){
    if(file.type.startsWith('image/')) stageNegFile(file);
  }
});

negativeSubmitBtn?.addEventListener('click', () => {
  const checked = [...document.querySelectorAll('.negative-select:checked')];
  if(checked.length === 0) return;

  resetNegDialog();

  const card = checked[0].closest('.candidate-card');
  const group = negativeGroupByKey(card?.dataset.key);
  if(!group) return;
  selectedNegativeGroup = group;

  negZoneId.value = group.ZoneID || card?.dataset.zone || '';
  negTitle.value = group.Title || '未发现 ' + (group.Service || '服务');

  // Populate assets list from the group
  negAssetsList.innerHTML = (group.Assets || []).map(a => {
    return '<li><code>' + netHostPort(a.IP, a.Port) + '</code></li>';
  }).join('');

  negativeDialog.showModal();
});

negativeForm?.addEventListener('submit', async (e) => {
  e.preventDefault();

  if(negPendingFiles.length === 0){
    alert('请至少上传一张截图作为本次验证未发现的证据。');
    return;
  }

  if(!selectedNegativeGroup || !selectedNegativeGroup.Assets || selectedNegativeGroup.Assets.length === 0){
    alert('没有已选指纹组。');
    return;
  }

  const assets = selectedNegativeGroup.Assets.map((a, i) => ({
    ip: a.IP,
    port: a.Port,
    protocol: a.Protocol || 'tcp',
    asset_name: netHostPort(a.IP, a.Port),
    position: i,
  }));

  // Derive a stable vulnerability key from the title
  const vulnKey = 'neg:' + negTitle.value.trim().toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '');

  const payload = {
    zone_id: negZoneId.value,
    vulnerability_key: vulnKey,
    outcome: 'not_observed',
    title: negTitle.value.trim(),
    severity: negSeverity.value,
    description: negDescription.value.trim(),
    remediation: '',
    notes: '',
    included: false,
    position: 0,
    assets,
    sources: [],
  };

  try {
    // Create the verification (not yet included — no evidence yet)
    const createRes = await fetch('/projects/' + projectID + '/verifications', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(payload),
    });
    if(!createRes.ok) throw new Error((await createRes.text()).trim() || '创建失败');
    const created = await createRes.json();
    negVerificationID = created.ID;

    // Upload all pending evidence files
    for(const f of negPendingFiles){
      await uploadEvidenceFile(f.file, f.caption, negVerificationID);
    }

    window.location.reload();
  } catch(err){
    alert(err.message || String(err));
  }
});

document.getElementById('neg-cancel')?.addEventListener('click', () => negativeDialog.close());
