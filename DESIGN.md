---
version: alpha
name: OpenIntern
description: A free, open, daily-refreshed listing of tech internships for students, printed like a batch report so it can be scanned row by row, marked up, and taken away as data.
colors:
  paper: "#fbfcfa"      # the ground
  ink: "#1e2420"        # text, primary buttons, focus rings
  muted: "#535c56"      # secondary text, 6.7:1 on paper, 5.9:1 on bar
  bar: "#e6f0e3"        # the green bar: every other row of a listing
  rule: "#c9d3c8"       # dividers (decorative, not a boundary a reader needs)
  edge: "#7d8a7f"       # control borders, 3.5:1 on paper
  marker: "#ffe866"     # your own marks: rows you applied to
  alarm: "#a8271f"      # stale data, failed runs, errors
  mark-bar: "#4f9a5a"   # the brand mark's bars only, never text
colors-dark:
  paper: "#191c1a"
  ink: "#e4e9e3"
  muted: "#a2aba3"      # 7.3:1 on paper, 6.3:1 on bar
  bar: "#222a23"
  rule: "#333c34"
  edge: "#6f7d71"       # 4.0:1 on paper
  marker: "#e3cf3f"     # text on it switches to paper (10.9:1)
  alarm: "#ff8d80"
  mark-bar: "#6fb77a"
typography:
  body:
    fontFamily: Atkinson Hyperlegible Next
    fontSize: 15px
    lineHeight: 1.45
    fontWeight: 400
  row-title:
    fontFamily: Atkinson Hyperlegible Next
    fontSize: 16.5px
    lineHeight: 1.25
    fontWeight: 700
  h2:
    fontFamily: Atkinson Hyperlegible Next
    fontSize: 20px
    fontWeight: 700
  h1:
    fontFamily: Atkinson Hyperlegible Next
    fontSize: 30px
    fontWeight: 800
    letterSpacing: -0.01em
  display:
    fontFamily: Atkinson Hyperlegible Next
    fontSize: clamp(32px, 5vw, 46px)
    fontWeight: 800
    letterSpacing: -0.015em
  small:
    fontFamily: Atkinson Hyperlegible Next
    fontSize: 13.5px
  code:
    fontFamily: Atkinson Hyperlegible Mono
    fontSize: 14px
rounded:
  control: 3px
  surface: 0
spacing:
  1: 4px
  2: 8px
  3: 12px
  4: 16px
  5: 24px
  6: 40px
  7: 64px
components:
  button-primary:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.paper}"
    rounded: "{rounded.control}"
    padding: 7px 14px
  button:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    border: 1px solid {colors.edge}
    rounded: "{rounded.control}"
    padding: 6px 12px
---

## Overview

OpenIntern is a free, open-source board of tech internships, read every day
from public job boards, with a public API and daily dumps. Two readers: students
scanning hundreds of listings fast, and developers reading the API reference.

The look comes from the thing the product actually produces: a daily batch
listing. Before screens, a nightly job printed its report on continuous
green-bar paper, whose alternating pale bands let a reader follow one row across
many columns. Students then went through such lists with a highlighter. The site
keeps exactly those two moves, because both are functional for this reader: the
**green bar** to follow a row across columns, the **marker** for the rows you
applied to. Everything else is plain: paper, ink, one legible face.

## Colors

