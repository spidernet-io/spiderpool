# changelog

***

how automatically generate the changelog

1. all PR should be labeled with "pr/release/***" and could be merged

2. when pushing a tag, automatically create the changelog

    the changelog content include:

    * New Features: it includes all PR labeled with "pr/release/feature-new"

    * Changed Features: it include all PR labeled with "pr/release/feature-changed"

    * Fixes: it include all PR labeled with "pr/release/bug"

    * all historical commit within this version

3. the changelog will be attached to github RELEASE and submit to /changelogs of branch 'github_pages'
