---
name: blog-post
description: Create and publish a new blog post or link post. Use when the user wants to post something to the blog, publish a note, thought, or interesting link, or says things like "new post", "blog this", "post about X", or "share this link". Owns all posting guidelines - writing style, front matter, hero images, link posts, and the publish flow (commit to main; CI deploys).
---

# Publish a blog post

The pipeline: a markdown file lands on `main` → the "Deploy site" workflow
builds the Hugo site and deploys it to GitHub Pages (and to the personal
server when that deploy is enabled — see `blog/DEPLOY.md`). Never touch the
`gh-pages` branch and never build or deploy by hand.

Live site: https://packetslave.github.io/experiments/blog/

## 1. Write the post

Turn the user's idea into a markdown post. Keep their voice and don't make
it formal — short posts are fine; this is a notes blog — but apply the
writing style rules below.

Blog content follows **ASD-STE100 Simplified Technical English** and the
**GOV.UK style guides** ([style guide](https://www.gov.uk/guidance/style-guide)
and [content design guidance](https://www.gov.uk/guidance/content-design/writing-for-gov-uk)).
Refer to them when unsure; the rules that matter most here:

From ASD-STE100:
- Keep sentences short — about 20 words for descriptive text, 25 at most.
- Use the active voice; make the doer of the action the subject.
- Give one instruction per sentence.
- Use one term for one thing throughout a post; do not vary words for style.
- Prefer simple, approved words (`start`, not `commence`; `use`, not `utilize`).

From GOV.UK:
- Use plain English; front-load the key information.
- Keep paragraphs short (ideally one idea, no more than 5 sentences).
- Use sentence case for titles and headings.
- Avoid Latin abbreviations (`e.g.`, `i.e.`, `etc.`) — write them out.
- Write numbers as digits (`5`, not `five`), except at the start of a sentence.
- Make link text describe the destination; never "click here".

The list page shows the first ~70 words as the excerpt, so front-load the
key point — the first sentences must stand alone.

## 2. Name the file

`blog/content/posts/YYYY-MM-DD-<slug>.md` — today's date (UTC), slug is a
short kebab-case version of the title.

## 3. Front matter

```yaml
---
title: "Human readable title"          # sentence case
date: 2026-08-12T14:30:00Z             # current UTC time, RFC 3339
slug: "human-readable-title"           # clean slug; becomes the URL path
tags: ["one", "two"]                   # optional, lowercase; [] if none fit
image: "images/posts/<slug>.jpg"       # optional hero image (see below)
imageCredit: "Photo: <who>, <license>" # required whenever image is set
caption: ""                            # optional subtitle in the title bar
draft: false                           # true = excluded from the built site
---
```

## 4. Hero image (optional, but preferred)

The GoodSpace theme shows a black-and-white hero above each post.
Format: grayscale JPEG, 1280x480 (8:3). License: CC0, public domain, or
"no known copyright restrictions" only, with source and license recorded
in `imageCredit`. Never use the GoodSpace demo photos (separately-licensed
stock) or images with unknown provenance.

Find one in this order:

1. **Image library first.** Read
   `blog/static/images/library/manifest.json` and look for an entry whose
   `keywords` fit the post's subject. If one fits, set front matter
   `image` to the entry's `file` and build `imageCredit` from its
   `creator` and `license` (`Photo: <creator>, <license> (via <source>)`).
   This is a text-only edit — it works from any session, and reusing an
   image across posts is fine.

2. **Fetch workflow.** If the library has no fit, publish the post first
   (step 5), then trigger the "Fetch hero image" workflow
   (`fetch-hero.yml`) with the GitHub Actions MCP tools
   (`actions_run_trigger`, ref `main`) and inputs:
   - `query`: 2-4 literal, visual search terms — concrete objects
     photograph well ("typewriter keys", "patch cables"), abstract
     concepts ("recovery", "simplicity") do not.
   - `slug`: the post's slug.

   The workflow searches Openverse hard-filtered to CC0/public-domain,
   processes the best match (`scripts/fetch_hero.py` is the reference
   implementation), saves it into the library, patches the post's front
   matter, and pushes — the site then redeploys itself. Watch the run via
   the Actions tools; if it finds nothing usable, retry once with
   different terms, then fall through.

3. **No image.** Publish without one — a post is publishable without an
   image, and the workflow can attach one later at any time.

**Stocking the library:** run the workflow with `query` set and `slug`
blank to add an image without touching any post. Last-resort offline
source for sessions with a git checkout: the scikit-image sample dataset
via PyPI (licenses documented per image in `skimage/data/_fetchers.py`).

## 5. Publish to `main`

How to get the file onto `main` depends on the session:

- **Session with a git checkout and push access**: commit on `main` with
  message `Post: <title>` and push.
- **Remote/mobile session without a checkout**: commit the post file
  directly to `main` with the `mcp__github__create_or_update_file` tool
  (owner `Packetslave`, repo `experiments`, branch `main`). This is the
  intended publish path for posts — a new post is content, not code, and
  does not need a PR. Anything beyond adding/editing a post file
  (templates, theme, config, workflow) is a code change: use the normal
  branch-and-PR flow instead.

## 6. Confirm

Tell the user the post URL:
`https://packetslave.github.io/experiments/blog/posts/<slug>/`
and note that the deploy takes a minute or two. If asked to verify, check
the "Deploy site" run via the GitHub Actions MCP tools rather than polling
the URL.

## Link posts

For sharing a URL — somewhere between a tweet and a link blog. Links get a
distinctive card style and appear interleaved with full posts in the home
page feed, at `/links/`, and in the "Latest Links" sidebar widget — but
never in the Recent Posts widget.

- **File**: `blog/content/links/YYYY-MM-DD-<slug>.md` — slug from the
  title, or from the site's domain name if there is no title.
- **Front matter**: `link` (the URL, required), `title` (optional — it
  becomes the link text; the URL itself is shown when absent), `date`
  (RFC 3339 UTC), `draft`. No tags, no hero image, no `slug` needed.
- **Body**: optional commentary, a sentence or two in the user's voice.
  Empty is fine — the link can stand alone.
- **Publish**: same flow as a post (step 5); confirm with the
  `/links/` page URL. Skip the hero image step entirely.

```yaml
---
title: "Openverse"                # optional
date: 2026-08-14T19:23:00Z
link: "https://openverse.org/"
draft: false
---

One or two sentences of commentary, if any.
```

## Editing or removing a post

Same flow: edit or delete the file in `blog/content/posts/` on `main`
(`mcp__github__create_or_update_file` needs the current file SHA to update;
`mcp__github__delete_file` removes one). CI redeploys automatically.
