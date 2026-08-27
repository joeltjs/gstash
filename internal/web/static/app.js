const $ = (id) => document.getElementById(id);
let stashes = [], branch = "-", sel = null;

const esc = (s) => String(s).replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));

async function load() {
  const r = await fetch("/api/stashes");
  if (!r.ok) { toast((await r.json()).error || "gagal memuat stash", true); return; }
  const d = await r.json();
  stashes = d.stashes || [];
  branch = d.branch || "-";
  $("branch").innerHTML = "branch: <b>" + esc(branch) + "</b>";
  renderTable();
}

function filtered() {
  const only = $("filter").checked;
  return stashes.filter(s => !only || s.branch === branch || s.source === "unknown");
}

function renderTable() {
  const rows = $("rows");
  rows.innerHTML = "";
  const list = filtered();
  $("empty").classList.toggle("hidden", list.length > 0);

  if (!list.some(s => s.ref === sel)) {
    sel = list.length ? list[0].ref : null;
  }

  for (const s of list) {
    const tr = document.createElement("tr");
    tr.className = s.ref === sel ? "sel" : "";
    tr.innerHTML =
      '<td class="ref">' + esc(s.ref) + "</td>" +
      '<td class="branch">' + esc(s.branch || "-") + "</td>" +
      '<td class="age">' + esc(s.age || "") + "</td>" +
      '<td class="msg">' + esc(s.message) + "</td>";
    tr.onclick = () => { sel = s.ref; renderTable(); loadDiff(); };
    rows.appendChild(tr);
  }

  if (sel) loadDiff();
  else showPlaceholder();
}

function showPlaceholder() {
  $("actionsbar").classList.add("hidden");
  $("diff").classList.add("hidden");
  $("pick").classList.remove("hidden");
}

async function loadDiff() {
  $("pick").classList.add("hidden");
  $("actionsbar").classList.remove("hidden");

  const s = stashes.find(x => x.ref === sel);
  $("meta").textContent = s ? s.ref + " · " + (s.age || "") + " · " + s.message : sel;

  const r = await fetch("/api/diff?ref=" + encodeURIComponent(sel));
  if (!r.ok) {
    $("diff").textContent = (await r.json()).error;
    $("diff").classList.remove("hidden");
    return;
  }
  const d = await r.json();
  const lines = String(d.diff).split("\n");
  let html = "";
  for (const l of lines) {
    const e = esc(l);
    if (l.startsWith("@@")) html += '<span class="l-hunk">' + e + "</span>\n";
    else if (l.startsWith("+") && !l.startsWith("+++")) html += '<span class="l-add">' + e + "</span>\n";
    else if (l.startsWith("-") && !l.startsWith("---")) html += '<span class="l-del">' + e + "</span>\n";
    else if (l.startsWith("diff ") || l.startsWith("index ")) html += '<span class="l-meta">' + e + "</span>\n";
    else html += e + "\n";
  }
  $("diff").innerHTML = html;
  $("diff").classList.remove("hidden");
}

const MUTATE_HEADERS = { "X-Requested-With": "gstash" };

/* Accept = git stash pop: apply the changes, then the stash is removed from the list */
async function accept() {
  if (!sel) return;
  const ref = sel;
  const r = await fetch("/api/pop?ref=" + encodeURIComponent(ref), { method: "POST", headers: MUTATE_HEADERS });
  const d = await r.json();
  if (!r.ok) { toast(d.error, true); return; }
  toast("Accept: " + ref + " applied to the working tree, stash removed", false);
  sel = null;
  await load();
}

/* Reject = git stash drop: the stash is deleted permanently, changes are not applied */
async function reject() {
  if (!sel) return;
  const ok = confirm(
    "Reject will PERMANENTLY delete " + sel + " from the git stash list.\n" +
    "The changes inside will NOT be applied and cannot be recovered through gstash.\n\nProceed?"
  );
  if (!ok) return;
  const r = await fetch("/api/drop?ref=" + encodeURIComponent(sel), { method: "POST", headers: MUTATE_HEADERS });
  const d = await r.json();
  if (!r.ok) { toast(d.error, true); return; }
  toast("Reject: " + sel + " permanently deleted from the stash list", false);
  sel = null;
  await load();
}

async function makeBranch() {
  if (!sel) return;
  const def = "stash-" + sel.replace(/[^a-z0-9-]/gi, "-");
  const name = prompt("New branch name for " + sel + ":", def);
  if (!name) return;
  const r = await fetch("/api/branch?ref=" + encodeURIComponent(sel) + "&name=" + encodeURIComponent(name), { method: "POST", headers: MUTATE_HEADERS });
  const d = await r.json();
  if (!r.ok) { toast(d.error, true); return; }
  toast("Branch '" + name + "' created from " + sel + ", HEAD moved there", false);
  sel = null;
  await load();
}

function toast(msg, err) {
  const el = $("toast");
  el.textContent = msg;
  el.className = err ? "err" : "ok";
  setTimeout(() => el.classList.add("hidden"), 3200);
}

/* ---------- panel split drag ---------- */

const STORE_KEY = "gstash.split.pct";

function applySplit(pct) {
  const clamped = Math.min(80, Math.max(15, pct));
  $("tablewrap").style.flex = "0 0 " + clamped + "%";
}

function initDrag() {
  const saved = parseFloat(localStorage.getItem(STORE_KEY));
  if (!isNaN(saved)) applySplit(saved);

  let dragging = false;
  $("divider").addEventListener("mousedown", (e) => {
    dragging = true;
    $("divider").classList.add("dragging");
    document.body.style.userSelect = "none";
    e.preventDefault();
  });
  window.addEventListener("mousemove", (e) => {
    if (!dragging) return;
    const pct = (e.clientY / window.innerHeight) * 100;
    applySplit(pct);
  });
  window.addEventListener("mouseup", () => {
    if (!dragging) return;
    dragging = false;
    $("divider").classList.remove("dragging");
    document.body.style.userSelect = "";
    const flex = $("tablewrap").style.flex; // "0 0 NN%"
    const pct = parseFloat(flex.split(" ")[2]);
    if (!isNaN(pct)) localStorage.setItem(STORE_KEY, String(pct));
  });
}

$("filter").onchange = () => { localStorage.setItem("gstash.filter.only", $("filter").checked ? "1" : "0"); renderTable(); };
$("btnAccept").onclick = accept;
$("btnReject").onclick = reject;
$("btnBranch").onclick = makeBranch;

if (localStorage.getItem("gstash.filter.only") === "1") {
  $("filter").checked = true;
}
initDrag();
load();
