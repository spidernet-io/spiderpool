# workflow

## workflow for PR

a pull request may trigger following workflow

### action: check markdown file issue 

you could find the reported issue description <https://github.com/DavidAnson/markdownlint/blob/main/doc/Rules.md>

use following to find the issue on you local machine
```
make lint-markdown
```

use following the fix issue on you local machine
```
make fix-markdown
```

### action: check yaml file issue 

you could find the reported issue description <https://yamllint.readthedocs.io/en/stable/rules.html>

use following to find the issue on you local machine
```
make lint-yaml
```
