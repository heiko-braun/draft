package reviewd

import (
	"fmt"
	"net/http"
)

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.GetAdminStats()
	if err != nil {
		s.logger.Error("admin stats query failed", "error", err.Error())
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, adminHTML, stats.Participants, stats.Repos, stats.Comments)
}

const adminHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>reviewd admin</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; color: #1a1a1a; padding: 2rem; }
  h1 { font-size: 1.5rem; margin-bottom: 1.5rem; }
  .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 1rem; max-width: 640px; }
  .card { background: #fff; border-radius: 8px; padding: 1.5rem; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
  .card .value { font-size: 2rem; font-weight: 700; }
  .card .label { font-size: 0.875rem; color: #666; margin-top: 0.25rem; }
</style>
</head>
<body>
<h1>reviewd admin</h1>
<div class="stats">
  <div class="card"><div class="value">%d</div><div class="label">Participants</div></div>
  <div class="card"><div class="value">%d</div><div class="label">Repos</div></div>
  <div class="card"><div class="value">%d</div><div class="label">Comments</div></div>
</div>
</body>
</html>`
