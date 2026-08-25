# sites/

Static sites published to seaside, one directory per domain.

```
sites/<domain>/
  public/        <- exactly what the webserver serves
  source/        <- inputs the public/ assets are derived from
  make-assets.py <- optional; regenerates public/ from source/
```

Only `public/` ships. The deploy job assembles a staging tree of every
site's `public/` and rsyncs that, so tooling and source material stay in
git without reaching the webserver.

## How a site gets published

The `deploy-parked` job in `.github/workflows/site.yml` joins the tailnet
as an ephemeral node and rsyncs the staging tree to seaside as
`parkeddeploy`. That account's SSH key carries a forced `rrsync -wo`
command jailed to `/var/www/parked`, so a leaked key can write parked
domains and nothing else — not incrementaldreams.com, not
packetslave.com, nowhere outside that directory.

`rsync --delete` runs against the whole tree, so removing a directory here
removes the site from the server on the next deploy.

Ansible (`src/homelab/ansible/webserver.yaml`) owns everything around the
content: the `parkeddeploy` user and its jailed `authorized_keys` line,
`/var/www/parked`, the nginx vhost, the certificate, the firewall. It does
not write page content. Content comes from this directory, via a commit.

Full background on the transport — the tailnet ACLs, the rrsync jail, key
rotation, troubleshooting — is in `blog/DEPLOY.md`; the parked sites reuse
that mechanism with their own deploy user.

## Adding a site

1. `mkdir -p sites/<domain>/public` and put the page in it.
2. Add the domain to `sites:` in homelab's `webserver.yaml` with
   `root: /var/www/parked/<domain>`, and run the playbook (twice for a new
   domain — see that repo's CLAUDE.md).
3. Commit to `main`. The workflow does the rest.
