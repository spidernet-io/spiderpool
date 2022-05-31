# workflow for release

If a tag is pushed , the following steps will run:

1. build the images with the pushed tag, and push to ghcr registry

2. generate the changelog by historical PR labeled as "pr/release/*"

    submit the changelog file to branch 'github_pages', with PR labeled as "pr/release/robot_update_githubpage".

3. build the chart package with the pushed tag, and submit a PR to branch 'github_pages'

4. submit '/docs' of branch 'main' to '/docs' of branch 'github_pages'

5. create a GitHub Release attached with the chart package and changelog

6. Finally, by hand, need approve the chart PR labeled as "pr/release/robot_update_githubpage" , and changelog PR labeled as "pr/release/robot_update_githubpage"