- **Paper (#fbfcfa / dark #191c1a):** the ground. A near-white with no warmth,
  so it never reads as cream or parchment; the dark ground is a neutral,
  slightly green-grey, not navy or espresso.
- **Ink (#1e2420 / #e4e9e3):** all text, primary buttons, focus rings.
  15.4:1 on paper (14.0:1 dark).
- **Muted (#535c56 / #a2aba3):** secondary facts (region, duration, excerpt).
  AA on both paper and bar in both themes.
- **Bar (#e6f0e3 / #222a23):** the green bar. Only on alternate rows of a
  listing (board rows, data tables, fact lists). Never a page or panel fill.
- **Marker (#ffe866 / #e3cf3f):** one job: the student's own applied marks
  (title highlight, pressed Applied button). Never used for decoration or for
  something the site says.
- **Alarm (#a8271f / #ff8d80):** one job: something is wrong (stale data,
  failed ingest runs, errors). 6.9:1 on paper.
- Rule, edge and mark-bar are structural: dividers, control borders (3:1 or
  better), and the bars inside the logo.

There is no link colour. Links are ink with an underline; the underline
thickens on hover. Students do not need a brand blue to know a title is a link
when every title in the list is one.

## Typography

One family: **Atkinson Hyperlegible Next** (Braille Institute, OFL), chosen
because its job is the reader's job here: telling similar letterforms apart at
small sizes while scanning. Weights 400 (body), 700 (row titles, headings),
800 (page titles and the home display line). **Atkinson Hyperlegible Mono**
only for code: curl commands, API paths, parameter names, JSON. Never for
labels or numbers in prose.

Scale: 13.5 small, 15 body, 16.5 row title, 20 h2, 30 h1, 32-46 display (home
only). Numbers in tables use `font-variant-numeric: tabular-nums`. Sentence
case everywhere; no all-caps labels, no letter-spaced eyebrows.

## Layout

The site's one structural idea: **every page is a report printed from the
corpus**. A short heading block sits flush left, then the rows. The board is
the main report; job detail is one record from it, health is the run log, docs
are the field list. Width follows the role of the page: listings use the full
`76rem` sheet, prose pages a `40rem` measure, docs `56rem`.

The board on wide screens is a columned listing: Company, Role (title, roles,
excerpt, actions), Location, Term, Seen. Columns line up down the page so a
reader can run an eye down "Location" alone. Filters sit in a left column as a
plain GET form. Below 1100 px the columns fold into each row; below 760 px the
filters fold into a disclosure above the list.

Home leads with the product: a left text block (headline, the honest sentence
about what it is, the real counts in a sentence, the curl command) beside the
newest real listings as clickable rows. No centered hero, no stat strip, no
pill row.

## Elevation & Depth

Flat. No shadows anywhere. Separation comes from the green bar, rules, and
space. The only thing that "lifts" is a focused control (outline).

## Shapes

Rows, tables, panels: square (0). Controls (buttons, inputs, selects,
checkboxes): 3px. Nothing is a pill.

## Components

- **Header:** brand mark + wordmark left, nav (Jobs, Docs, GitHub) right on the
  same baseline, a rule below. Identical on every page.
- **Footer:** one sentence and one row of links (Docs, Compare, Privacy,
  Health, GitHub, Dumps), a rule above. Identical on every page.
- **Listing rows** (`.rows`): `li` per role family; odd rows paper, even rows
  bar. Applied rows get the marker behind the title.
- **Fact list** (`.facts`): a `dl` printed as banded label/value rows. Used on
  job detail and health.
- **Data table** (`.table`): banded rows, ink header with a 2px ink rule,
  tabular numbers, horizontal scroll inside its own wrapper.
- **Buttons:** primary (ink fill, paper text) for the one action per row or
  page (Apply); secondary (paper, edge border) for everything else; text links
  for minor actions (Report issue). Hover darkens or fills; active presses by
  1px; focus is a 2px ink outline with 2px offset; disabled lowers opacity.
- **Notice** (`.notice`): a full-width line with an alarm-coloured rule above
  and below, for stale data and errors.
- **Code** (`.code`): mono on bar, square, scrolls horizontally; the curl bar
  adds a Copy button (JavaScript only).

## Motion

Almost none. Colour and underline transitions of 120 ms on interactive
elements; everything is disabled under `prefers-reduced-motion: reduce`.

## Do's and Don'ts

- Do reuse the header, footer, tokens and components above on every page,
  unchanged. Add a token here before a page uses it.
- Do keep the bar for rows a reader follows across columns, and the marker for
  the reader's own marks. Nothing else gets colour.
- Do keep every page working with JavaScript off: filters are a GET form, the
  apply picker is a `<details>`, the Applied toggle and copy buttons appear only
  when the script runs.
- Don't use: SD1 cream + terracotta, SD2 near-black + acid accent, SD9 navy,
  slate, espresso or sand grounds, SD4 all-caps or mono labels, `·`-joined meta
  strings, SD5 an accented headline word, SD6 decorative numbering, SD7 fake
  window chrome, K2 rounded shadowed cards, K12 coloured badge chips, I2 emoji
  as icons, CP3 arrows glued to links, L1 the centered hero, L4/L11 stat strips
  or a big number as the hero, coloured initials avatars.
