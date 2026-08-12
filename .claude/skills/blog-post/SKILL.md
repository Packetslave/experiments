---
name: blog-post
description: Create and publish a new blog post. Use when the user wants to post something to the blog, publish a note or thought, or says things like "new post", "blog this", or "post about X". Turns a rough idea into a markdown post and publishes it by committing to main, which triggers the deploy workflow.
---

# Publish a blog post

Posts live in `blog/content/posts/` and are published automatically: any push
to `main` that touches `blog/**` triggers `.github/workflows/blog.yml`, which
builds the Hugo site and deploys it to `gh-pages/blog/`. The live site is
https://packetslave.github.io/experiments/blog/

## Steps

1. **Write the post.** Turn the user's idea into a markdown post. Keep their
   voice — if they dictated rough phone notes, clean up the prose lightly but
   don't make it formal. Short posts are fine; this is a notes blog.

2. **Name the file** `blog/content/posts/YYYY-MM-DD-<slug>.md` where the slug
   is a short kebab-case version of the title (today's date, UTC).

3. **Front matter** (YAML):

   ```yaml
   ---
   title: "Human Readable Title"
   date: 2026-08-12T14:30:00Z   # current UTC time, RFC 3339
   slug: "human-readable-title" # clean slug; becomes the URL path
   tags: ["one", "two"]         # optional, lowercase; omit or [] if none fit
   draft: false                 # true = excluded from the built site
   ---
   ```

4. **Publish to `main`.** Do not build the site locally and do not touch
   `gh-pages` — CI handles both. How to get the file onto `main` depends on
   the session:
   - **Local session with push access to `main`:** commit the file on `main`
     with message `Post: <title>` and push.
   - **Remote/mobile session (working on a feature branch):** commit the
     post file directly to `main` with the `mcp__github__create_or_update_file`
     tool (owner `Packetslave`, repo `experiments`, branch `main`). This is
     the intended publish path for posts — a new post is content, not code,
     and does not need a PR. Anything beyond adding/editing a post file
     (templates, config, workflow) is a code change: use the normal
     branch-and-PR flow instead.

5. **Confirm.** Tell the user the post URL:
   `https://packetslave.github.io/experiments/blog/posts/<slug>/`
   and note that the deploy takes a minute or two. If asked to verify, check
   the "Deploy blog" run via the GitHub Actions MCP tools rather than polling
   the URL.

## Editing or removing a post

Same flow: edit or delete the file in `blog/content/posts/` on `main`
(`mcp__github__create_or_update_file` needs the current file SHA to update;
`mcp__github__delete_file` removes one). CI redeploys automatically.
