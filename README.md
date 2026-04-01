# gtc — GitLab CLI

Query MRs, branches and commits across multiple GitLab repositories from your terminal.

## Install

```bash
git clone https://github.com/lucasvavon/gtc
cd gtc
make install          # installs to $(go env GOPATH)/bin/gtc
```

## Setup

```bash
gtc init              # interactive wizard → writes ~/.gtc.yaml
# or
export GITLAB_TOKEN=glpat-xxxx
```

Config file precedence: `--output` flag > env vars > `~/.gtc.yaml` > defaults.

## Usage

```bash
# Merge Requests
gtc mrs list                                      # repos from config
gtc mrs list --repos mygroup/api,mygroup/web      # override repos
gtc mrs list --state merged --output json

# Branches
gtc branches list --repo mygroup/api
gtc branches list --repo mygroup/api --merged

# Commits
gtc commits log --repo mygroup/api
gtc commits log --repo mygroup/api --author alice --since 7d
```

## Config reference (`~/.gtc.yaml`)

| Key       | Env var        | Default               | Description                     |
|-----------|----------------|-----------------------|---------------------------------|
| `token`   | `GITLAB_TOKEN` | —                     | Personal Access Token (read_api)|
| `base_url`| `GTC_BASE_URL` | `https://gitlab.com`  | GitLab instance URL             |
| `output`  | `GTC_OUTPUT`   | `table`               | `table` \| `json` \| `yaml`     |
| `repos`   | —              | `[]`                  | Default repositories            |

## Build from source

```bash
make build            # → ./gtc
make test             # run tests
make lint             # golangci-lint
```
