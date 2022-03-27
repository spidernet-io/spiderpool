# how automatically generate the verison changelog

### 1 all PR should be labeled with "pr/release/***" and could be merged

### 2 when push a tag, automatically create the changelog.

the changelog content include:

(1) New Features: it include all PR labeld with "pr/release/feature-new"

(2) Changed Features: it include all PR labeld with "pr/release/feature-changed"

(3) Fixes: it include all PR labeld with "pr/release/bug"

(4) all historical commit within this version

### 3 the changelog will attach to github RELEASE and submit to /changelogs of branch 'main'




# workflow for release

if a tag vXX.XX.XX is puhed , the following will auto trigger:

### 1 build the images with the pushed tag, to ghcr registry

### 2 generate the changelog by historical PR with "pr/release/*",
and submit a PR commit to branch 'main', with PR label "pr/release/robot_changelog".

### 3 build the chart package with the pushed tag, and submit a PR with new chart and index.yaml to branch 'webserver'

### 4 create a Github Realse attached with the chart and changelog

### finnally, by hand, need approve the chart PR with label "pr/release/robot_changelog" , and changelog PR with label "pr/release/robot_changelog"

