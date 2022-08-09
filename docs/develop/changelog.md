# Changelog

How to automatically generate changelogs:

1. All PRs should be labeled with "pr/release/***" and can be merged.

2. When you add the label, the changelog will be created automatically.

    The changelog contents include:

    * New Features: it includes all PRs labeled with "pr/release/feature-new"

    * Changed Features: it includes all PRs labeled with "pr/release/feature-changed"

    * Fixes: it includes all PRs labeled with "pr/release/bug"

    * All historical commits within this version

3. The changelog will be attached to Github RELEASE and submitted to /changelogs of branch 'github_pages'.
