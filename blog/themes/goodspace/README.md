# GoodSpace — Hugo port

A Hugo port of the blog templates (blog with right sidebar, single post) from
the [GoodSpace WordPress theme](https://themeforest.net/item/good-space-responsive-minimal-wp-theme/2278615)
by GoodLayers, made for this site under a purchased ThemeForest license.

Provenance:

- **Templates** are original Hugo templates modeled on the theme's PHP/HTML
  structure, which GoodLayers licenses under the GPL.
- **CSS** is written from scratch for this port. It reproduces the theme's
  design tokens (colors, type scale, 960px Skeleton grid) in a fraction of
  the original size. Skeleton itself is MIT (Dave Gamache).
- The tiled background texture is a small pattern tile from the purchased
  theme package, embedded as a data URI.

Not intended for redistribution as a standalone theme.

## Front matter extras

- `image`: path or URL to a featured image, shown 640px wide above the post
  info on list and single pages.
- `caption`: short text shown next to the page title in the title bar
  (single posts).

## Site params

- `params.caption`: caption text beside the blog title on the home page.
- `params.author`: name shown in the "By:" line of post info.
- `params.description`: text of the About widget in the sidebar.
- `menus.main`: header navigation.
