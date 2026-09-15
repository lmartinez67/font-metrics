package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"unicode/utf16"
)

// sfnt is the container format used by both TrueType and OpenType fonts:
// a fixed header, a table directory, and then the tables themselves at
// arbitrary offsets. We keep the whole file in memory because tables
// reference each other by absolute offset and there's no way to know
// how much of the file we'll need until we've read the directory.

const (
	versionTrueType = 0x00010000
	versionAppleTrue = 0x74727565 // 'true', used by some older Mac fonts
	versionOpenType  = 0x4f54544f // 'OTTO', CFF-flavored OpenType
)

type tableRecord struct {
	offset uint32
	length uint32
}

// Font holds a parsed sfnt table directory. Individual tables are only
// decoded on demand, since most callers only care about a handful of them.
type Font struct {
	data    []byte
	tables  map[string]tableRecord
	version uint32
}

func parseFont(r io.Reader) (*Font, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading font data: %w", err)
	}
	if len(data) < 12 {
		return nil, errors.New("file too small to be a font")
	}

	version := binary.BigEndian.Uint32(data[0:4])
	switch version {
	case versionTrueType, versionAppleTrue, versionOpenType:
	default:
		return nil, fmt.Errorf("unrecognized sfnt version %#08x", version)
	}

	numTables := binary.BigEndian.Uint16(data[4:6])

	const headerSize = 12
	const recordSize = 16
	tables := make(map[string]tableRecord, numTables)
	for i := 0; i < int(numTables); i++ {
		recOffset := headerSize + i*recordSize
		if recOffset+recordSize > len(data) {
			return nil, errors.New("truncated table directory")
		}
		rec := data[recOffset : recOffset+recordSize]
		tag := string(rec[0:4])
		tables[tag] = tableRecord{
			offset: binary.BigEndian.Uint32(rec[8:12]),
			length: binary.BigEndian.Uint32(rec[12:16]),
		}
	}

	return &Font{data: data, tables: tables, version: version}, nil
}

func (f *Font) table(tag string) ([]byte, error) {
	rec, ok := f.tables[tag]
	if !ok {
		return nil, fmt.Errorf("font has no %q table", tag)
	}
	end := uint64(rec.offset) + uint64(rec.length)
	if end > uint64(len(f.data)) {
		return nil, fmt.Errorf("%q table extends past end of file", tag)
	}
	return f.data[rec.offset:end], nil
}

// VersionString reports the flavor of sfnt this file is, mainly so the
// report can distinguish glyph outlines stored as TrueType curves from
// ones stored as CFF (PostScript-style) outlines.
func (f *Font) VersionString() string {
	switch f.version {
	case versionTrueType, versionAppleTrue:
		return "TrueType"
	case versionOpenType:
		return "OpenType (CFF)"
	default:
		return fmt.Sprintf("unknown (%#08x)", f.version)
	}
}

// HeadMetrics is the subset of the "head" table we report.
type HeadMetrics struct {
	UnitsPerEm             uint16
	XMin, YMin, XMax, YMax int16
}

func (f *Font) Head() (HeadMetrics, error) {
	t, err := f.table("head")
	if err != nil {
		return HeadMetrics{}, err
	}
	if len(t) < 44 {
		return HeadMetrics{}, errors.New("head table too short")
	}
	return HeadMetrics{
		UnitsPerEm: binary.BigEndian.Uint16(t[18:20]),
		XMin:       int16(binary.BigEndian.Uint16(t[36:38])),
		YMin:       int16(binary.BigEndian.Uint16(t[38:40])),
		XMax:       int16(binary.BigEndian.Uint16(t[40:42])),
		YMax:       int16(binary.BigEndian.Uint16(t[42:44])),
	}, nil
}

// HheaMetrics is the subset of the "hhea" (horizontal header) table we report.
type HheaMetrics struct {
	Ascender, Descender, LineGap int16
	NumberOfHMetrics             uint16
}

func (f *Font) Hhea() (HheaMetrics, error) {
	t, err := f.table("hhea")
	if err != nil {
		return HheaMetrics{}, err
	}
	if len(t) < 36 {
		return HheaMetrics{}, errors.New("hhea table too short")
	}
	return HheaMetrics{
		Ascender:         int16(binary.BigEndian.Uint16(t[4:6])),
		Descender:        int16(binary.BigEndian.Uint16(t[6:8])),
		LineGap:          int16(binary.BigEndian.Uint16(t[8:10])),
		NumberOfHMetrics: binary.BigEndian.Uint16(t[34:36]),
	}, nil
}

// NumGlyphs reads the glyph count from the "maxp" table.
func (f *Font) NumGlyphs() (uint16, error) {
	t, err := f.table("maxp")
	if err != nil {
		return 0, err
	}
	if len(t) < 6 {
		return 0, errors.New("maxp table too short")
	}
	return binary.BigEndian.Uint16(t[4:6]), nil
}

// HasTable reports whether the font's directory lists the given tag.
// OS/2 in particular is common but not required by the sfnt spec, so
// callers need to check before treating its absence as an error.
func (f *Font) HasTable(tag string) bool {
	_, ok := f.tables[tag]
	return ok
}

// OS2Metrics is the subset of the "OS/2" table we report. XHeight and
// CapHeight are only present in table version 2 and later; HasXHeight
// tells the caller whether they were actually read from the file rather
// than left at their zero value.
type OS2Metrics struct {
	Version            uint16
	WeightClass        uint16
	XHeight, CapHeight int16
	HasXHeight         bool
}

