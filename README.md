# fontmetrics

A command-line tool that reads the metrics baked into a TrueType or
OpenType font file: units per em, ascender, descender, line gap, glyph
count, and the font's overall bounding box.

These numbers live in a font's binary tables (`head`, `hhea`, `maxp`) and
most tools only expose them through a GUI font inspector or a full font
editor. `fontmetrics` just reads the tables and prints the numbers, which
is what you actually want when you're debugging line-height math in CSS,
comparing two font files, or writing a build script that needs to check a
font before shipping it.

## Usage

From a file:

```
$ fontmetrics Inter-Regular.ttf
format:          TrueType
units per em:    2048
ascender:        2005
descender:       -462
line gap:        0
glyphs:          3892
bounding box:    [-1029 -462 2775 2005]
```

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
themselves. `fontmetrics` parses that directory and pulls the three
tables that carry the metrics above. It doesn't touch glyph outlines,
kerning, or the tables that hold text like the font's name and license,
so it works the same whether you're pointing it at a full desktop font
or a subsetted webfont.

## Limitations

Right now it only reads `head`, `hhea`, and `maxp`. It doesn't yet read
`OS/2` (which carries x-height, cap-height, and weight class on fonts
that include it), the `name` table (font family and style strings), or
unwrap WOFF/WOFF2 compression. See the roadmap in the repo for what's
planned next.
