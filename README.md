# plr — Pulumi Local Runner

**plr** runs Pulumi programs from remote Git repositories on your local machine.

## The problem

You have Pulumi apps scattered across multiple repositories. Pulumi doesn't natively support cloning and running a program from a remote repo locally — [Pulumi Deployments](https://www.pulumi.com/docs/pulumi-cloud/deployments/) exists but requires Pulumi Cloud as the execution environment.

What you actually want is simple: point at a repo, pick a stack, run `pulumi up`. From anywhere. Without manually cloning, navigating to the right subdirectory, and remembering which branch goes with which environment.

## What plr does

plr is a thin orchestration layer that:

1. **Clones** (or fetches) Git repositories into a local cache
2. **Checks out** the right branch or tag for each stack
3. **Runs** Pulumi operations (`up`, `preview`, `destroy`, `refresh`) via the [Automation API](https://www.pulumi.com/docs/using-pulumi/automation-api/)
4. **Respects dependency order** between stacks

All configuration lives in a single file (`~/.config/plr/config.yaml`), so you can operate on any stack from any directory.

## Install

```bash
go install github.com/gfyrag/plr@latest
```

Enable zsh completion:

```bash
plr completion zsh > ~/.oh-my-zsh/completions/_plr
exec zsh
```

## Quick start

```bash
# Register an app
plr app add networking --repo git@github.com:org/infra-networking.git --path deployment/pulumi

# Add stacks (each can target a different branch/tag)
plr stack add networking dev --branch main --org myorg
plr stack add networking prod --ref v1.2.0 --org myorg

# Add another app with a dependency
plr app add kubernetes --repo git@github.com:org/infra-k8s.git
plr stack add kubernetes dev --branch main --org myorg --depends-on networking/dev

# Preview everything
plr preview

# Target a specific stack
plr up networking/dev

# Import an existing Pulumi stack config file
plr config import networking/dev /path/to/Pulumi.dev.yaml
```

## Commands

| Command | Description |
|---|---|
| `plr up [target...]` | Deploy stacks |
| `plr preview [target...]` | Preview changes (with detailed diffs) |
| `plr destroy [target...]` | Destroy stacks |
| `plr refresh [target...]` | Refresh state |
| `plr sync [target...]` | Clone/pull repos without running Pulumi |
| `plr app add <name> --repo <url>` | Register an app |
| `plr app list` | List apps |
| `plr app remove <name>` | Remove an app |
| `plr stack add <app> <name>` | Add a stack to an app |
| `plr stack list [app]` | List stacks |
| `plr stack remove <app/stack>` | Remove a stack |
| `plr config set <app/stack> <key> <value>` | Set a Pulumi config value |
| `plr config get <app/stack> <key>` | Get a config value |
| `plr config list <app/stack>` | List all config values |
| `plr config import <app/stack> <file>` | Import a `Pulumi.<stack>.yaml` file |
| `plr config rm <app/stack> <key>` | Remove a config value |

Targets can be `app` (all stacks) or `app/stack` (specific stack). No target means everything.

## Configuration

plr follows the XDG convention:

- **Config**: `~/.config/plr/config.yaml`
- **Repo cache**: `~/.cache/plr/repos/`

```yaml
apps:
  - name: networking
    repo: git@github.com:org/infra-networking.git
    path: deployment/pulumi    # subdirectory containing Pulumi.yaml
    stacks:
      - name: dev
        branch: main
        org: myorg             # Pulumi Cloud organization
      - name: prod
        ref: v1.2.0            # pin to a tag
        org: myorg
        dependsOn:
          - networking/dev     # deploy dev before prod

  - name: kubernetes
    repo: git@github.com:org/infra-k8s.git
    stacks:
      - name: dev
        branch: develop
        org: myorg
        dependsOn:
          - networking/dev
```

### Stack fields

| Field | Description |
|---|---|
| `name` | Stack name (used as Pulumi stack name) |
| `branch` | Git branch to track (mutually exclusive with `ref`) |
| `ref` | Git tag or commit to pin (mutually exclusive with `branch`) |
| `org` | Pulumi Cloud organization — enables fully qualified stack names (`org/project/stack`) |
| `project` | Override the Pulumi project name (auto-detected from `Pulumi.yaml` by default) |
| `dependsOn` | List of `app/stack` targets that must be deployed first |

## How it works

```
plr up networking/dev
```

1. Fetches `git@github.com:org/infra-networking.git` into `~/.cache/plr/repos/networking/`
2. Checks out `origin/main` (detached HEAD)
3. Resolves the fully qualified stack name: `myorg/infra-networking/dev`
4. Calls `auto.UpsertStackLocalSource()` pointing at `deployment/pulumi/`
5. Runs `stack.Up()` with output streamed to your terminal

When multiple stacks are targeted, plr topologically sorts them by `dependsOn` and runs them in order.

## Pulumi backend

plr uses whatever Pulumi backend you have configured (`pulumi login`). It doesn't manage backends — if you're logged into Pulumi Cloud, stacks are created there. If you use a local or S3 backend, that works too.

## License

MIT
