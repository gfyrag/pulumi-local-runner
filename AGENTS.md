# PLR Development Notes

## Architecture

### Config files are the source of truth

The plr config files (`~/.config/plr/apps/<app>/<stack>.yaml`) are the
authoritative source for stack configuration. The `Pulumi.<stack>.yaml`
files written to workdirs are **ephemeral** — they are created before a
Pulumi operation and cleaned up after. They must never be treated as
the source of truth.

### Secrets

Secrets are stored **in plaintext** in the plr config files (stack files
and bases). They are identified by the `secret: true` flag in the
`x-plr-config` schema of each project's `Pulumi.yaml`.

At runtime, plr extracts secret values from the merged config YAML and
passes them to Pulumi via `SetConfig(secret=true)` through the
Automation API. Pulumi then encrypts them with its own salt/passphrase.

**Never** store Pulumi-encrypted secrets (`{secure: v1:...}`) in plr
config files — they are tied to a specific salt and become unreadable
if the salt changes or the ephemeral config file is recreated.

### Pulumi state

Pulumi maintains its own state in `~/.pulumi/stacks/`. The salt for
secret encryption is stored there, not in plr config files.
