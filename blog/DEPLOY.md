# Deploying the blog to a personal server

The `deploy-server` job in `.github/workflows/site.yml` publishes the blog to
a server you control, alongside (or instead of) GitHub Pages. It is designed
so that a compromise of the repository or of GitHub Actions gives an attacker
as little as possible:

- The runner joins your **tailnet as an ephemeral node**, so the server never
  exposes SSH to the public internet.
- The deploy key is **restricted by a forced command** (`rrsync`), so even a
  leaked key can only write files inside the blog directory — no shell, no
  reads, no port forwarding, nowhere else on the filesystem.
- The SSH host key is **learned over the tailnet at deploy time**
  (`ssh-keyscan`). Tailscale's WireGuard layer already authenticates that
  the MagicDNS name resolves to your real node, so the scan cannot be
  intercepted from outside the tailnet — and there is no pinned key to go
  stale when the server's host key rotates.

The job stays disabled until the repository variable `SERVER_DEPLOY` is set
to `true`, so you can land this setup incrementally.

## 1. Server: deploy user and directory

As root on the server:

```bash
useradd --create-home --shell /bin/sh blogdeploy
mkdir -p /var/www/blog
chown blogdeploy:blogdeploy /var/www/blog
```

Point your web server (nginx, Caddy, ...) at `/var/www/blog`.

## 2. Server: restricted SSH key

Generate a dedicated keypair (on your own machine, not the server):

```bash
ssh-keygen -t ed25519 -f blog_deploy_key -N '' -C 'github-actions blog deploy'
```

Install the **public** key on the server with a forced command. `rrsync`
ships with rsync (Debian/Ubuntu: `/usr/bin/rrsync`; if your rsync is older
than 3.2.4, copy it from `/usr/share/doc/rsync/scripts/rrsync`).

`/home/blogdeploy/.ssh/authorized_keys` (one line):

```
from="100.64.0.0/10",command="/usr/bin/rrsync -wo /var/www/blog",restrict ssh-ed25519 AAAA...your-public-key... github-actions blog deploy
```

What each part does:

- `from="100.64.0.0/10"` — the key only works from Tailscale addresses
  (the CGNAT range Tailscale uses), even if SSH is somehow reachable
  another way.
- `command="/usr/bin/rrsync -wo /var/www/blog"` — every session runs rrsync
  and nothing else; `-wo` makes it write-only and jails it to that directory.
- `restrict` — disables port/agent/X11 forwarding and PTY allocation.

Optional extra hardening in `sshd_config`:

```
Match User blogdeploy
    PasswordAuthentication no
    AllowTcpForwarding no
    X11Forwarding no
```

If the server should not expose SSH publicly at all, firewall port 22 to the
`tailscale0` interface (for example `ufw allow in on tailscale0 to any port 22`
plus removing any public allow rule).

## 3. Tailscale: tag, ACL, and OAuth client

In the Tailscale admin console:

1. **Tag the server** (for example `tag:server`) and define `tag:ci` for CI
   nodes. In the tailnet policy file:

   ```jsonc
   "tagOwners": {
     "tag:ci":     ["autogroup:admin"],
     "tag:server": ["autogroup:admin"],
   },
   "acls": [
     // CI runners may reach the server on SSH only.
     { "action": "accept", "src": ["tag:ci"], "dst": ["tag:server:22"] },
     // ...your existing rules...
   ],
   ```

   With a default-deny policy, the ephemeral CI node can reach port 22 on the
   server and nothing else on your tailnet.

2. **Create an OAuth client** (Settings → OAuth clients) with the
   `auth_keys` scope, restricted to `tag:ci`. The GitHub Action uses it to
   mint ephemeral, pre-authorized nodes that disappear after each run.

## 4. GitHub: secrets and variables

Repository **secrets** (Settings → Secrets and variables → Actions):

| Secret | Value |
|---|---|
| `TS_OAUTH_CLIENT_ID` | OAuth client ID from step 3 |
| `TS_OAUTH_SECRET` | OAuth client secret from step 3 |
| `BLOG_DEPLOY_SSH_KEY` | Contents of the **private** key `blog_deploy_key` |

Repository **variables**:

| Variable | Value | Example |
|---|---|---|
| `SERVER_DEPLOY` | `true` to enable the job | `true` |
| `BLOG_BASE_URL` | Public URL of the blog on your server | `https://example.com/blog/` |
| `DEPLOY_SSH_USER` | Deploy user from step 1 | `blogdeploy` |
| `DEPLOY_SSH_HOST` | Server's MagicDNS name or Tailscale IP | `myserver.tailnet-name.ts.net` |

## 5. Enable and test

Set `SERVER_DEPLOY` to `true`, then trigger "Deploy site" manually
(`workflow_dispatch`) or push any change under `blog/`. The `deploy-server`
job should join the tailnet, rsync `blog/public/` to `/var/www/blog`, and the
ephemeral node should vanish from the admin console after the run.

The rsync target in the workflow is `:/` — that is not the server root; the
forced rrsync command maps it to `/var/www/blog`.

## Key rotation

Rotate by generating a new keypair, replacing the line in
`authorized_keys`, and updating the `BLOG_DEPLOY_SSH_KEY` secret. The OAuth
client can be revoked and re-created in the Tailscale console at any time;
runs in flight fail safely.

## Retiring the GitHub Pages copy

The Pages deploy (`deploy-pages` job) keeps working as a staging mirror. To
stop publishing the blog there, remove the Hugo step from `deploy-pages` and
the blog card from `docs/index.html`; the tools keep deploying as before.
