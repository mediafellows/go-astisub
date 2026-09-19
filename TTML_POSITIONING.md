# DFXP / TTML positioning in WebVTT

`ReadFromTTML` resolves layout and `WriteToWebVTT` emits standalone cue settings.
DFXP uses the same reader. Native WebVTT region serialization is unchanged.

## Mapping

Geometry comes from the selected region, including its referenced and nested
styles. Later style references override earlier ones; inline attributes override
nested and referenced styles. Paragraph alignment inherits through body/div
containers, with paragraph references and inline attributes taking precedence.
Region selection inherits through nested divisions. Shared references and style
chains are supported; missing references and cycles return descriptive errors.
Non-inheritable region properties such as `displayAlign` are not taken from a
paragraph or its ancestors.

With `WriteToWebVTTWithOptions(w, WebVTTOptions{TTMLExactAlignment: true})`,
for a horizontal region's content rectangle `(x, y, width, height)`:

| TTML displayAlign | WebVTT line |
| --- | --- |
| before (default) | y%,start |
| center | (y + height/2)%,center |
| after | (y + height)%,end |

`textAlign` determines the horizontal anchor, `position` and `align`; `size` is
the content width. Logical start/end resolve against paragraph direction.
Vertical `tblr`/`tbrl` (and aliases) use `vertical:lr`/`vertical:rl` with the
corresponding axes. Anchoring applies to the entire multiline cue, without
estimating font heights or a fixed number of region lines.

Percent origins/extents use the root canvas. Pixels require a declared root
`tts:extent`; cell lengths use `ttp:cellResolution` (default 32 by 15).
Padding accepts one to four logical edge values; percentage padding is relative
to the region's dimensions. Missing origin means (0,0); missing/auto extent means
the full canvas. Existing documents without an explicit region retain player
default placement.

The original region/style graph remains available for TTML output. Resolved cue
geometry is retained with each item, including through fragmentation. Writing
WebVTT does not mutate that graph, emit TTML-derived REGION blocks, or attach
region references to the converted cues.

## Default player compatibility

`WriteToWebVTT` keeps its existing signature and emits plain percentages such as
`line:90% position:50% size:80%`. Native Chromium 150 ignores entire settings with
alignment suffixes, so emitting those by default would lose placement again.
The default retains top/middle/bottom coordinates but center/after positions are
approximate for multiline text: the renderer anchors the start of the cue there.
At the exact viewport boundaries, the default uses snap-to-lines `line:0` or
`line:-1` to keep multiline cues visible instead of clipping at 0% or 100%.
Other near-boundary content may still be adjusted by the renderer.

Supporting renderers can opt into exact block and position anchors using the
additive `WriteToWebVTTWithOptions` method. No duplicate settings or nonstandard
WebVTT syntax is emitted. This option only affects TTML-derived cues; native
WebVTT settings and region behavior are unchanged.

## Limits and playback fallback

Unknown units (including font-relative lengths), pixels without canvas dimensions,
invalid/non-finite geometry, invalid cell grids, empty or out-of-viewport content
rectangles, and unknown writing modes omit geometric cue settings. Readable text,
cue order and timing are retained; valid alignment/writing direction can remain.
Zero pixel lengths do not require canvas dimensions. There is no guessed video
resolution and no conversion of pixel numbers directly into percentages.

This is a positioning conversion, not a complete TTML renderer. Font metrics,
region backgrounds/clipping, animated layout, stacking simultaneous paragraphs
as a single region block, and pixel-identical typography are not reproduced.
Nested divisions are flattened for layout; this change does not add inherited
or sequential timing semantics. Existing timestamp and color behavior remains.

## Specification references

- [TTML1 styling and layout](https://www.w3.org/TR/ttml1/), especially sections
  8.2.6 (displayAlign), 8.2.7 (extent), 8.2.14 (origin), 8.2.16 (padding),
  8.2.18 (textAlign), 8.2.24 (writingMode), 8.4 (style resolution), and 9 (layout).
- [TTML / WebVTT mapping guidance](https://w3c.github.io/ttml-webvtt-mapping/):
  flatten applicable styles and convert region geometry to cue settings.
- [Current WebVTT specification](https://www.w3.org/TR/webvtt1/): authoritative
  cue syntax and alignment semantics; older examples in the mapping document
  predate the current position-anchor syntax.

Tests use invented text and geometry. Customer subtitle files are not fixtures.
Run `go test -race ./...`, `go vet ./...`, and `go build ./...`.
