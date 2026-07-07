async function refreshEvents(){
  if(!window.anchorRunID) return;
  const res = await fetch('/api/runs/' + window.anchorRunID + '/events');
  if(!res.ok) return;
  const events = await res.json();
  const box = document.getElementById('events');
  if(box){ box.textContent = events.map(e => `${e.time} [${e.stage}] ${e.message}`).join('\n'); }
}
setInterval(refreshEvents, 1000);
refreshEvents();
