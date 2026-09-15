# fontmetrics

A command-line tool that reads the metrics baked into a TrueType or
OpenType font file: family and style name, units per em, ascender,
descender, line gap, glyph count, the font's overall bounding box, and,
when the font has an OS/2 table, its weight class, x-height, and
cap-height.

These numbers live in a font's binary tables (`name`, `head`, `hhea`,
`maxp`, `OS/2`) and most tools only expose them through a GUI font
inspector or a full font editor. `fontmetrics` just reads the tables and
prints the numbers, which is what you actually want when you're debugging
line-height math in CSS, comparing two font files, or writing a build
script that needs to check a font before shipping it.

## Usage

From a file:

```
$ fontmetrics Inter-Regular.ttf
format:          TrueType
family:          Inter
style:           Regular
units per em:    2048
ascender:        2005
descender:       -462
line gap:        0
glyphs:          3892
bounding box:    [-1029 -462 2775 2005]
weight class:    400 (Regular)
x-height:        1118
cap height:      1493
```

The family and style lines only appear if the font has a `name` table and
that table actually has a value for them, which is effectively always for
real fonts. `fontmetrics` prefers the typographic family/subfamily names
(IDs 16/17) when present, since those give the intended family grouping
for fonts with many weights, and falls back to the legacy family/subfamily
names (IDs 1/2) that every font is required to carry.

The weight class, x-height, and cap-height lines only appear if the font
has an `OS/2` table; x-height and cap-height specifically need an `OS/2`
table version 2 or later, since earlier versions don't carry those two
fields at all.

From stdin, using `-` or just piping in:

```
$ cat Inter-Regular.ttf | fontmetrics
$ curl -s https://example.com/font.otf | fontmetrics -
```

With no arguments and nothing on stdin, it blocks waiting for input, same
as any other Unix filter.

## Building

```
$ go build -o fontmetrics .
```

Requires Go 1.22 or later. No third-party dependencies.

## How it works

Both TrueType and OpenType fonts use the same outer container, called
sfnt: a short header naming how many tables the file has, followed by a
directory of tag/offset/length triples, followed by the tables
themselves. `fontmetrics` parses that directory and pulls the tables that carry the
metrics above, plus the family and style strings out of `name`. It
doesn't touch glyph outlines, kerning, or the rest of `name`'s fifty-odd
other fields (copyright, trademark, license URL, and so on), so it works
the same whether you're pointing it at a full desktop font or a subsetted
webfont.

## Limitations

Right now it reads `name`, `head`, `hhea`, `maxp`, and `OS/2`. It doesn't
yet unwrap WOFF/WOFF2 compression, so it only works on plain TrueType and
OpenType files. See the roadmap in the repo for what's planned next.
