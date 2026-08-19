const dropZone = document.querySelector("#dropZone");
const fileInput = document.querySelector("#fileInput");
const errorEl = document.querySelector("#error");

const workspace = document.querySelector("#workspace");
const imageCountEl = document.querySelector("#imageCount");
const startOverBtn = document.querySelector("#startOver");

const formatSelect = document.querySelector("#formatSelect");
const qualityOption = document.querySelector("#qualityOption");
const qualityRange = document.querySelector("#qualityRange");
const qualityValue = document.querySelector("#qualityValue");
const maxDimension = document.querySelector("#maxDimension");
const outputFolderBtn = document.querySelector("#outputFolderBtn");

const convertBtn = document.querySelector("#convertBtn");
const convertStatus = document.querySelector("#convertStatus");
const imageGrid = document.querySelector("#imageGrid");

let batchId = null;
let images = []; // [{id, filename, info, error, file, converted, convertedFilename, convertedPath}]
let outputDir = ""; // "" = app default (Downloads)

function showError(message) {
  errorEl.textContent = message;
  errorEl.classList.remove("hidden");
}
function clearError() {
  errorEl.classList.add("hidden");
  errorEl.textContent = "";
}

function resetToDropZone() {
  batchId = null;
  images = [];
  imageGrid.innerHTML = "";
  fileInput.value = "";
  workspace.classList.add("hidden");
  dropZone.classList.remove("hidden");
  clearError();
}

// ---- options ------------------------------------------------------------

function updateQualityVisibility() {
  qualityOption.classList.toggle("hidden", formatSelect.value !== "jpeg");
}
formatSelect.addEventListener("change", updateQualityVisibility);
updateQualityVisibility();

qualityRange.addEventListener("input", () => {
  qualityValue.textContent = qualityRange.value;
});

outputFolderBtn.addEventListener("click", async () => {
  try {
    const res = await fetch("/api/dialog/output-folder", { method: "POST" });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "couldn't open folder picker");
    if (data.path) {
      outputDir = data.path;
      outputFolderBtn.textContent = shortenPath(data.path);
      outputFolderBtn.title = data.path;
    }
  } catch (err) {
    showError(String(err.message || err));
  }
});

function shortenPath(path) {
  const parts = path.split("/").filter(Boolean);
  return parts.length > 0 ? parts[parts.length - 1] : path;
}

// ---- drop zone ------------------------------------------------------------

dropZone.addEventListener("click", () => fileInput.click());
fileInput.addEventListener("change", () => handleFiles(fileInput.files));

["dragenter", "dragover"].forEach((evt) =>
  dropZone.addEventListener(evt, (e) => {
    e.preventDefault();
    dropZone.classList.add("dragover");
  })
);
["dragleave", "drop"].forEach((evt) =>
  dropZone.addEventListener(evt, (e) => {
    e.preventDefault();
    dropZone.classList.remove("dragover");
  })
);
dropZone.addEventListener("drop", (e) => {
  if (e.dataTransfer?.files?.length) handleFiles(e.dataTransfer.files);
});

async function handleFiles(fileList) {
  const files = Array.from(fileList);
  if (files.length === 0) return;
  clearError();

  const form = new FormData();
  for (const file of files) form.append("file", file, file.name);

  let data;
  try {
    const res = await fetch("/api/upload", { method: "POST", body: form });
    data = await res.json();
    if (!res.ok) throw new Error(data.error || "upload failed");
  } catch (err) {
    showError(String(err.message || err));
    return;
  }

  batchId = data.batchId;
  images = data.images.map((p, i) => ({ ...p, file: files[i], converted: false }));

  dropZone.classList.add("hidden");
  workspace.classList.remove("hidden");
  renderGrid();
}

// ---- rendering ------------------------------------------------------------

function renderGrid() {
  imageCountEl.textContent = `${images.length} image${images.length === 1 ? "" : "s"}`;
  imageGrid.innerHTML = "";
  images.forEach((p) => imageGrid.appendChild(renderCard(p)));
  updateConvertStatus();
}

