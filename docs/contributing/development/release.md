# workflow for release

if a tag vXX.XX.XX is pushed , the following will auto trigger:

## 1 build the images with the pushed tag, push to ghcr registry

## 2 generate the changelog by historical PR with "pr/release/*"

submit the changelog file to branch 'github_pages', with PR label "pr/release/robot_update_githubpage".

## 3 build the chart package with the pushed tag, and submit a PR to branch 'github_pages'

it commits the new chart package to '/charts' and update /index.yaml on the branch 'github_pages'

## 4 submit '/docs' of branch 'main' to '/docs' of branch 'github_pages'

## 5 create a GitHub Release attached with the chart and changelog

## Finally, by hand, need approve the chart PR with label "pr/release/robot_update_githubpage" , and changelog PR with label "pr/release/robot_update_githubpage"
