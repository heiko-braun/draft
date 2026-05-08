package review

// reviewUIHTML is the single-page application served at GET /.
const reviewUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Draft Review</title>
<link href="https://fonts.googleapis.com/css2?family=Geist:wght@400;500;600&display=swap" rel="stylesheet">
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
  font-family: 'Geist', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
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
.doc-group-header {
  padding: 0.4rem 1rem;
  font-size: 0.7rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  margin-top: 0.5rem;
}

/* Center content */
.center {
  overflow-y: auto;
  padding: 2rem 3rem;
  position: relative;
}
.center .doc-content {
  max-width: 900px;
  margin: 0 auto;
  line-height: 1.6;
}
.center .doc-content h1 {
  font-size: 2.25rem;
  font-weight: 600;
  margin: 0 0 2rem 0;
  color: hsl(240 10% 3.9%);
}
.center .doc-content h2 {
  font-size: 1.5rem;
  font-weight: 600;
  margin: 2rem 0 1rem 0;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid hsl(240 5.9% 90%);
  color: hsl(240 10% 3.9%);
}
.center .doc-content h3 {
  font-size: 1.25rem;
  font-weight: 600;
  margin: 1.5rem 0 0.75rem 0;
  color: hsl(240 10% 3.9%);
}
.center .doc-content p {
  margin: 1rem 0;
  color: hsl(240 3.8% 46.1%);
  position: relative;
}
.center .doc-content ul, .center .doc-content ol {
  margin: 1rem 0;
  padding-left: 1.5rem;
}
.center .doc-content li {
  margin: 0.5rem 0;
  color: hsl(240 3.8% 46.1%);
}
.center .doc-content code {
  background: hsl(240 4.8% 95.9%);
  padding: 0.2rem 0.4rem;
  border-radius: 0.25rem;
  font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, "Liberation Mono", monospace;
  font-size: 0.875em;
  color: hsl(240 10% 3.9%);
}
.center .doc-content pre {
  background: hsl(240 4.8% 95.9%);
  padding: 1rem;
  border-radius: 0.5rem;
  overflow-x: auto;
  margin: 1rem 0;
  border: 1px solid hsl(240 5.9% 90%);
}
.center .doc-content pre code {
  background: none;
  padding: 0;
}
.center .doc-content blockquote {
  border-left: 3px solid hsl(240 5.9% 90%);
  padding-left: 1rem;
  margin: 1rem 0;
  color: hsl(240 3.8% 46.1%);
}
.center .doc-content table {
  border-collapse: collapse;
  width: 100%;
  margin: 1rem 0;
  border: 1px solid hsl(240 5.9% 90%);
  border-radius: 0.5rem;
  overflow: hidden;
}
.center .doc-content th, .center .doc-content td {
  border-bottom: 1px solid hsl(240 5.9% 90%);
  padding: 0.75rem;
  text-align: left;
}
.center .doc-content th {
  background: hsl(240 4.8% 95.9%);
  font-weight: 500;
  color: hsl(240 10% 3.9%);
}
.center .doc-content tr:last-child td {
  border-bottom: none;
}
.center .doc-content input[type="checkbox"] {
  margin-right: 0.5rem;
  accent-color: hsl(240 10% 3.9%);
}

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

/* Inline highlights */
.review-highlight {
  background-color: rgba(37, 99, 235, 0.12);
  border-bottom: 2px solid var(--accent);
  cursor: pointer;
  border-radius: 2px;
  transition: background-color 0.15s;
}
.review-highlight:hover { background-color: rgba(37, 99, 235, 0.25); }
.review-highlight.resolved {
  background-color: rgba(22, 163, 74, 0.08);
  border-bottom-color: var(--success);
  opacity: 0.6;
}

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
.panel-close:hover { color: var(--text); }
.panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 1rem;
}

