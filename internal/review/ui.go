package review

// reviewUIHTML is the single-page application served at GET /.
const reviewUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Draft Review</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
:root {
  --sidebar-w: 250px;
  --panel-w: 350px;
  --border: #e2e2e5;
  --bg: #ffffff;
  --bg-muted: #f8f8fa;
  --text: #1a1a2e;
  --text-muted: #6b6b80;
  --accent: #2563eb;
  --accent-light: #dbeafe;
  --success: #16a34a;
  --warning: #d97706;
  --radius: 6px;
}
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  color: var(--text);
  background: var(--bg);
  height: 100vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.layout {
  display: grid;
  grid-template-columns: var(--sidebar-w) 1fr 0px;
  flex: 1;
  overflow: hidden;
  transition: grid-template-columns 0.2s;
}
.layout.panel-open {
  grid-template-columns: var(--sidebar-w) 1fr var(--panel-w);
}

/* Sidebar */
.sidebar {
  border-right: 1px solid var(--border);
  overflow-y: auto;
  background: var(--bg-muted);
  padding: 1rem 0;
}
.sidebar h2 {
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  padding: 0 1rem;
  margin-bottom: 0.5rem;
}
.doc-list {
  list-style: none;
}
.doc-item {
  padding: 0.5rem 1rem;
  cursor: pointer;
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.875rem;
  border-left: 3px solid transparent;
}
.doc-item:hover { background: var(--border); }
.doc-item.active {
  background: var(--accent-light);
  border-left-color: var(--accent);
}
.doc-item .title { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.badge {
  background: var(--accent);
  color: white;
  font-size: 0.7rem;
  padding: 0.1rem 0.4rem;
  border-radius: 9999px;
  min-width: 1.2rem;
  text-align: center;
}
.badge.zero { background: var(--border); color: var(--text-muted); }

/* Center content */
.center {
  overflow-y: auto;
  padding: 2rem 3rem;
  position: relative;
}
.center .doc-content {
  max-width: 800px;
  margin: 0 auto;
  line-height: 1.7;
}
.center .doc-content h1 { font-size: 2rem; margin-bottom: 1rem; }
.center .doc-content h2 { font-size: 1.5rem; margin: 1.5rem 0 0.75rem; padding-bottom: 0.25rem; border-bottom: 1px solid var(--border); }
.center .doc-content h3 { font-size: 1.25rem; margin: 1.25rem 0 0.5rem; }
.center .doc-content p { margin: 0.75rem 0; position: relative; }
.center .doc-content ul, .center .doc-content ol { margin: 0.75rem 0; padding-left: 1.5rem; }
.center .doc-content li { margin: 0.25rem 0; }
.center .doc-content code { background: var(--bg-muted); padding: 0.15rem 0.35rem; border-radius: 3px; font-size: 0.875em; }
.center .doc-content pre { background: var(--bg-muted); padding: 1rem; border-radius: var(--radius); overflow-x: auto; margin: 1rem 0; border: 1px solid var(--border); }
.center .doc-content pre code { background: none; padding: 0; }
.center .doc-content blockquote { border-left: 3px solid var(--border); padding-left: 1rem; color: var(--text-muted); margin: 1rem 0; }
.center .doc-content table { border-collapse: collapse; width: 100%; margin: 1rem 0; }
.center .doc-content th, .center .doc-content td { border: 1px solid var(--border); padding: 0.5rem; text-align: left; }
.center .doc-content th { background: var(--bg-muted); font-weight: 600; }

/* Gutter markers */
.gutter-marker {
  position: absolute;
  left: -2rem;
  width: 1.2rem;
  height: 1.2rem;
  background: var(--accent);
  color: white;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.6rem;
  cursor: pointer;
  font-weight: 600;
}
.gutter-marker.resolved { background: var(--success); opacity: 0.5; }

/* Floating comment button */
.comment-btn {
  position: absolute;
  display: none;
  background: var(--accent);
  color: white;
  border: none;
  padding: 0.35rem 0.75rem;
  border-radius: var(--radius);
  font-size: 0.8rem;
  cursor: pointer;
  z-index: 100;
  box-shadow: 0 2px 8px rgba(0,0,0,0.15);
}
.comment-btn:hover { opacity: 0.9; }

/* Right panel */
.panel {
  border-left: 1px solid var(--border);
  overflow-y: auto;
  background: var(--bg);
  display: flex;
  flex-direction: column;
}
.panel-header {
  padding: 1rem;
  border-bottom: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.panel-header h3 { font-size: 0.875rem; font-weight: 600; }
.panel-close {
  background: none;
  border: none;
  font-size: 1.2rem;
  cursor: pointer;
  color: var(--text-muted);
}
.panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 1rem;
}
.comment-item {
  margin-bottom: 1rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid var(--border);
}
.comment-item:last-child { border-bottom: none; }
.comment-author { font-weight: 600; font-size: 0.8rem; }
.comment-time { font-size: 0.7rem; color: var(--text-muted); margin-left: 0.5rem; }
.comment-body { margin-top: 0.25rem; font-size: 0.875rem; line-height: 1.5; }
.comment-excerpt {
  background: var(--bg-muted);
  border-left: 3px solid var(--accent);
  padding: 0.5rem;
  margin-bottom: 0.75rem;
  font-size: 0.8rem;
  color: var(--text-muted);
  font-style: italic;
}
.panel-footer {
  padding: 1rem;
  border-top: 1px solid var(--border);
}
.panel-footer textarea {
  width: 100%;
  min-height: 60px;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 0.5rem;
  font-family: inherit;
  font-size: 0.875rem;
  resize: vertical;
}
.panel-footer .actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.5rem;
  justify-content: space-between;
}
.btn {
  padding: 0.35rem 0.75rem;
  border-radius: var(--radius);
  border: 1px solid var(--border);
  background: var(--bg);
  font-size: 0.8rem;
  cursor: pointer;
}
.btn:hover { background: var(--bg-muted); }
.btn-primary { background: var(--accent); color: white; border-color: var(--accent); }
.btn-primary:hover { opacity: 0.9; }
.btn-success { background: var(--success); color: white; border-color: var(--success); }
.btn-success:hover { opacity: 0.9; }