function renderCard(p) {
  const card = document.createElement("div");
  card.className = "image-card";
  card.dataset.id = p.id;

  const img = document.createElement("img");
  img.className = "image-thumb";
  img.src = URL.createObjectURL(p.file);
  img.alt = "";
  card.appendChild(img);

  const body = document.createElement("div");
  body.className = "image-body";

  const top = document.createElement("div");
  top.className = "image-top";

  const checkbox = document.createElement("input");
  checkbox.type = "checkbox";
  checkbox.checked = !p.error;
  checkbox.disabled = !!p.error;
  checkbox.addEventListener("change", updateConvertStatus);
  top.appendChild(checkbox);

  const name = document.createElement("span");
  name.className = "image-name";
  name.textContent = p.filename;
  name.title = p.filename;
  top.appendChild(name);
  body.appendChild(top);

  if (p.error) {
    const err = document.createElement("div");
    err.className = "image-error";
    err.textContent = p.error;
    body.appendChild(err);
  } else if (p.info) {
    const meta = document.createElement("div");
    meta.className = "image-meta";
    meta.textContent = `${p.info.format.toUpperCase()} · ${p.info.width}×${p.info.height} · ${formatBytes(p.info.bytes)}`;
    body.appendChild(meta);
  }

  const actions = document.createElement("div");
  actions.className = "image-actions";
  body.appendChild(actions);

  card.appendChild(body);
  card._checkbox = checkbox;
  card._actions = actions;
  return card;
}

function formatBytes(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function updateConvertStatus() {
  const selected = images.filter((p) => !p.error && !p.converted && cardFor(p.id)?._checkbox?.checked);
  convertBtn.disabled = selected.length === 0;
  convertStatus.textContent = selected.length === 0 ? "" : `${selected.length} selected`;
}

function cardFor(id) {
  return imageGrid.querySelector(`.image-card[data-id="${id}"]`);
}

// ---- convert ------------------------------------------------------------

convertBtn.addEventListener("click", async () => {
  const ids = images
    .filter((p) => !p.error && !p.converted && cardFor(p.id)?._checkbox?.checked)
    .map((p) => p.id);
  if (ids.length === 0) return;

  convertBtn.disabled = true;
  convertStatus.textContent = "Converting…";

  try {
    const res = await fetch("/api/convert", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        batchId,
        ids,
        format: formatSelect.value,
        quality: Number(qualityRange.value),
        maxDimension: Number(maxDimension.value),
        outputDir,
      }),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "conversion failed");

    for (const r of data.results) {
      const image = images.find((p) => p.id === r.id);
      if (!image) continue;
      if (r.error) {
        image.error = r.error;
      } else {
        image.converted = true;
        image.convertedFilename = r.filename;
        image.convertedPath = r.path;
      }
      updateCard(image);
    }
    const okCount = data.results.filter((r) => !r.error).length;
    convertStatus.textContent = `Saved ${okCount} image${okCount === 1 ? "" : "s"} to ${shortenPath(data.outputDir)}.`;
  } catch (err) {
    showError(String(err.message || err));
    convertStatus.textContent = "";
  } finally {
    updateConvertStatus();
  }
});

function updateCard(image) {
  const card = cardFor(image.id);
  if (!card) return;

  if (image.converted) {
    card.classList.add("converted");
    card._checkbox.checked = false;
    card._checkbox.disabled = true;

    const badge = document.createElement("div");
    badge.className = "converted-badge";
    badge.textContent = `✓ Converted — saved as ${image.convertedFilename}`;
    card.querySelector(".image-body").insertBefore(badge, card._actions);

    const openBtn = document.createElement("button");
    openBtn.type = "button";
    openBtn.className = "link-btn";
    openBtn.textContent = "Open file";
    openBtn.addEventListener("click", () => {
      fetch("/api/open", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ path: image.convertedPath }) });
    });
    const revealBtn = document.createElement("button");
    revealBtn.type = "button";
    revealBtn.className = "link-btn";
    revealBtn.textContent = "Show in folder";
    revealBtn.addEventListener("click", () => {
      fetch("/api/reveal", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ path: image.convertedPath }) });
    });
    card._actions.append(openBtn, revealBtn);
  } else if (image.error) {
    const body = card.querySelector(".image-body");
    const existing = body.querySelector(".image-error");
    if (existing) {
      existing.textContent = image.error;
    } else {
      const err = document.createElement("div");
      err.className = "image-error";
      err.textContent = image.error;
      body.appendChild(err);
    }
  }
}

// ---- start over ------------------------------------------------------------

startOverBtn.addEventListener("click", resetToDropZone);
