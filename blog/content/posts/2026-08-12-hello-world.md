---
title: "Hello, world"
date: 2026-08-12T23:00:00Z
slug: "hello-world"
tags: ["meta"]
image: "images/posts/hello-world.jpg"
imageCredit: "Photo: Rachel Michetti, CC0 (via scikit-image)"
draft: false
---

This blog is built with [Hugo](https://gohugo.io/) and lives in the same
repository as my other experiments. There's no theme dependency and no build
tooling to maintain — just markdown files in `blog/content/posts/` and a
handful of templates.

The fun part is the publishing workflow: I can post from my phone by opening
a Claude Code session and saying "new blog post about X." Claude writes the
markdown, commits it to `main`, and a GitHub Actions workflow builds the site
and deploys it here.

```bash
# the entire local workflow, when I'm at a keyboard
hugo new content posts/$(date +%F)-some-idea.md
$EDITOR blog/content/posts/*-some-idea.md
git add blog && git commit -m "Post: some idea" && git push
```

More to come.
