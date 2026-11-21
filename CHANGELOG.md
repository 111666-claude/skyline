# Changelog

## Unreleased

### 🏙️ Compare view

- New `-compare "FROM..TO,FROM..TO"` flag draws two date ranges as side-by-side cities on one shared height scale, with a title per city and a single shared legend. Works with GitHub, Gitea (`-gitea`) and `-svg`
- New `-titles "A,B"` flag sets the two city titles; the default is the date ranges. Overlong titles are shortened with an ellipsis
- Ranges grow outward to whole Sunday-first weeks, missing days render as dark windows, and weeks spanning New Year stay one building
- Overlapping or empty ranges are errors, future end dates clamp to today, and a too-narrow terminal stops with a message naming both widths instead of wrapping
- GitHub and Gitea rate limiting now fails with a clear "rate limited" error (including the quota reset time when GitHub provides one)

## 1.1.0 - October 9, 2026

### 🍵 Gitea

- New `-gitea <base-url>` flag draws the city from a Gitea or Forgejo instance, including self-hosted ones
- Public heatmaps work without a token. Set `GITEA_TOKEN` for instances that require sign-in
- Window brightness levels are worked out from your own year's activity, since Gitea doesn't provide GitHub-style levels

## 1.0.0 - October 3, 2026

### 🏷️ Versioning

- First stable release. Same code and features as 0.2.0, now tagged as 1.0.0
- The themes preview image in the README now shows all 12 themes

## 0.2.0 - October 3, 2026

### 🎨 Themes

- Two new themes: `prism` (neon rainbow on dark) and `rainbow` (classic rainbow on black)
- Every theme except `neon` and `synthwave` got darker backgrounds and more contrast between window levels, so lit windows stand out instead of blending into one shade
- `matrix` now sits on near-black, `ice` mixes deep and light blues on dark gray, and `amber` borrows Ayu's orange, warm yellow and soft red
- `sunset`, `toxic`, `vapor` and `crimson` now use a second hue in their windows (toxic got a purple accent, for example)

## 0.1.0 - October 3, 2026

### 🌃 Skyline

- First release. Draws your GitHub contribution graph as an ASCII city in the terminal
- Building height follows weekly totals on a log scale, and window colors follow GitHub's four contribution levels
- 10 color themes via `-theme`: neon, synthwave, matrix, amber, ice, sunset, toxic, vapor, crimson and mono
- `-themes` lists every theme with a color swatch
- The record week gets an antenna, and month labels run along the street
- `-height` and `-weeks` flags control the size of the city

### 🖼️ SVG

- `-svg` writes an animated SVG with flickering windows and twinkling stars, and the animation respects reduced motion
- Example GitHub Actions workflow that redraws the skyline nightly for a profile README
