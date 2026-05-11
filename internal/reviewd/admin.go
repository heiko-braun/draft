package reviewd

import (
	"encoding/json"
	"net/http"
)

type adminData struct {
	Stats        *AdminStats        `json:"stats"`
	Participants []AdminParticipant `json:"participants"`
	Repos        []AdminRepo        `json:"repos"`
}

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.GetAdminStats()
	if err != nil {
		s.logger.Error("admin stats query failed", "error", err.Error())
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	participants, err := s.store.ListAllParticipants()
	if err != nil {
		s.logger.Error("admin list participants failed", "error", err.Error())
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	repos, err := s.store.ListAllRepos()
	if err != nil {
		s.logger.Error("admin list repos failed", "error", err.Error())
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := adminData{
		Stats:        stats,
		Participants: participants,
		Repos:        repos,
	}

	dataJSON, _ := json.Marshal(data)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(adminHTMLPrefix))
	w.Write(dataJSON)
	w.Write([]byte(adminHTMLSuffix))
}

const adminHTMLPrefix = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>reviewd admin</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; color: #1a1a1a; padding: 2rem; }
  h1 { font-size: 1.5rem; margin-bottom: 1.5rem; }
  .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 1rem; max-width: 480px; }
  .card { background: #fff; border-radius: 8px; padding: 1.5rem; box-shadow: 0 1px 3px rgba(0,0,0,0.1); cursor: pointer; transition: box-shadow 0.15s, transform 0.15s; user-select: none; }
  .card:hover { box-shadow: 0 3px 8px rgba(0,0,0,0.15); transform: translateY(-1px); }
  .card.active { box-shadow: 0 0 0 2px #1a1a1a; }
  .card .value { font-size: 2rem; font-weight: 700; }
  .card .label { font-size: 0.875rem; color: #666; margin-top: 0.25rem; }
  #detail { margin-top: 2rem; max-width: 720px; }
  table { width: 100%; border-collapse: collapse; background: #fff; border-radius: 8px; overflow: hidden; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
  th { text-align: left; padding: 0.75rem 1rem; background: #fafafa; font-size: 0.75rem; text-transform: uppercase; color: #666; border-bottom: 1px solid #eee; }
  td { padding: 0.75rem 1rem; border-bottom: 1px solid #f0f0f0; font-size: 0.875rem; }
  tr:last-child td { border-bottom: none; }
  .empty { color: #999; padding: 2rem; text-align: center; }
  .num { text-align: right; }
</style>
</head>
<body>
<h1>reviewd admin</h1>
<div class="stats">
  <div class="card" data-panel="participants"><div class="value" id="count-participants"></div><div class="label">Participants</div></div>
  <div class="card" data-panel="repos"><div class="value" id="count-repos"></div><div class="label">Repos</div></div>
</div>
<div id="detail"></div>
<script>
const DATA = `

const adminHTMLSuffix = `;
document.getElementById('count-participants').textContent = DATA.stats.participants;
document.getElementById('count-repos').textContent = DATA.stats.repos;

let activePanel = null;

function formatDate(s) {
  if (!s) return '';
  const d = new Date(s);
  return d.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' });
}

function esc(s) {
  const el = document.createElement('span');
  el.textContent = s;
  return el.innerHTML;
}

function renderParticipants() {
  const rows = (DATA.participants || []).map(p =>
    '<tr><td>' + esc(p.name) + '</td><td>' + esc(p.email) + '</td><td>' + formatDate(p.created_at) + '</td></tr>'
  ).join('');
  if (!rows) return '<div class="empty">No participants</div>';
  return '<table><thead><tr><th>Name</th><th>Email</th><th>Joined</th></tr></thead><tbody>' + rows + '</tbody></table>';
}

function renderRepos() {
  const rows = (DATA.repos || []).map(r =>
    '<tr><td>' + esc(r.github_owner) + '/' + esc(r.github_repo) + '</td><td class="num">' + r.comment_count + '</td><td>' + formatDate(r.created_at) + '</td></tr>'
  ).join('');
  if (!rows) return '<div class="empty">No repos</div>';
  return '<table><thead><tr><th>Repository</th><th class="num">Comments</th><th>Created</th></tr></thead><tbody>' + rows + '</tbody></table>';
}

const renderers = { participants: renderParticipants, repos: renderRepos };

document.querySelectorAll('.card').forEach(card => {
  card.addEventListener('click', () => {
    const panel = card.dataset.panel;
    document.querySelectorAll('.card').forEach(c => c.classList.remove('active'));
    if (activePanel === panel) {
      activePanel = null;
      document.getElementById('detail').innerHTML = '';
    } else {
      activePanel = panel;
      card.classList.add('active');
      document.getElementById('detail').innerHTML = renderers[panel]();
    }
  });
});
</script>
</body>
</html>`
