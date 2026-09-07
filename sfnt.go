package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
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
