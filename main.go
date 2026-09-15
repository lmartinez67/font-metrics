// fontmetrics prints the metrics stored in a TrueType or OpenType font's
// head, hhea, and maxp tables, without needing a font editor or a browser
// devtools panel to find them.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) > 2 {
		usage()
		os.Exit(2)
	}

	var r io.Reader = os.Stdin
	path := "stdin"
	if len(os.Args) == 2 && os.Args[1] != "-" {
		path = os.Args[1]
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fontmetrics: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		r = f
	}

	font, err := parseFont(bufio.NewReader(r))
	if err != nil {
		fmt.Fprintf(os.Stderr, "fontmetrics: %s: %v\n", path, err)
		os.Exit(1)
	}

	if err := report(os.Stdout, font); err != nil {
		fmt.Fprintf(os.Stderr, "fontmetrics: %s: %v\n", path, err)
		os.Exit(1)
	}
}

func report(w io.Writer, f *Font) error {
	head, err := f.Head()
	if err != nil {
		return err
	}
	hhea, err := f.Hhea()
	if err != nil {
		return err
	}
	numGlyphs, err := f.NumGlyphs()
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "format:          %s\n", f.VersionString())
	if f.HasTable("name") {
		family, style := f.FamilyAndStyle()
		if family != "" {
			fmt.Fprintf(w, "family:          %s\n", family)
		}
		if style != "" {
			fmt.Fprintf(w, "style:           %s\n", style)
		}
	}
	fmt.Fprintf(w, "units per em:    %d\n", head.UnitsPerEm)
	fmt.Fprintf(w, "ascender:        %d\n", hhea.Ascender)
	fmt.Fprintf(w, "descender:       %d\n", hhea.Descender)
	fmt.Fprintf(w, "line gap:        %d\n", hhea.LineGap)
	fmt.Fprintf(w, "glyphs:          %d\n", numGlyphs)
	fmt.Fprintf(w, "bounding box:    [%d %d %d %d]\n", head.XMin, head.YMin, head.XMax, head.YMax)

	// OS/2 is common but optional in the sfnt spec, so a font missing it
	// just gets fewer lines in the report rather than an error.
	if f.HasTable("OS/2") {
		os2, err := f.OS2()
		if err != nil {
			return err
		}
		if name := WeightClassName(os2.WeightClass); name != "" {
			fmt.Fprintf(w, "weight class:    %d (%s)\n", os2.WeightClass, name)
		} else {
			fmt.Fprintf(w, "weight class:    %d\n", os2.WeightClass)
		}
		if os2.HasXHeight {
			fmt.Fprintf(w, "x-height:        %d\n", os2.XHeight)
			fmt.Fprintf(w, "cap height:      %d\n", os2.CapHeight)
		}
	}
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: fontmetrics [file]")
	fmt.Fprintln(os.Stderr, "  reads a TrueType or OpenType font and prints its metrics")
	fmt.Fprintln(os.Stderr, `  with no file, or "-", reads the font from stdin`)
}
