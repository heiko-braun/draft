package cli

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

func newPresentCmd() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "present",
		Short: "Serve specs as HTML with table of contents",
		Long: `Present serves all specs in the specs/ directory as styled HTML pages with table of contents.
The index page lists all available specs, and each spec is rendered with an auto-generated TOC.

Example:
  draft present
  draft present --port 8080`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return serveSpecs(port)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 3000, "Port to serve the presentation on")

	return cmd
}

type specInfo struct {
	Name     string
	Path     string
	Modified string
}

func serveSpecs(port int) error {
	// Check if specs directory exists
	if _, err := os.Stat("specs"); os.IsNotExist(err) {
		return fmt.Errorf("specs directory not found in current directory")
	}

	addr := fmt.Sprintf("localhost:%d", port)
	url := fmt.Sprintf("http://%s", addr)

	fmt.Printf("Starting spec presentation server...\n")
	fmt.Printf("Server running at %s\n", url)
	fmt.Println("Press Ctrl+C to stop")

	// Open browser after a short delay
	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser(url)
	}()

	// Set up HTTP handlers
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/spec/", handleSpec)

	// Start server
	if err := http.ListenAndServe(addr, nil); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	specs, err := getSpecList()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading specs: %v", err), http.StatusInternalServerError)
		return
	}

	data := struct {
		RepoURL  string
		RepoName string
		Specs    []specInfo
	}{
		RepoURL:  getRepoURL(),
		RepoName: getRepoName(),
		Specs:    specs,
	}

	tmpl := template.Must(template.New("index").Parse(indexTemplate))
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Error rendering index: %v", err)
	}
}

func handleSpec(w http.ResponseWriter, r *http.Request) {
	specName := strings.TrimPrefix(r.URL.Path, "/spec/")
	if specName == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	specPath := filepath.Join("specs", specName+".md")
	content, err := os.ReadFile(specPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	htmlContent, toc, err := renderMarkdown(content)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error rendering markdown: %v", err), http.StatusInternalServerError)
		return
	}

	data := struct {
		Title   string
		TOC     template.HTML
		Content template.HTML
	}{
		Title:   specName,
		TOC:     template.HTML(toc),
		Content: template.HTML(htmlContent),
	}

	tmpl := template.Must(template.New("spec").Parse(specTemplate))
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Error rendering spec: %v", err)
	}
}

func getRepoURL() string {
	// Try to get remote URL from git
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	output, err := cmd.Output()
	if err == nil {
		url := strings.TrimSpace(string(output))
		// Convert SSH URL to HTTPS
		if strings.HasPrefix(url, "git@github.com:") {
			url = strings.Replace(url, "git@github.com:", "https://github.com/", 1)
			url = strings.TrimSuffix(url, ".git")
		}
		return url
	}
	return ""
}

func getRepoName() string {
	// Try to get repo name from git
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err == nil {
		path := strings.TrimSpace(string(output))
		return filepath.Base(path)
	}

	// Fallback to current directory name
	cwd, err := os.Getwd()
	if err == nil {
		return filepath.Base(cwd)
	}

	return "Draft"
}

func getSpecList() ([]specInfo, error) {
	var specs []specInfo

	err := filepath.WalkDir("specs", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".md" && d.Name() != "TEMPLATE.md" {
			name := strings.TrimSuffix(d.Name(), ".md")

			// Get file modification time
			info, err := d.Info()
			if err != nil {
				return err
			}
			modTime := info.ModTime().Format("2006-01-02")

			specs = append(specs, specInfo{
				Name:     name,
				Path:     "/spec/" + name,
				Modified: modTime,
			})
		}
		return nil
	})

	return specs, err
}

func renderMarkdown(source []byte) (string, string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)

	// Parse markdown to AST
	reader := text.NewReader(source)
	doc := md.Parser().Parse(reader)

	// Generate TOC
	toc := generateTOC(doc, source)

	// Render to HTML
	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, source, doc); err != nil {
		return "", "", err
	}

	return buf.String(), toc, nil
}

