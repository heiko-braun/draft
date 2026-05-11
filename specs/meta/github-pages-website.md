---
title: GitHub Pages Website
description: Static website with installation, usage instructions, and spec workflow showcase
status: implemented
author: Heiko Braun <ike.braun@googlemail.com>
---

# Feature: GitHub Pages Website

## Goal

Create a GitHub Pages website that helps external users discover draft, understand how to install it, and learn the core spec workflow (create → verify → implement). The site serves as the primary entry point for new users.

## Acceptance Criteria

- [x] Static HTML/CSS site deployed to GitHub Pages from this repository
- [x] Installation instructions for macOS, Linux, and Windows
- [x] Basic usage section covering `draft init`, `draft spec`, `draft present`
- [x] Spec workflow showcase demonstrating create → verify → implement cycle
- [x] Custom styling using Geist font and modern design system (similar to `draft present` styling)

## Approach

Create static HTML/CSS files in a `/docs` directory (GitHub Pages standard location). Use modern blue color palette with Tailwind CSS CDN and Lucide icons. Structure as a single-page site with sections for installation, usage, and workflow. Configure GitHub Pages to serve from `/docs` folder on main branch.

## Affected Modules

- `/docs/index.html` — Main website file (new)
- `install.sh` — Installation script for ~/.local/bin (new)
- `.github/` — May need GitHub Pages configuration or deployment workflow (new, if automated deployment needed)

New module boundary: Website lives entirely in `/docs/`, independent of Go codebase.

## Test Strategy

Manual verification:
- [ ] Site loads correctly at `https://{username}.github.io/{repo}/`
- [ ] All links work
- [ ] Installation commands are accurate for each platform
- [ ] Responsive design works on mobile and desktop
- [ ] Page renders correctly in major browsers (Chrome, Firefox, Safari)

No automated tests required for static content.

## Out of Scope

- Interactive demos or playground
- Versioned documentation
- Search functionality
- Blog or changelog section
- Community/contribution guidelines beyond basic info
- Auto-generated API/CLI reference
- Multiple pages (keep as single-page site)

## Notes

GitHub Pages can be enabled in repository settings under "Pages" → Source: "Deploy from a branch" → Branch: "main" → Folder: "/docs".
