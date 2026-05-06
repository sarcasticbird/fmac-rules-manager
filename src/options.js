const CONTAINER_COLORS = {
  blue: "#37adff", turquoise: "#00c79a", green: "#51cd00",
  yellow: "#ffcb00", orange: "#ff9f00", red: "#ff613d",
  pink: "#ff4bda", purple: "#af51f5", toolbar: "#7c7c7d",
};

let allRules = [];
let containers = [];
let containersByID = {};
let selectedSites = new Set();
let sortField = "site";
let sortAsc = true;

function escapeHTML(s) {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

async function sendCommand(cmd) {
  return browser.runtime.sendMessage(cmd);
}

// MAC's assignManager.js getSiteStoreKey() builds keys as `hostname + port` (no colon).
function cleanHostname(input) {
  let s = input.trim().toLowerCase();
  try {
    const url = new URL(s.includes("://") ? s : `https://${s}`);
    const port = url.port;
    s = url.hostname;
    if (port && port !== "80" && port !== "443") {
      s += port;
    }
  } catch {
    s = s.replace(/[^a-z0-9.\-]/g, "");
  }
  s = s.replace(/^\.+|\.+$/g, "");
  return s;
}

function isValidHostname(s) {
  if (!s) return false;
  // MAC stores non-standard ports appended directly: localhost5173
  const match = s.match(/^([a-z0-9.-]+?)(\d+)$/);
  const host = match ? match[1] : s;
  if (host.length > 253) return false;
  const labels = host.split(".");
  return labels.every((l) => l.length > 0 && l.length <= 63 && /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/.test(l));
}

function showBanner(message, type) {
  const banner = document.getElementById("banner");
  banner.textContent = message;
  banner.className = `banner ${type}`;
  banner.classList.remove("hidden");
  if (type === "success") {
    setTimeout(() => banner.classList.add("hidden"), 3000);
  }
}

function getActiveContainerId() {
  return document.getElementById("active-container").value;
}

function populateContainerDropdowns() {
  const activeSel = document.getElementById("active-container");
  const prev = activeSel.value;
  activeSel.textContent = "";
  const allOpt = document.createElement("option");
  allOpt.value = "";
  allOpt.textContent = "All containers";
  activeSel.appendChild(allOpt);
  containers.forEach((c) => {
    const opt = document.createElement("option");
    opt.value = c.userContextId;
    opt.textContent = c.name;
    activeSel.appendChild(opt);
  });
  activeSel.value = prev;

  const bulkSel = document.getElementById("bulk-reassign-container");
  bulkSel.textContent = "";
  containers.forEach((c) => {
    const opt = document.createElement("option");
    opt.value = c.userContextId;
    opt.textContent = c.name;
    bulkSel.appendChild(opt);
  });
}

function updateContainerView() {
  const ctxId = getActiveContainerId();
  const input = document.getElementById("input-site");
  const addControls = document.querySelectorAll(".add-controls");
  const hint = document.getElementById("input-hint");

  if (ctxId) {
    addControls.forEach((el) => el.classList.remove("hidden"));
    hint.classList.remove("hidden");
    input.placeholder = "Search or add sites (comma-separated)...";
  } else {
    addControls.forEach((el) => el.classList.add("hidden"));
    hint.classList.add("hidden");
    input.placeholder = "Search sites (comma-separated)...";
  }

  const count = ctxId
    ? allRules.filter((r) => r.userContextId === parseInt(ctxId)).length
    : allRules.length;
  document.getElementById("rule-count").textContent = `${count} rule${count !== 1 ? "s" : ""}`;

  selectedSites.clear();
  renderTable();
}

function containerBadge(ctxId) {
  const c = containersByID[ctxId];
  const name = c ? c.name : `Unknown (${ctxId})`;
  const color = c ? CONTAINER_COLORS[c.color] || "#7c7c7d" : "#7c7c7d";
  const span = document.createElement("span");
  span.className = "container-badge";
  const dot = document.createElement("span");
  dot.className = "container-dot";
  dot.style.background = color;
  span.appendChild(dot);
  span.appendChild(document.createTextNode(name));
  return span;
}

function renderTable() {
  const raw = document.getElementById("input-site").value.toLowerCase();
  const terms = raw.split(",").map((s) => s.trim()).filter(Boolean);
  const containerFilter = getActiveContainerId();
  const tbody = document.getElementById("rules-body");
  const emptyState = document.getElementById("empty-state");
  const emptyMsg = document.getElementById("empty-message");

  let filtered = allRules;
  if (containerFilter) {
    const ctxId = parseInt(containerFilter);
    filtered = filtered.filter((r) => r.userContextId === ctxId);
  }
  if (terms.length > 0) {
    filtered = filtered.filter((r) =>
      terms.some((t) => r.site.toLowerCase().includes(t))
    );
  }

  filtered.sort((a, b) => {
    let va, vb;
    if (sortField === "site") {
      va = a.site;
      vb = b.site;
    } else {
      va = (containersByID[a.userContextId]?.name || "").toLowerCase();
      vb = (containersByID[b.userContextId]?.name || "").toLowerCase();
    }
    const cmp = va < vb ? -1 : va > vb ? 1 : 0;
    return sortAsc ? cmp : -cmp;
  });

  tbody.textContent = "";

  if (filtered.length === 0) {
    if (containerFilter && terms.length === 0) {
      emptyMsg.textContent = "No rules in this container. Add sites above.";
    } else if (terms.length > 0) {
      emptyMsg.textContent = "No matching sites found.";
    } else {
      emptyMsg.textContent = "No site rules found. Select a container to add rules, or import from a JSON file.";
    }
    emptyState.classList.remove("hidden");
    document.getElementById("rules-table").classList.add("hidden");
    return;
  }

  emptyState.classList.add("hidden");
  document.getElementById("rules-table").classList.remove("hidden");

  filtered.forEach((rule) => {
    const tr = document.createElement("tr");
    if (selectedSites.has(rule.site)) tr.classList.add("selected");

    const tdCheck = tr.insertCell();
    tdCheck.className = "col-check";
    const cb = document.createElement("input");
    cb.type = "checkbox";
    cb.dataset.site = rule.site;
    cb.checked = selectedSites.has(rule.site);
    tdCheck.appendChild(cb);

    const tdSite = tr.insertCell();
    tdSite.className = "col-site";
    tdSite.textContent = rule.site;

    const tdContainer = tr.insertCell();
    tdContainer.className = "col-container";
    tdContainer.appendChild(containerBadge(rule.userContextId));

    const tdNever = tr.insertCell();
    tdNever.className = "col-neverask";
    tdNever.textContent = rule.neverAsk ? "Yes" : "No";

    const tdActions = tr.insertCell();
    tdActions.className = "col-actions";
    const editBtn = document.createElement("button");
    editBtn.className = "action-btn edit";
    editBtn.dataset.site = rule.site;
    editBtn.title = "Edit";
    editBtn.textContent = "Edit";
    const delBtn = document.createElement("button");
    delBtn.className = "action-btn delete";
    delBtn.dataset.site = rule.site;
    delBtn.title = "Delete";
    delBtn.textContent = "Del";
    tdActions.appendChild(editBtn);
    tdActions.appendChild(delBtn);

    tbody.appendChild(tr);
  });

  updateBulkActions();
}

function updateBulkActions() {
  const bulk = document.getElementById("bulk-actions");
  const count = document.getElementById("selected-count");
  if (selectedSites.size > 0) {
    bulk.classList.remove("hidden");
    count.textContent = `${selectedSites.size} selected`;
  } else {
    bulk.classList.add("hidden");
  }
}

function showLockPanel() {
  document.getElementById("mac-lock-panel").classList.remove("hidden");
  document.getElementById("container-picker").classList.add("hidden");
  document.getElementById("table-section").classList.add("hidden");
}

function hideLockPanel() {
  document.getElementById("mac-lock-panel").classList.add("hidden");
  document.getElementById("container-picker").classList.remove("hidden");
  document.getElementById("table-section").classList.remove("hidden");
}

function showReminder() {
  document.getElementById("mac-reminder").classList.remove("hidden");
}

function isLockedError(error) {
  return error && error.includes("SQLITE_BUSY");
}

async function loadRules() {
  document.getElementById("loading").classList.remove("hidden");

  const macStatus = await sendCommand({ cmd: "check-mac" });
  if (macStatus.ok && macStatus.data.enabled) {
    document.getElementById("loading").classList.add("hidden");
    showLockPanel();
    return;
  }

  const resp = await sendCommand({ cmd: "list" });
  document.getElementById("loading").classList.add("hidden");

  if (!resp.ok) {
    if (isLockedError(resp.error)) {
      showLockPanel();
      return;
    }
    showBanner(resp.error, "error");
    return;
  }

  hideLockPanel();
  showReminder();

  allRules = resp.data.rules || [];
  containers = resp.data.containers || [];
  containersByID = {};
  containers.forEach((c) => { containersByID[c.userContextId] = c; });

  populateContainerDropdowns();
  updateContainerView();
}

async function handleAdd() {
  const raw = document.getElementById("input-site").value;
  const cleaned = raw.split(",").map(cleanHostname).filter(Boolean);
  const ctxId = parseInt(getActiveContainerId());
  const neverAsk = document.getElementById("check-neverask").checked;

  if (cleaned.length === 0) {
    showBanner("Please enter a hostname.", "error");
    return;
  }

  const invalid = cleaned.filter((s) => !isValidHostname(s));
  if (invalid.length > 0) {
    showBanner(`Invalid hostname${invalid.length > 1 ? "s" : ""}: ${invalid.join(", ")}`, "error");
    return;
  }

  const sites = cleaned;

  const conflicts = [];
  for (const site of sites) {
    const existing = allRules.find((r) => r.site === site && r.userContextId !== ctxId);
    if (existing) {
      const cName = containersByID[existing.userContextId]?.name || `Container ${existing.userContextId}`;
      conflicts.push(`${site} (currently in ${cName})`);
    }
  }

  if (conflicts.length > 0) {
    const targetName = containersByID[ctxId]?.name || `Container ${ctxId}`;
    if (!confirm(`These sites already exist in other containers and will be reassigned to ${targetName}:\n\n${conflicts.join("\n")}\n\nContinue?`)) {
      return;
    }
  }

  const errors = [];
  for (const site of sites) {
    const resp = await sendCommand({ cmd: "add", site, userContextId: ctxId, neverAsk });
    if (!resp.ok) {
      errors.push(`${site}: ${resp.error}`);
    }
  }

  document.getElementById("input-site").value = "";

  if (errors.length > 0) {
    showBanner(`Failed: ${errors.join("; ")}`, "error");
  } else {
    showBanner(sites.length === 1 ? "Saved." : `Added ${sites.length} rules.`, "success");
  }
  await loadRules();
}

async function handleDelete(site) {
  if (!confirm(`Delete rule for "${site}"?`)) return;
  const resp = await sendCommand({ cmd: "delete", site });
  if (!resp.ok) {
    showBanner(resp.error, "error");
    return;
  }
  showBanner("Deleted.", "success");
  await loadRules();
}

async function handleEdit(site) {
  const rule = allRules.find((r) => r.site === site);
  if (!rule) return;

  const row = document.querySelector(`button.edit[data-site="${site}"]`).closest("tr");
  const containerCell = row.querySelector(".col-container");

  const select = document.createElement("select");
  select.className = "edit-select";
  containers.forEach((c) => {
    const opt = document.createElement("option");
    opt.value = c.userContextId;
    opt.textContent = c.name;
    if (c.userContextId === rule.userContextId) opt.selected = true;
    select.appendChild(opt);
  });

  containerCell.textContent = "";
  containerCell.appendChild(select);
  select.focus();

  let saved = false;
  const save = async () => {
    if (saved) return;
    saved = true;
    const newCtxId = parseInt(select.value);
    if (newCtxId !== rule.userContextId) {
      const resp = await sendCommand({ cmd: "update", site, userContextId: newCtxId, neverAsk: rule.neverAsk });
      if (!resp.ok) {
        showBanner(resp.error, "error");
      } else {
        showBanner("Saved.", "success");
      }
    }
    await loadRules();
  };

  select.addEventListener("change", save);
  select.addEventListener("blur", save);
  select.addEventListener("keydown", (e) => {
    if (e.key === "Escape") {
      saved = true;
      containerCell.textContent = "";
      containerCell.appendChild(containerBadge(rule.userContextId));
    }
  });
}


async function handleExport() {
  const resp = await sendCommand({ cmd: "export" });
  if (!resp.ok) {
    showBanner(resp.error, "error");
    return;
  }

  const json = JSON.stringify(resp.data, null, 2);
  const blob = new Blob([json], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const timestamp = new Date().toISOString().replace(/[:.]/g, "-").slice(0, 19);

  browser.downloads.download({
    url,
    filename: `container-rules-${timestamp}.json`,
    saveAs: true,
  });
}

async function handleImport(file) {
  const text = await file.text();
  let data;
  try {
    data = JSON.parse(text);
  } catch {
    showBanner("Invalid JSON file.", "error");
    return;
  }

  const rules = data.rules || [];
  if (rules.length === 0) {
    showBanner("No rules found in file.", "error");
    return;
  }

  const importRules = rules.map((r) => ({
    site: r.site,
    userContextId: r.userContextId,
    neverAsk: r.neverAsk !== undefined ? r.neverAsk : true,
  }));

  if (!confirm(`Import ${importRules.length} rules?`)) return;

  const mode = confirm(
    "Replace all existing rules?\n\nOK = Replace (delete all, then import)\nCancel = Merge (add new, update existing)"
  ) ? "replace" : "merge";

  const resp = await sendCommand({ cmd: "import", rules: importRules, mode });
  if (!resp.ok) {
    showBanner(resp.error, "error");
    return;
  }

  const action = mode === "replace" ? "Replaced" : "Merged";
  showBanner(`${action}: ${resp.data.added} added, ${resp.data.updated} updated, ${resp.data.skipped} skipped.`, "success");
  await loadRules();
}

async function handleBulkDelete() {
  for (const site of selectedSites) {
    await sendCommand({ cmd: "delete", site });
  }
  selectedSites.clear();
  showBanner("Deleted selected rules.", "success");
  await loadRules();
}

async function handleBulkReassign() {
  const ctxId = parseInt(document.getElementById("bulk-reassign-container").value);
  for (const site of selectedSites) {
    const rule = allRules.find((r) => r.site === site);
    await sendCommand({ cmd: "update", site, userContextId: ctxId, neverAsk: rule?.neverAsk ?? true });
  }
  selectedSites.clear();
  showBanner("Reassigned selected rules.", "success");
  await loadRules();
}

function init() {
  document.getElementById("btn-add").addEventListener("click", handleAdd);
  document.getElementById("input-site").addEventListener("keydown", (e) => {
    if (e.key === "Enter") handleAdd();
  });
  document.getElementById("btn-export").addEventListener("click", handleExport);
  document.getElementById("btn-import").addEventListener("change", (e) => {
    if (e.target.files[0]) handleImport(e.target.files[0]);
    e.target.value = "";
  });
  document.getElementById("input-site").addEventListener("input", renderTable);
  document.getElementById("active-container").addEventListener("change", updateContainerView);
  document.getElementById("btn-bulk-delete").addEventListener("click", handleBulkDelete);
  document.getElementById("btn-bulk-reassign").addEventListener("click", handleBulkReassign);

  document.getElementById("btn-retry").addEventListener("click", loadRules);

  function copyAddonsURL(btn) {
    navigator.clipboard.writeText("about:addons").then(() => {
      const orig = btn.textContent;
      btn.textContent = "Copied!";
      setTimeout(() => { btn.textContent = orig; }, 1500);
    });
  }
  document.getElementById("btn-copy-addons").addEventListener("click", (e) => copyAddonsURL(e.target));

  document.getElementById("check-all").addEventListener("change", (e) => {
    if (e.target.checked) {
      allRules.forEach((r) => selectedSites.add(r.site));
    } else {
      selectedSites.clear();
    }
    renderTable();
  });

  document.getElementById("rules-body").addEventListener("click", (e) => {
    const btn = e.target.closest("button");
    if (btn?.classList.contains("edit")) {
      handleEdit(btn.dataset.site);
    } else if (btn?.classList.contains("delete")) {
      handleDelete(btn.dataset.site);
    }
  });

  document.getElementById("rules-body").addEventListener("change", (e) => {
    if (e.target.type === "checkbox") {
      const site = e.target.dataset.site;
      if (e.target.checked) {
        selectedSites.add(site);
      } else {
        selectedSites.delete(site);
      }
      renderTable();
    }
  });

  document.querySelectorAll("th.sortable").forEach((th) => {
    th.addEventListener("click", () => {
      const field = th.dataset.sort;
      if (sortField === field) {
        sortAsc = !sortAsc;
      } else {
        sortField = field;
        sortAsc = true;
      }
      renderTable();
    });
  });

  loadRules();
}

document.addEventListener("DOMContentLoaded", init);