func generateTOC(doc ast.Node, source []byte) string {
	var toc strings.Builder
	toc.WriteString("<ul class=\"toc\">\n")

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		if heading, ok := n.(*ast.Heading); ok {
			if heading.Level >= 1 && heading.Level <= 3 {
				id := ""
				if attr, found := n.Attribute([]byte("id")); found && attr != nil {
					id = string(attr.([]byte))
				}

				text := extractText(heading, source)
				indent := strings.Repeat("  ", heading.Level-1)

				if id != "" {
					toc.WriteString(fmt.Sprintf("%s<li class=\"toc-level-%d\"><a href=\"#%s\">%s</a></li>\n",
						indent, heading.Level, id, text))
				}
			}
		}

		return ast.WalkContinue, nil
	})

	toc.WriteString("</ul>")
	return toc.String()
}

func extractText(n ast.Node, source []byte) string {
	var text strings.Builder
	ast.Walk(n, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if t, ok := node.(*ast.Text); ok {
			text.Write(t.Segment.Value(source))
		}
		return ast.WalkContinue, nil
	})
	return text.String()
}

func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	if err != nil {
		log.Printf("Failed to open browser: %v", err)
		log.Printf("Please open %s manually", url)
	}
}

const indexTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.RepoName}} Specs</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Geist:wght@400;500;600&display=swap" rel="stylesheet">
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: 'Geist', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            color: hsl(240 10% 3.9%);
            background: hsl(0 0% 100%);
        }
        .container {
            max-width: 800px;
            margin: 0 auto;
            padding: 3rem 2rem;
        }
        .title-container {
            margin-bottom: 2rem;
        }
        h1 {
            font-size: 1.5rem;
            font-weight: 600;
            margin-bottom: 0.25rem;
            color: hsl(240 10% 3.9%);
        }
        h1 a {
            color: hsl(240 10% 3.9%);
            text-decoration: none;
        }
        h1 a:hover {
            text-decoration: underline;
        }
        .subtitle {
            font-size: 1rem;
            font-weight: 400;
            color: hsl(240 3.8% 46.1%);
        }
        table {
            width: 100%;
            border-collapse: collapse;
            border: 1px solid hsl(240 5.9% 90%);
            border-radius: 0.5rem;
            overflow: hidden;
        }
        thead {
            background: hsl(240 4.8% 95.9%);
            border-bottom: 1px solid hsl(240 5.9% 90%);
        }
        th {
            text-align: left;
            padding: 0.75rem 1rem;
            font-weight: 500;
            font-size: 0.875rem;
            color: hsl(240 10% 3.9%);
        }
        tbody tr {
            border-bottom: 1px solid hsl(240 5.9% 90%);
        }
        tbody tr:last-child {
            border-bottom: none;
        }
        tbody tr:hover {
            background: hsl(240 4.8% 95.9%);
        }
        td {
            padding: 0.75rem 1rem;
            font-size: 0.95rem;
        }
        td a {
            color: hsl(240 10% 3.9%);
            text-decoration: none;
        }
        td a:hover {
            text-decoration: underline;
        }
        .date {
            color: hsl(240 3.8% 46.1%);
            font-size: 0.875rem;
        }
        .empty {
            color: hsl(240 3.8% 46.1%);
            padding: 2rem;
            text-align: center;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="title-container">
            {{if .RepoURL}}
            <h1><a href="{{.RepoURL}}" target="_blank">{{.RepoURL}}</a></h1>
            {{else}}
            <h1>{{.RepoName}}</h1>
            {{end}}
            <div class="subtitle">Specifications</div>
        </div>
        {{if .Specs}}
        <table>
            <thead>
                <tr>
                    <th>Name</th>
                    <th>Last Modified</th>
                </tr>
            </thead>
            <tbody>
            {{range .Specs}}
                <tr>
                    <td><a href="{{.Path}}">{{.Name}}</a></td>
                    <td class="date">{{.Modified}}</td>
                </tr>
            {{end}}
            </tbody>
        </table>
        {{else}}
        <p class="empty">No specs found in specs/ directory</p>
        {{end}}
    </div>
</body>
</html>
`

const specTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} - Draft Spec</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Geist:wght@400;500;600&display=swap" rel="stylesheet">
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: 'Geist', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            color: hsl(240 10% 3.9%);
            background: hsl(0 0% 100%);
        }
        .header {
            border-bottom: 1px solid hsl(240 5.9% 90%);
            padding: 1rem 2rem;
            position: sticky;
            top: 0;
            z-index: 100;
            background: hsl(0 0% 100%);
        }
        .header a {
            color: hsl(240 10% 3.9%);
            text-decoration: none;
            font-size: 0.875rem;
            display: inline-flex;
            align-items: center;
            gap: 0.25rem;
        }
        .header a:hover {
            text-decoration: underline;
        }
        .container {
            display: grid;
            grid-template-columns: 1fr 250px;
            gap: 2rem;
            max-width: 1400px;
            margin: 0 auto;
            padding: 2rem;
        }
        .sidebar {
            position: sticky;
            top: 80px;
            height: fit-content;
            border: 1px solid hsl(240 5.9% 90%);
            border-radius: 0.5rem;
            padding: 1.5rem;
        }
        .sidebar h3 {
            font-size: 0.875rem;
            font-weight: 500;
            margin-bottom: 1rem;
            color: hsl(240 10% 3.9%);
        }
        .toc {
            list-style: none;
        }
        .toc li {
            margin: 0.5rem 0;
        }
        .toc a {
            color: hsl(240 3.8% 46.1%);
            text-decoration: none;
            font-size: 0.875rem;
            transition: color 0.2s;
        }
        .toc a:hover {
            color: hsl(240 10% 3.9%);
        }
        .toc-level-2 {
            margin-left: 1rem;
        }
        .toc-level-3 {
            margin-left: 2rem;
        }
        .content {
            max-width: 900px;
        }
        .content h1 {
            font-size: 2.25rem;
            font-weight: 600;
            margin: 0 0 2rem 0;
            color: hsl(240 10% 3.9%);
        }
        .content h2 {
            font-size: 1.5rem;
            font-weight: 600;
            margin: 2rem 0 1rem 0;
            padding-bottom: 0.5rem;
            border-bottom: 1px solid hsl(240 5.9% 90%);
            color: hsl(240 10% 3.9%);
        }
        .content h3 {
            font-size: 1.25rem;
            font-weight: 600;
            margin: 1.5rem 0 0.75rem 0;
            color: hsl(240 10% 3.9%);
        }
        .content p {
            margin: 1rem 0;
            color: hsl(240 3.8% 46.1%);
        }
        .content ul, .content ol {
            margin: 1rem 0;
            padding-left: 1.5rem;
        }
        .content li {
            margin: 0.5rem 0;
            color: hsl(240 3.8% 46.1%);
        }
        .content code {
            background: hsl(240 4.8% 95.9%);
            padding: 0.2rem 0.4rem;
            border-radius: 0.25rem;
            font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, "Liberation Mono", monospace;
            font-size: 0.875em;
            color: hsl(240 10% 3.9%);
        }
        .content pre {
            background: hsl(240 4.8% 95.9%);
            padding: 1rem;
            border-radius: 0.5rem;
            overflow-x: auto;
            margin: 1rem 0;
            border: 1px solid hsl(240 5.9% 90%);
        }
        .content pre code {
            background: none;
            padding: 0;
        }
        .content blockquote {
            border-left: 3px solid hsl(240 5.9% 90%);
            padding-left: 1rem;
            margin: 1rem 0;
            color: hsl(240 3.8% 46.1%);
        }
        .content table {
            border-collapse: collapse;
            width: 100%;
            margin: 1rem 0;
            border: 1px solid hsl(240 5.9% 90%);
            border-radius: 0.5rem;
            overflow: hidden;
        }
        .content th, .content td {
            border-bottom: 1px solid hsl(240 5.9% 90%);
            padding: 0.75rem;
            text-align: left;
        }
        .content th {
            background: hsl(240 4.8% 95.9%);
            font-weight: 500;
            color: hsl(240 10% 3.9%);
        }
        .content tr:last-child td {
            border-bottom: none;
        }
        .content input[type="checkbox"] {
            margin-right: 0.5rem;
            accent-color: hsl(240 10% 3.9%);
        }
        @media (max-width: 768px) {
            .container {
                grid-template-columns: 1fr;
            }
            .sidebar {
                position: static;
            }
        }
    </style>
</head>
<body>
    <div class="header">
        <a href="/">← Back to all specs</a>
    </div>
    <div class="container">
        <main class="content">
            {{.Content}}
        </main>
        <aside class="sidebar">
            <h3>Table of Contents</h3>
            {{.TOC}}
        </aside>
    </div>
</body>
</html>
`