func (f *Font) OS2() (OS2Metrics, error) {
	t, err := f.table("OS/2")
	if err != nil {
		return OS2Metrics{}, err
	}
	if len(t) < 6 {
		return OS2Metrics{}, errors.New("OS/2 table too short")
	}
	m := OS2Metrics{
		Version:     binary.BigEndian.Uint16(t[0:2]),
		WeightClass: binary.BigEndian.Uint16(t[4:6]),
	}
	// sxHeight and sCapHeight were added in version 2; earlier versions
	// don't carry them at all, and a short read here just means an old
	// font, not a malformed one.
	if m.Version >= 2 && len(t) >= 90 {
		m.XHeight = int16(binary.BigEndian.Uint16(t[86:88]))
		m.CapHeight = int16(binary.BigEndian.Uint16(t[88:90]))
		m.HasXHeight = true
	}
	return m, nil
}

// weightClassNames maps the common usWeightClass values from the OS/2
// spec to the names font tools usually show for them. Values outside
// this table (fonts can use anything from 1 to 1000) are printed as
// plain numbers.
var weightClassNames = map[uint16]string{
	100: "Thin",
	200: "Extra Light",
	300: "Light",
	400: "Regular",
	500: "Medium",
	600: "Semi Bold",
	700: "Bold",
	800: "Extra Bold",
	900: "Black",
}

// WeightClassName returns the human-readable name for a usWeightClass
// value, or "" if it doesn't match one of the standard values.
func WeightClassName(class uint16) string {
	return weightClassNames[class]
}

// Standard name IDs from the "name" table spec. There are dozens more
// (copyright, trademark, license URL, ...) but these four are the ones
// worth surfacing in a metrics report.
const (
	nameIDFamily            = 1
	nameIDSubfamily         = 2
	nameIDTypographicFamily = 16
	nameIDTypographicSub    = 17
)

// NameRecord is one entry in the "name" table: a platform/encoding/language
// tagged string keyed by a standard name ID.
type NameRecord struct {
	PlatformID uint16
	EncodingID uint16
	LanguageID uint16
	NameID     uint16
	Value      string
}

// Names reads every record out of the font's "name" table. Fonts commonly
// repeat the same strings once per platform they want to support (Mac,
// Windows, sometimes Unicode), so a given NameID usually shows up more
// than once with different PlatformID/LanguageID.
func (f *Font) Names() ([]NameRecord, error) {
	t, err := f.table("name")
	if err != nil {
		return nil, err
	}
	if len(t) < 6 {
		return nil, errors.New("name table too short")
	}

	count := binary.BigEndian.Uint16(t[2:4])
	stringAreaOffset := int(binary.BigEndian.Uint16(t[4:6]))

	const headerSize = 6
	const recordSize = 12
	records := make([]NameRecord, 0, count)
	for i := 0; i < int(count); i++ {
		recOffset := headerSize + i*recordSize
		if recOffset+recordSize > len(t) {
			return nil, errors.New("truncated name record")
		}
		rec := t[recOffset : recOffset+recordSize]
		length := int(binary.BigEndian.Uint16(rec[8:10]))
		strOffset := int(binary.BigEndian.Uint16(rec[10:12]))

		start := stringAreaOffset + strOffset
		end := start + length
		if start < 0 || end > len(t) {
			// A record pointing past the end of the table is a malformed
			// font, but there's no reason to fail the whole report over
			// one bad string when the metrics tables are fine.
			continue
		}

		platformID := binary.BigEndian.Uint16(rec[0:2])
		records = append(records, NameRecord{
			PlatformID: platformID,
			EncodingID: binary.BigEndian.Uint16(rec[2:4]),
			LanguageID: binary.BigEndian.Uint16(rec[4:6]),
			NameID:     binary.BigEndian.Uint16(rec[6:8]),
			Value:      decodeNameString(platformID, t[start:end]),
		})
	}
	return records, nil
}

// decodeNameString converts the raw bytes of a name table entry to a Go
// string. Platforms 0 (Unicode) and 3 (Windows) store text as UTF-16BE.
// Platform 1 (Macintosh) stores single-byte Mac Roman, which is identical
// to ASCII for the printable range that family and style names actually
// use, so a plain byte conversion is close enough without a full Mac Roman
// table.
func decodeNameString(platformID uint16, b []byte) string {
	if platformID == 1 {
		return string(b)
	}
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	u16 := make([]uint16, len(b)/2)
	for i := range u16 {
		u16[i] = binary.BigEndian.Uint16(b[i*2 : i*2+2])
	}
	return string(utf16.Decode(u16))
}

// BestName returns the value of the given name ID, preferring the Windows
// platform's US English record since that's the one most consistently
// filled in across fonts in the wild, and falling back to whatever record
// is available otherwise.
func (f *Font) BestName(nameID uint16) (string, bool) {
	records, err := f.Names()
	if err != nil {
		return "", false
	}
	var fallback string
	haveFallback := false
	for _, r := range records {
		if r.NameID != nameID {
			continue
		}
		const platformWindows, languageUSEnglish = 3, 0x0409
		if r.PlatformID == platformWindows && r.LanguageID == languageUSEnglish {
			return r.Value, true
		}
		if !haveFallback {
			fallback, haveFallback = r.Value, true
		}
	}
	return fallback, haveFallback
}

// FamilyAndStyle returns the font's family and style names. It prefers the
// typographic name IDs (16/17), which carry the intended grouping for
// fonts with many weights (e.g. family "Inter", not "Inter Semi Bold"),
// and falls back to the legacy family/subfamily IDs (1/2) that every font
// is required to have.
func (f *Font) FamilyAndStyle() (family, style string) {
	family, ok := f.BestName(nameIDTypographicFamily)
	if !ok {
		family, _ = f.BestName(nameIDFamily)
	}
	style, ok = f.BestName(nameIDTypographicSub)
	if !ok {
		style, _ = f.BestName(nameIDSubfamily)
	}
	return family, style
}