/* Status bar */
.status-bar {
  border-top: 1px solid var(--border);
  padding: 0.4rem 1rem;
  display: flex;
  align-items: center;
  gap: 1rem;
  font-size: 0.75rem;
  color: var(--text-muted);
  background: var(--bg-muted);
}
.status-bar .repo { font-weight: 500; }
.status-bar .spacer { flex: 1; }
.status-bar .pending { color: var(--warning); font-weight: 500; }
.status-bar button {
  padding: 0.2rem 0.5rem;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg);
  font-size: 0.7rem;
  cursor: pointer;
}
.status-bar button:hover { background: var(--border); }

/* Empty state */
.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--text-muted);
  font-size: 0.9rem;
}
</style>
</head>
<body>

<div class="layout" id="layout">
  <aside class="sidebar">
    <h2>Documents</h2>
    <ul class="doc-list" id="doc-list"></ul>
  </aside>
  <main class="center" id="center">
    <div class="empty-state">Select a document to begin reviewing</div>
  </main>
  <aside class="panel" id="panel">
    <div class="panel-header">
      <h3 id="panel-title">Thread</h3>
      <button class="panel-close" onclick="closePanel()">&times;</button>
    </div>
    <div class="panel-body" id="panel-body"></div>
    <div class="panel-footer" id="panel-footer">
      <textarea id="reply-input" placeholder="Write a reply..."></textarea>
      <div class="actions">
        <div>
          <button class="btn btn-success" id="btn-resolve" onclick="resolveThread()">Resolve</button>
          <button class="btn" id="btn-reopen" onclick="reopenThread()" style="display:none">Reopen</button>
        </div>
        <button class="btn btn-primary" onclick="postReply()">Reply</button>
      </div>
    </div>
  </aside>
</div>

<button class="comment-btn" id="comment-btn" onclick="startNewThread()">Comment</button>

<div class="status-bar">
  <span class="repo" id="status-repo">--</span>
  <span id="status-branch">--</span>
  <span class="spacer"></span>
  <span class="pending" id="status-pending"></span>
  <button onclick="doSync()">Sync</button>
  <button onclick="doPublish()">Publish</button>
