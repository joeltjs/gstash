const $ = (id) => document.getElementById(id);
let stashes = [], branch = "-", sel = null;

const esc = (s) => String(s).replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));

async function load() {
  try {
    const r = await fetch("/api/stashes");
    if (!r.ok) {
      const err = await r.json().catch(() => ({ error: "Failed to load stashes" }));
      toast(err.error || "Failed to load stashes", true);
      return;
    }
    const d = await r.json();
    stashes = d.stashes || [];
    branch = d.branch || "-";
    $("branch").innerHTML = "branch: <b>" + esc(branch) + "</b>";
    renderTable();
  } catch (e) {
    toast("Connection error while fetching stashes", true);
  }
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
    let srcBadge = "";
    if (s.source === "inferred") srcBadge = ' <span class="badge-inferred" title="Inferred from parent commit">~</span>';
    else if (s.source === "unknown") srcBadge = ' <span class="badge-unknown" title="Unknown branch origin">?</span>';

    tr.innerHTML =
      '<td class="ref">' + esc(s.ref) + "</td>" +
      '<td class="branch">' + esc(s.branch || "-") + srcBadge + "</td>" +
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
  $("diff-container").classList.add("hidden");
  $("pick").classList.remove("hidden");
}

async function loadDiff() {
  $("pick").classList.add("hidden");
  $("actionsbar").classList.remove("hidden");

  const s = stashes.find(x => x.ref === sel);
  $("meta").innerHTML = s
    ? "<b>" + esc(s.ref) + "</b> &middot; " + esc(s.age || "") + " &middot; " + esc(s.message)
    : "<b>" + esc(sel) + "</b>";

  try {
    const r = await fetch("/api/diff?ref=" + encodeURIComponent(sel));
    if (!r.ok) {
      const err = await r.json().catch(() => ({ error: "Failed to load diff" }));
      $("diff").textContent = err.error || "Failed to load diff";
      $("diff-container").classList.remove("hidden");
      return;
    }
    const d = await r.json();
    const rawDiff = String(d.diff || "");
    if (!rawDiff.trim()) {
      $("diff").innerHTML = '<div class="diff-line l-plain">No changes recorded in this stash.</div>';
      $("diff-container").classList.remove("hidden");
      return;
    }

    const lines = rawDiff.split("\n");
    let html = "";
    for (const l of lines) {
      const e = esc(l);
      if (l.startsWith("+++") || l.startsWith("---")) {
        html += '<div class="diff-line l-meta">' + e + "</div>";
      } else if (l.startsWith("+")) {
        html += '<div class="diff-line l-add">' + e + "</div>";
      } else if (l.startsWith("-")) {
        html += '<div class="diff-line l-del">' + e + "</div>";
      } else if (l.startsWith("@@")) {
        html += '<div class="diff-line l-hunk">' + e + "</div>";
      } else if (l.startsWith("diff --git") || l.startsWith("index ")) {
        html += '<div class="diff-line l-meta">' + e + "</div>";
      } else if (l.includes("|") && (l.includes("+") || l.includes("-"))) {
        html += '<div class="diff-line l-stat">' + e + "</div>";
      } else {
        html += '<div class="diff-line l-plain">' + e + "</div>";
      }
    }
    $("diff").innerHTML = html;
    $("diff-container").classList.remove("hidden");
  } catch (e) {
    $("diff").textContent = "Network error loading diff";
    $("diff-container").classList.remove("hidden");
  }
}

const MUTATE_HEADERS = { "X-Requested-With": "gstash" };

async function accept() {
  if (!sel) return;
  const ref = sel;
  try {
    const r = await fetch("/api/pop?ref=" + encodeURIComponent(ref), { method: "POST", headers: MUTATE_HEADERS });
    const d = await r.json();
    if (!r.ok) { toast(d.error || "Failed to pop stash", true); return; }
    toast("Pop successful: " + ref + " applied and removed from stash list", false);
    sel = null;
    await load();
  } catch (e) {
    toast("Error executing pop", true);
  }
}

async function reject() {
  if (!sel) return;
  const ok = confirm(
    "Are you sure you want to drop " + sel + "?\n\n" +
    "This will PERMANENTLY delete the stash without applying changes."
  );
  if (!ok) return;
  try {
    const r = await fetch("/api/drop?ref=" + encodeURIComponent(sel), { method: "POST", headers: MUTATE_HEADERS });
    const d = await r.json();
    if (!r.ok) { toast(d.error || "Failed to drop stash", true); return; }
    toast("Drop successful: " + sel + " permanently removed", false);
    sel = null;
    await load();
  } catch (e) {
    toast("Error executing drop", true);
  }
}

async function makeBranch() {
  if (!sel) return;
  const def = "stash-" + sel.replace(/[^a-z0-9-]/gi, "-");
  const name = prompt("Enter new branch name for " + sel + ":", def);
  if (!name || !name.trim()) return;
  try {
    const r = await fetch("/api/branch?ref=" + encodeURIComponent(sel) + "&name=" + encodeURIComponent(name.trim()), { method: "POST", headers: MUTATE_HEADERS });
    const d = await r.json();
    if (!r.ok) { toast(d.error || "Failed to create branch", true); return; }
    toast("Branch '" + name.trim() + "' created from " + sel + " and checked out", false);
    sel = null;
    await load();
  } catch (e) {
    toast("Error creating branch", true);
  }
}

function toast(msg, err) {
  const el = $("toast");
  el.textContent = msg;
  el.className = err ? "err" : "ok";
  setTimeout(() => el.classList.add("hidden"), 3500);
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
    const flex = $("tablewrap").style.flex;
    const pct = parseFloat(flex.split(" ")[2]);
    if (!isNaN(pct)) localStorage.setItem(STORE_KEY, String(pct));
  });
}

// Keyboard navigation in web dashboard
window.addEventListener("keydown", (e) => {
  if (e.target && (e.target.tagName === "INPUT" || e.target.tagName === "TEXTAREA")) return;
  const list = filtered();
  if (list.length === 0) return;
  const idx = list.findIndex(s => s.ref === sel);

  if (e.key === "j" || e.key === "ArrowDown") {
    e.preventDefault();
    if (idx < list.length - 1) {
      sel = list[idx + 1].ref;
      renderTable();
      loadDiff();
    }
  } else if (e.key === "k" || e.key === "ArrowUp") {
    e.preventDefault();
    if (idx > 0) {
      sel = list[idx - 1].ref;
      renderTable();
      loadDiff();
    }
  } else if (e.key === "a") {
    e.preventDefault();
    accept();
  } else if (e.key === "d" || e.key === "r") {
    e.preventDefault();
    reject();
  } else if (e.key === "b") {
    e.preventDefault();
    makeBranch();
  } else if (e.key === "Tab") {
    e.preventDefault();
    $("filter").checked = !$("filter").checked;
    $("filter").dispatchEvent(new Event("change"));
  }
});

$("filter").onchange = () => { localStorage.setItem("gstash.filter.only", $("filter").checked ? "1" : "0"); renderTable(); };
$("btnAccept").onclick = accept;
$("btnReject").onclick = reject;
$("btnBranch").onclick = makeBranch;

if (localStorage.getItem("gstash.filter.only") === "1") {
  $("filter").checked = true;
}
initDrag();
load();
