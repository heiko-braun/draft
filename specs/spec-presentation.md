# Feature: Spec Presentation

## Goal

Add a `draft present` command that serves an index of all specs and individual spec pages as styled HTML with table of contents from localhost, enabling engineers to browse and review specs in a pleasant browser-based format.

## Acceptance Criteria

- [ ] `draft present` command serves specs from localhost (no arguments needed)
- [ ] Index page lists all specs in the `specs/` directory with links
- [ ] Individual spec pages include an auto-generated table of contents from markdown headings
- [ ] Markdown is rendered using goldmark with pleasant styling
- [ ] Command opens the browser automatically to the index page
- [ ] Server can be stopped with Ctrl+C

## Approach

Add a new `present` subcommand in `internal/cli/present.go` following the existing command pattern. Create an HTTP server with two routes: `/` for the index listing all specs, and `/{spec-name}` for individual rendered specs. Use goldmark to parse and render markdown to HTML, generate TOCs from headings, and wrap in basic HTML with embedded CSS styling. Start server on localhost (default port 3000, configurable via flag) and open browser to index.

## Out of Scope

- Auto-reload when file changes
- Custom themes/styling beyond basic markdown CSS
- Exporting to PDF or other formats
- Search/filtering in the index

## Notes

File path resolution follows existing pattern: `specs/{spec-name}.md`
