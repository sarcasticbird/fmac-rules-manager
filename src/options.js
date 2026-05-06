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

function showBanner(message, type) {
  const banner = document.getElementById("banner");
  banner.textContent = message;
  banner.className = `banner ${type}`;
  banner.classList.remove("hidden");
  if (type === "success") {
    setTimeout(() => banner.classList.add("hidden"), 3000);
  }
}

function hideBanner() {
  document.getElementById("banner").classList.add("hidden");
}

function populateContainerDropdowns() {
  const selects = [
    document.getElementById("select-container"),
    document.getElementById("bulk-reassign-container"),
  ];
  selects.forEach((sel) => {
    sel.innerHTML = "";
    containers.forEach((c) => {
      const opt = document.createElement("option");
      opt.value = c.userContextId;
      opt.textContent = c.name;
      sel.appendChild(opt);
    });
  });
}

function containerBadge(ctxId) {
  const c = containersByID[ctxId];
  const name = c ? escapeHTML(c.name) : `Unknown (${ctxId})`;
  const color = c ? CONTAINER_COLORS[c.color] || "#7c7c7d" : "#7c7c7d";
  return `<span class="container-badge"><span class="container-dot" style="background:${color}"></span>${name}</span>`;
}

function renderTable() {
  const filter = document.getElementById("input-filter").value.toLowerCase();
  const tbody = document.getElementById("rules-body");
  const emptyState = document.getElementById("empty-state");

  let filtered = allRules;
  if (filter) {
    filtered = allRules.filter((r) => {
      const cName = (containersByID[r.userContextId]?.name || "").toLowerCase();
      return r.site.toLowerCase().includes(filter) || cName.includes(filter);
    });
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

  tbody.innerHTML = "";

  if (filtered.length === 0) {
    emptyState.classList.remove("hidden");
    document.getElementById("rules-table").classList.add("hidden");
    return;
  }

  emptyState.classList.add("hidden");
  document.getElementById("rules-table").classList.remove("hidden");

  filtered.forEach((rule) => {
    const tr = document.createElement("tr");
    if (selectedSites.has(rule.site)) tr.classList.add("selected");

    const safeSite = escapeHTML(rule.site);
    tr.innerHTML = `
      <td class="col-check"><input type="checkbox" data-site="${safeSite}" ${selectedSites.has(rule.site) ? "checked" : ""}></td>
      <td class="col-site">${safeSite}</td>
      <td class="col-container">${containerBadge(rule.userContextId)}</td>
      <td class="col-neverask">${rule.neverAsk ? "Yes" : "No"}</td>
      <td class="col-actions">
        <button class="action-btn edit" data-site="${safeSite}" title="Edit">Edit</button>
        <button class="action-btn delete" data-site="${safeSite}" title="Delete">Del</button>
      </td>
    `;
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

async function startSession() {
  const resp = await sendCommand({ cmd: "begin-session" });
  if (!resp.ok) {
    showBanner(resp.error, "error");
    return false;
  }
  return true;
}

async function loadRules() {
  document.getElementById("loading").classList.remove("hidden");
  const resp = await sendCommand({ cmd: "list" });
  document.getElementById("loading").classList.add("hidden");

  if (!resp.ok) {
    showBanner(resp.error, "error");
    return;
  }

  allRules = resp.data.rules || [];
  containers = resp.data.containers || [];
  containersByID = {};
  containers.forEach((c) => { containersByID[c.userContextId] = c; });

  populateContainerDropdowns();
  renderTable();
}

async function handleAdd() {
  const site = document.getElementById("input-site").value.trim().toLowerCase();
  const ctxId = parseInt(document.getElementById("select-container").value);
  const neverAsk = document.getElementById("check-neverask").checked;

  if (!site) {
    showBanner("Please enter a hostname.", "error");
    return;
  }

  const resp = await sendCommand({ cmd: "add", site, userContextId: ctxId, neverAsk });
  if (!resp.ok) {
    showBanner(resp.error, "error");
    return;
  }

  document.getElementById("input-site").value = "";
  showSuccess();
  await loadRules();
}

async function handleDelete(site) {
  if (!confirm(`Delete rule for "${site}"?`)) return;
  const resp = await sendCommand({ cmd: "delete", site });
  if (!resp.ok) {
    showBanner(resp.error, "error");
    return;
  }
  showSuccess();
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

  containerCell.innerHTML = "";
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
        showSuccess();
      }
    }
    await loadRules();
  };

  select.addEventListener("change", save);
  select.addEventListener("blur", save);
  select.addEventListener("keydown", (e) => {
    if (e.key === "Escape") {
      saved = true;
      containerCell.innerHTML = containerBadge(rule.userContextId);
    }
  });
}

function showSuccess(message) {
  showBanner(message || "Saved.", "success");
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

  const rules = data.rules || data.siteMappings || [];
  if (rules.length === 0) {
    showBanner("No rules found in file.", "error");
    return;
  }

  const importRules = rules.map((r) => ({
    site: r.site,
    userContextId: r.userContextId,
    neverAsk: r.neverAsk !== undefined ? r.neverAsk : true,
  }));

  const resp = await sendCommand({ cmd: "import", rules: importRules });
  if (!resp.ok) {
    showBanner(resp.error, "error");
    return;
  }

  showBanner(`Imported: ${resp.data.added} added, ${resp.data.updated} updated, ${resp.data.skipped} skipped.`, "success");
  showSuccess();
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
  document.getElementById("input-filter").addEventListener("input", renderTable);
  document.getElementById("btn-bulk-delete").addEventListener("click", handleBulkDelete);
  document.getElementById("btn-bulk-reassign").addEventListener("click", handleBulkReassign);

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

  startSession().then((ok) => {
    if (ok) loadRules();
  });

  window.addEventListener("beforeunload", () => {
    sendCommand({ cmd: "end-session" });
  });
}

document.addEventListener("DOMContentLoaded", init);