</div>

<script>
let currentDoc = null;
let currentThread = null;
let documents = [];
let selectionAnchor = null;

async function api(path, opts) {
  const res = await fetch(path, opts);
  if (!res.ok && res.status !== 201) throw new Error(await res.text());
  return res.json();
}

async function loadDocuments() {
  documents = await api('/api/documents');
  if (!documents) documents = [];
  renderDocList();
}

function renderDocList() {
  const list = document.getElementById('doc-list');
  list.innerHTML = '';
  documents.sort((a, b) => a.path.localeCompare(b.path));
  documents.forEach(doc => {
    const li = document.createElement('li');
    li.className = 'doc-item' + (currentDoc && currentDoc.path === doc.path ? ' active' : '');
    li.innerHTML = '<span class="title">' + escHtml(doc.title || doc.path) + '</span>' +
      '<span class="badge ' + (doc.thread_count === 0 ? 'zero' : '') + '">' + doc.thread_count + '</span>';
    li.onclick = () => selectDoc(doc.path);
    list.appendChild(li);
  });
}

async function selectDoc(path) {
  const detail = await api('/api/documents/' + encodeURIComponent(path));
  currentDoc = detail;
  currentThread = null;
  closePanel();
  renderDocContent();
  renderDocList();
}

function renderDocContent() {
  const center = document.getElementById('center');
  if (!currentDoc) {
    center.innerHTML = '<div class="empty-state">Select a document to begin reviewing</div>';
    return;
  }
  center.innerHTML = '<div class="doc-content">' + currentDoc.html + '</div>';
  addGutterMarkers();
  setupTextSelection();
}

function addGutterMarkers() {
  if (!currentDoc || !currentDoc.threads) return;
  const paragraphs = document.querySelectorAll('[data-paragraph-index]');
  const threadsByPara = {};
  currentDoc.threads.forEach(t => {
    const idx = t.anchor.paragraph_index;
    if (!threadsByPara[idx]) threadsByPara[idx] = [];
    threadsByPara[idx].push(t);
  });
  paragraphs.forEach(p => {
    const idx = parseInt(p.getAttribute('data-paragraph-index'), 10);
    const threads = threadsByPara[idx];
    if (!threads || threads.length === 0) return;
    p.style.position = 'relative';
    const openThreads = threads.filter(t => t.status === 'open');
    const marker = document.createElement('span');
    marker.className = 'gutter-marker' + (openThreads.length === 0 ? ' resolved' : '');
    marker.textContent = threads.length;
    marker.onclick = (e) => { e.stopPropagation(); showThread(threads[0]); };
    p.appendChild(marker);
  });
}

function setupTextSelection() {
  const center = document.getElementById('center');
  center.addEventListener('mouseup', (e) => {
    const sel = window.getSelection();
    if (!sel || sel.isCollapsed || !sel.toString().trim()) {
      hideCommentBtn();
      return;
    }
    const range = sel.getRangeAt(0);
    const paraEl = range.startContainer.parentElement.closest('[data-paragraph-index]');
    if (!paraEl) { hideCommentBtn(); return; }
    const rect = range.getBoundingClientRect();
    const centerRect = center.getBoundingClientRect();
    const btn = document.getElementById('comment-btn');
    btn.style.top = (rect.top - centerRect.top + center.scrollTop - 30) + 'px';
    btn.style.left = (rect.left - centerRect.left + rect.width / 2 - 30) + 'px';
    btn.style.display = 'block';
    selectionAnchor = {
      paragraphIndex: parseInt(paraEl.getAttribute('data-paragraph-index'), 10),
      excerpt: sel.toString().substring(0, 200)
    };
  });
}

function hideCommentBtn() {
  document.getElementById('comment-btn').style.display = 'none';
  selectionAnchor = null;
}

