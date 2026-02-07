---
title: Spec Review GitHub Action
description: GitHub Action that creates Discussions from specs with status review and posts an initial agent review
status: review
author: heiko-braun
---

# Feature: spec-review-action

## Goal

Automate the creation of GitHub Discussions when a spec is pushed with `status: review`, and optionally post an initial AI-generated review comment. This removes manual overhead from the review workflow and ensures every spec marked for review gets a corresponding discussion in a consistent, discoverable format.

## Acceptance Criteria

- [x] When a push to `specs/*.md` contains a spec with `status: review` in its YAML front-matter, a GitHub Discussion is created in the "Spec Review" category
- [x] The discussion title follows the convention `[Spec Review] {slug}`, where `{slug}` is the spec filename without extension
- [x] The discussion body contains the full spec content (minus front-matter for cleaner readability)
- [x] If a discussion with the same title already exists, the action skips creation (idempotent)
- [x] An optional second job calls the Claude API with the spec content and posts an initial review comment on the discussion

## Approach

A two-job GitHub Action workflow triggered on `push` events filtered to `specs/*.md`. 

**Job 1 — Create Discussion:** Parses the changed spec files, reads YAML front-matter to check for `status: review`, and uses the GitHub GraphQL API (via `gh api graphql` or `actions/github-script`) to create a Discussion in the "Spec Review" category. Before creating, it queries existing discussions to check for a title match on `[Spec Review] {slug}` to ensure idempotency.

**Job 2 — Auto-Review (optional):** Depends on Job 1. Calls the Anthropic API with the spec content plus relevant repo context (e.g. README, related source files) as a system prompt, and posts the response as a comment on the newly created discussion. This job is gated by the presence of an `ANTHROPIC_API_KEY` secret — if not configured, it skips gracefully.

## Affected Modules

- `.github/workflows/spec-review.yml` — new workflow file
- Repository settings — "Spec Review" discussion category must exist (documented in README, not automated)
- No changes to existing `draft` CLI code, skills, or spec format

## Test Strategy

- **Unit test the front-matter parsing**: Push a spec with `status: review`, verify discussion is created. Push with `status: proposed`, verify no discussion is created.
- **Idempotency test**: Push the same spec twice with `status: review`, verify only one discussion exists.
- **Auto-review test**: With `ANTHROPIC_API_KEY` set, verify a review comment is posted. Without it, verify the job skips without failure.
- **Manual validation**: Run the workflow on the `draft` repo itself using a test spec.

## Out of Scope

- Writing `discussion_url` back to the spec file (resolved by naming convention instead)
- Updating the discussion body when a spec is refined (handled by the `/refine` skill, not this action)
- Syncing `status: approved` back to the spec when a discussion is marked answered (separate action/spec)
- Creating the "Spec Review" discussion category automatically (one-time manual setup)

## Open Questions

1. Should the discussion body include the YAML front-matter or strip it for readability?
2. Should the auto-review job receive additional repo context beyond the spec itself (e.g. existing source files listed in "Affected Modules")? If so, how much context is practical within API limits?
3. Should the action also handle spec file renames (slug changes) — e.g. close the old discussion and open a new one?
