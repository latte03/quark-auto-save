# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

Existing backend: Python Flask + APScheduler, with the incumbent UI in Jinja2 + Vue 2 + Bootstrap 4 (no build step).
Fork-local frontend replacement (user-confirmed 2026-10-28): React 19 + TypeScript + Vite + Tailwind CSS + shadcn/ui. The backend HTTP API stays the contract; the built frontend ships as static assets served by Flask.
This fork is maintained locally long-term; upstream `Cp0204/quark-auto-save` fixes are cherry-picked rather than merged wholesale.

## Users

A single operator: the owner of a self-hosted instance on a home NAS or server. Technical enough to run Docker and paste a Quark cookie, but not interested in reading source code during daily use. Mostly checks on a phone whether subscriptions saved new episodes; occasionally sits at a desktop to add tasks or repair dead share links. No multi-user roles, no public sign-up, high tolerance for information density.

## Product Purpose

Automates Quark Drive share-link saving: signs in daily for capacity rewards, re-saves updated files from subscribed shares, renames them with regex or magic variables, records dead share links, and hands new files to media libraries or downloaders through plugins. Success means new episodes land in the right folder under the right names, dead links are visible and repairable, and the operator rarely needs to open Quark's own web UI.

## Positioning

A self-hosted subscription, naming, and media-library handoff layer over Quark share links. It does not host content or bypass access; it wraps Quark's own APIs. The product-specific mechanisms are the naming pipeline (magic regex, magic variables, collision-aware sequence numbers) and the plugin hook pipeline (task_before / run / task_after).

## Operating Context

- Runs on a crontab schedule (APScheduler); most sessions are short status checks from a phone or desktop.
- Configuration is one JSON document: cookie, crontab, push_config, tasklist, magic_regex, plugins, source. The Flask server owns reads and writes.
- The first cookie performs saves; every configured cookie performs daily sign-in.
- A task carries shareurl, savepath, pattern/replace, update_subdir, startfid, enddate, runweek, addition (per-plugin options).
- Dead links are recorded as `shareurl_ban` on the task and shown in the UI.
- Plugins cover media refresh (Emby, Plex, fnOS), Alist refresh / STRM generation / cross-drive sync, Aria2 download, cloud unarchive, and SmartStrm webhooks.
- An AI skill and a Tampermonkey userscript use the same HTTP API with a token derived from the WebUI credentials.

## Capabilities and Constraints

Confirmed capabilities: daily sign-in rewards; multi-account sign-in with first-account saving; share validation and ban recording; target folder auto-creation; existing-file skipping with optional extension-insensitive comparison; regex and magic renaming; subdirectory updates in recursive or resave mode; per-task end date and weekday schedule; CloudSaver and PanSou resource search; SSE run log; cloud file delete/rename; plugin lifecycle; multi-channel notifications.

Constraints: the Flask backend stays; existing HTTP endpoints are the contract for this prototype; the frontend must build to static assets servable by Flask; mobile web is required; there is no multi-user permission model; `fnv_refresh_v2` is a Linux-only binary plugin.

Open decision: whether the prototype justifies new backend endpoints or a different config model. Deferred until the prototype is reviewed.

## Brand Commitments

Name: 夸克自动转存 / QAS. Existing icon at `img/icon.png`. No formal voice guide; copy is Chinese, terse, and functional. Preserve domain terminology: 转存, 追更, 魔法匹配, 魔法变量, 失效链接, 插件, 保存路径.

Visual stance (user-chosen 2026-10-28): the category-standard admin console, executed at high craft. Dark sidebar, top bar, summary metrics, dense task table, standard shadcn-style controls. Conventions are embraced without irony or novelty styling. The quality bar is Sonarr / Radarr: same-category self-hosted media automation tools with mature information density, state vocabulary, and mobile behavior.

## Evidence on Hand

Real references in the repo: `img/screenshot_webui-1.png`, `img/screenshot_webui-2.png`, `img/run_log.png`, `quark_config.json`, and the plugin list in `plugins/`. No testimonials, customer logos, or benchmarks exist; do not invent any.

## Product Principles

1. The next action is obvious at a glance: what ran, what is new, what is broken.
2. Show effective state, not raw configuration: active fields first, advanced settings folded away.
3. Never hide a global problem on one page; surface dead links and failed runs across the app.
4. Keep the naming pipeline explainable: show the final filename, not only the regex.
5. Preserve power features for the single expert operator; density over hand-holding.

## Accessibility & Inclusion

No product-specific standard was established. Apply WCAG AA contrast, keyboard-reachable controls, visible focus, and at least 44px touch targets on mobile.