async function startNewThread() {
  if (!selectionAnchor || !currentDoc) return;
  const body = prompt('Enter your comment:');
  if (!body) { hideCommentBtn(); return; }
  const req = {
    review_id: '',
    document: currentDoc.path,
    anchor: {
      heading_path: [],
      paragraph_index: selectionAnchor.paragraphIndex,
      excerpt: selectionAnchor.excerpt,
      content_hash: '',
      char_range: [0, 0],
      source_ref: ''
    },
    body: body,
    author: ''
  };
  await api('/api/threads', { method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(req) });
  hideCommentBtn();
  await selectDoc(currentDoc.path);
}

function showThread(thread) {
  currentThread = thread;
  const layout = document.getElementById('layout');
  layout.classList.add('panel-open');
  document.getElementById('panel-title').textContent = 'Thread';
  renderThreadPanel();
}

function renderThreadPanel() {
  const body = document.getElementById('panel-body');
  if (!currentThread) { body.innerHTML = ''; return; }
  let html = '';
  if (currentThread.anchor && currentThread.anchor.excerpt) {
    html += '<div class="comment-excerpt">"' + escHtml(currentThread.anchor.excerpt) + '"</div>';
  }
  (currentThread.comments || []).forEach(c => {
    html += '<div class="comment-item">' +
      '<span class="comment-author">' + escHtml(c.author) + '</span>' +
      '<span class="comment-time">' + formatTime(c.created_at) + '</span>' +
      '<div class="comment-body">' + escHtml(c.body) + '</div>' +
      '</div>';
  });
  body.innerHTML = html;
  // Toggle resolve/reopen buttons.
  document.getElementById('btn-resolve').style.display = currentThread.status === 'open' ? '' : 'none';
  document.getElementById('btn-reopen').style.display = currentThread.status !== 'open' ? '' : 'none';
}

function closePanel() {
  document.getElementById('layout').classList.remove('panel-open');
  currentThread = null;
}

async function postReply() {
  if (!currentThread || !currentDoc) return;
  const input = document.getElementById('reply-input');
  const body = input.value.trim();
  if (!body) return;
  await api('/api/threads/' + currentThread.id + '/comments?document=' + encodeURIComponent(currentDoc.path),
    { method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({ body: body, author: '' }) });
  input.value = '';
  await refreshThread();
}

async function resolveThread() {
  if (!currentThread || !currentDoc) return;
  await api('/api/threads/' + currentThread.id + '/resolve?document=' + encodeURIComponent(currentDoc.path), { method: 'POST' });
  await refreshThread();
  await selectDoc(currentDoc.path);
}

async function reopenThread() {
  if (!currentThread || !currentDoc) return;
  await api('/api/threads/' + currentThread.id + '/reopen?document=' + encodeURIComponent(currentDoc.path), { method: 'POST' });
  await refreshThread();
  await selectDoc(currentDoc.path);
}

async function refreshThread() {
  if (!currentThread || !currentDoc) return;
  const threads = await api('/api/threads?document=' + encodeURIComponent(currentDoc.path));
  const updated = threads.find(t => t.id === currentThread.id);
  if (updated) { currentThread = updated; renderThreadPanel(); }
}

async function doSync() {
  await api('/api/sync', { method: 'POST' });
  await loadDocuments();
  await loadStatus();
  if (currentDoc) await selectDoc(currentDoc.path);
}

async function doPublish() {
  const res = await api('/api/publish', { method: 'POST' });
  if (res.error) { alert('Publish failed: ' + res.error); }
  await loadStatus();
}

async function loadStatus() {
  const s = await api('/api/status');
  document.getElementById('status-repo').textContent = s.repo_name || '--';
  document.getElementById('status-branch').textContent = s.branch || '--';
  const pending = document.getElementById('status-pending');
  if (s.pending_changes) {
    pending.textContent = 'Pending changes';
  } else {
    pending.textContent = '';
  }
}

function formatTime(ts) {
  if (!ts) return '';
  const d = new Date(ts);
  return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'});
}

function escHtml(s) {
  if (!s) return '';
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

// Initialize.
(async () => {
  await loadDocuments();
  await loadStatus();
  // Check URL hash for direct document open.
  if (window.location.hash) {
    const path = decodeURIComponent(window.location.hash.substring(1));
    if (path) await selectDoc(path);
  }
})();
</script>
</body>
</html>
`