/* Comment modal */
.modal-overlay {
  display: none;
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.4);
  z-index: 1000;
  align-items: center;
  justify-content: center;
}
.modal-overlay.open { display: flex; }
.modal {
  background: var(--bg);
  border-radius: 0.5rem;
  width: 500px;
  max-width: 90vw;
  box-shadow: 0 20px 60px rgba(0,0,0,0.2);
  display: flex;
  flex-direction: column;
}
.modal-header {
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.modal-header h3 { font-size: 0.95rem; font-weight: 600; }
.modal-close {
  background: none;
  border: none;
  font-size: 1.3rem;
  cursor: pointer;
  color: var(--text-muted);
  line-height: 1;
}
.modal-close:hover { color: var(--text); }
.modal-body {
  padding: 1.25rem;
}
.modal-excerpt {
  background: var(--bg-muted);
  border-left: 3px solid var(--accent);
  padding: 0.5rem 0.75rem;
  margin-bottom: 1rem;
  font-size: 0.85rem;
  color: var(--text-muted);
  font-style: italic;
  border-radius: 0 var(--radius) var(--radius) 0;
}
.modal-body textarea {
  width: 100%;
  min-height: 100px;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 0.75rem;
  font-family: inherit;
  font-size: 0.875rem;
  resize: vertical;
}
.modal-body textarea:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 2px var(--accent-light);
}
.modal-footer {
  padding: 0.75rem 1.25rem;
  border-top: 1px solid var(--border);
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
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
          <button class="btn" style="color:var(--warning)" onclick="deleteThread()">Delete</button>
        </div>
        <button class="btn btn-primary" onclick="postReply()">Reply</button>
      </div>
    </div>
  </aside>
</div>

<div class="modal-overlay" id="comment-modal" onclick="onModalOverlayClick(event)">
  <div class="modal">
    <div class="modal-header">
      <h3>New Comment</h3>
      <button class="modal-close" onclick="closeCommentModal()">&times;</button>
    </div>
    <div class="modal-body">
      <div class="modal-excerpt" id="modal-excerpt"></div>
      <textarea id="modal-comment-input" placeholder="Write your comment..."></textarea>
    </div>
    <div class="modal-footer">
      <button class="btn" onclick="closeCommentModal()">Cancel</button>
      <button class="btn btn-primary" onclick="submitNewComment()">Submit</button>
    </div>
  </div>
</div>

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
  // Sort by most recently modified first
  documents.sort((a, b) => (b.mod_time || 0) - (a.mod_time || 0));
  // Group by top-level directory
  const groups = {};
  documents.forEach(doc => {
    const slash = doc.path.indexOf('/');
    const group = slash > 0 ? doc.path.substring(0, slash) : '.';
    if (!groups[group]) groups[group] = [];
    groups[group].push(doc);
  });
  // Render each group
  Object.keys(groups).sort().forEach(group => {
    const header = document.createElement('li');
    header.className = 'doc-group-header';
    header.textContent = group === '.' ? 'root' : group;
    list.appendChild(header);
    groups[group].forEach(doc => {
      const li = document.createElement('li');
      li.className = 'doc-item' + (currentDoc && currentDoc.path === doc.path ? ' active' : '');
      const fileName = doc.path.substring(doc.path.lastIndexOf('/') + 1);
      li.innerHTML = '<span class="title">' + escHtml(doc.title || fileName) + '</span>' +
        '<span class="badge ' + (doc.thread_count === 0 ? 'zero' : '') + '">' + doc.thread_count + '</span>';
      li.onclick = () => selectDoc(doc.path);
      list.appendChild(li);
    });
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
  center.innerHTML = '<div class="doc-content" id="doc-content-wrapper">' + currentDoc.html + '</div>';
  applyHighlights();
  setupTextSelection();
}

function applyHighlights() {
  if (!currentDoc || !currentDoc.threads) return;
  const paragraphs = document.querySelectorAll('[data-paragraph-index]');

  currentDoc.threads.forEach(t => {
    if (!t.anchor || !t.anchor.excerpt) return;
    const idx = t.anchor.paragraph_index;
    const para = [...paragraphs].find(p => parseInt(p.getAttribute('data-paragraph-index'), 10) === idx);
    if (!para) return;

    // Find and wrap the excerpt text within this paragraph.
    const excerpt = t.anchor.excerpt;
    const walker = document.createTreeWalker(para, NodeFilter.SHOW_TEXT);
    let fullText = '';
    const textNodes = [];
    while (walker.nextNode()) {
      textNodes.push({ node: walker.currentNode, start: fullText.length });
      fullText += walker.currentNode.textContent;
    }

    const matchStart = fullText.indexOf(excerpt);
    if (matchStart === -1) return;
    const matchEnd = matchStart + excerpt.length;

    // Find which text nodes to wrap.
    for (let i = textNodes.length - 1; i >= 0; i--) {
      const tn = textNodes[i];
      const nodeEnd = tn.start + tn.node.textContent.length;
      if (tn.start >= matchEnd || nodeEnd <= matchStart) continue;

      const relStart = Math.max(0, matchStart - tn.start);
      const relEnd = Math.min(tn.node.textContent.length, matchEnd - tn.start);

      const range = document.createRange();
      range.setStart(tn.node, relStart);
      range.setEnd(tn.node, relEnd);

      const mark = document.createElement('mark');
      mark.className = 'review-highlight' + (t.status !== 'open' ? ' resolved' : '');
      mark.dataset.threadId = t.id;
      mark.onclick = (e) => { e.stopPropagation(); showThread(t); };
      range.surroundContents(mark);
    }
  });
}

function setupTextSelection() {
  const center = document.getElementById('center');
  center.addEventListener('mouseup', (e) => {
    // Ignore clicks on recogito highlights (those open the thread panel).
    if (e.target.closest('.r6o-annotation')) return;
    // Delay to let browser finalize selection after recogito processes.
    setTimeout(() => {
      const sel = window.getSelection();
      if (!sel || sel.isCollapsed || !sel.toString().trim()) return;
      const range = sel.getRangeAt(0);
      // Walk up from the range start to find a paragraph element.
      let node = range.startContainer;
      if (node.nodeType === 3) node = node.parentElement;
      const paraEl = node.closest('[data-paragraph-index]');
      if (!paraEl) return;
      const excerpt = sel.toString().substring(0, 200);
      const paragraphIndex = parseInt(paraEl.getAttribute('data-paragraph-index'), 10);
      selectionAnchor = { paragraphIndex, excerpt };
      // Open our styled modal.
      document.getElementById('modal-excerpt').textContent = '"' + excerpt + '"';
      document.getElementById('modal-comment-input').value = '';
      document.getElementById('comment-modal').classList.add('open');
      setTimeout(() => document.getElementById('modal-comment-input').focus(), 50);
    }, 10);
  });
}

function closeCommentModal() {
  document.getElementById('comment-modal').classList.remove('open');
  selectionAnchor = null;
  window.getSelection().removeAllRanges();
}

function onModalOverlayClick(e) {
  if (e.target === document.getElementById('comment-modal')) closeCommentModal();
}

async function submitNewComment() {
  const input = document.getElementById('modal-comment-input');
  const body = input.value.trim();
  if (!body || !selectionAnchor || !currentDoc) return;
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
  document.getElementById('comment-modal').classList.remove('open');
  selectionAnchor = null;
  await loadDocuments();
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
  await loadDocuments();
  await selectDoc(currentDoc.path);
}

async function reopenThread() {
  if (!currentThread || !currentDoc) return;
  await api('/api/threads/' + currentThread.id + '/reopen?document=' + encodeURIComponent(currentDoc.path), { method: 'POST' });
  await refreshThread();
  await loadDocuments();
  await selectDoc(currentDoc.path);
}

async function deleteThread() {
  if (!currentThread || !currentDoc) return;
  if (!confirm('Delete this thread and all its comments?')) return;
  await api('/api/threads/' + currentThread.id + '/delete?document=' + encodeURIComponent(currentDoc.path), { method: 'POST' });
  closePanel();
  await loadDocuments();
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
