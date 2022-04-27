# changelog

## how automatically generate the version changelog

### 1 all PR should be labeled with "pr/release/***" and could be merged

### 2 when push a tag, automatically create the changelog

the changelog content include:

(1) New Features: it include all PR labeled with "pr/release/feature-new"

(2) Changed Features: it include all PR labeled with "pr/release/feature-changed"

(3) Fixes: it include all PR labeled with "pr/release/bug"

(4) all historical commit within this version

### 3 the changelog will be attached to github RELEASE and submit to /changelogs of branch 'github_pages'
