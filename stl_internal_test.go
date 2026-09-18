package astisub

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/asticode/go-astikit"
	"github.com/stretchr/testify/assert"
)

func TestSTLColorFromSRTAndIntoTTML(t *testing.T) {
	s, err := ReadFromSRT(strings.NewReader("1\n00:00:01,000 --> 00:00:02,000\n<font color=\"#ff0000\">Red</font>\n"))
	assert.NoError(t, err)
	if !assert.Len(t, s.Items, 1) {
		return
	}
	s.Metadata = &Metadata{Framerate: 25, STLDisplayStandardCode: "0"}
	var out bytes.Buffer
	assert.NoError(t, s.WriteToSTL(&out))
	converted, err := ReadFromSTL(bytes.NewReader(out.Bytes()), STLOptions{})
	assert.NoError(t, err)
	if assert.Len(t, converted.Items, 1) && assert.Len(t, converted.Items[0].Lines, 1) && assert.Len(t, converted.Items[0].Lines[0].Items, 1) {
		style := converted.Items[0].Lines[0].Items[0].InlineStyle
		assert.Equal(t, ColorRed, style.STLColor)
		assert.Equal(t, ColorRed, style.TTMLColor)
	}
}

func TestSTLStringMatchesColorValue(t *testing.T) {
	for _, tc := range []struct {
		name  string
		color *Color
		code  byte
	}{
		{name: "new red value", color: &Color{Red: 255}, code: 0x01},
		{name: "parsed hex red", color: newColorFromHTMLString("#ff0000"), code: 0x01},
		{name: "lime maps to STL green", color: &Color{Green: 255}, code: 0x02},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := (LineItem{Text: "text", InlineStyle: &StyleAttributes{STLColor: tc.color}}).STLString()
			assert.Equal(t, append([]byte{tc.code}, []byte("text")...), []byte(got))
		})
	}
}

func TestOpenSubtitleStyleCodes(t *testing.T) {
	h, err := newSTLCharacterHandler(stlCharacterCodeTableNumberLatin)
	assert.NoError(t, err)
	i := &Item{}
	assert.NoError(t, parseOpenSubtitleRow(i, h, func() styler { return newSTLStyler() }, []byte{0x80, 'A', 0x81, 'B'}))
	if assert.Len(t, i.Lines, 1) && assert.Len(t, i.Lines[0].Items, 2) {
		assert.Equal(t, "A", i.Lines[0].Items[0].Text)
		assert.Equal(t, "B", i.Lines[0].Items[1].Text)
		assert.Equal(t, astikit.BoolPtr(true), i.Lines[0].Items[0].InlineStyle.STLItalics)
		assert.Equal(t, astikit.BoolPtr(false), i.Lines[0].Items[1].InlineStyle.STLItalics)
	}
}

func TestSTLUnknownDiskFormatCode(t *testing.T) {
	// A GSI block whose disk format code is not recognized used to leave the framerate
	// at 0, which panicked with an integer division by zero when parsing a subsequent
	// TTI block's timecodes. It now falls back to the default framerate of 25.
	b := bytes.Repeat([]byte{0x20}, stlBlockSizeGSI)
	// Valid character code table number (Latin) so parsing reaches timecode handling.
	b[12] = 0x30
	b[13] = 0x30
	copy(b[3:11], []byte("XXXXXXXX")) // unknown disk format code
	in := append(b, make([]byte, stlBlockSizeTTI)...)
	assert.NotPanics(t, func() {
		ReadFromSTL(bytes.NewReader(in), STLOptions{})
	})
}

func TestSTLDuration(t *testing.T) {
	// Default
	d, err := parseDurationSTL("12345678", 100)
	assert.NoError(t, err)
	assert.Equal(t, 12*time.Hour+34*time.Minute+56*time.Second+780*time.Millisecond, d)
	s := formatDurationSTL(d, 100)
	assert.Equal(t, "12345678", s)

	// Bytes
	b := formatDurationSTLBytes(d, 100)
	assert.Equal(t, []byte{0xc, 0x22, 0x38, 0x4e}, b)
	d2 := parseDurationSTLBytes([]byte{0xc, 0x22, 0x38, 0x4e}, 100)
	assert.Equal(t, d, d2)
}

func TestSTLCharacterHandler(t *testing.T) {
	h, err := newSTLCharacterHandler(stlCharacterCodeTableNumberLatin)
	assert.NoError(t, err)
	o := h.decode(0x1f)
	assert.Equal(t, []byte(nil), o)
	o = h.decode(0x65)
	assert.Equal(t, []byte("e"), o)
	o = h.decode(0xc1)
	assert.Equal(t, []byte(nil), o)
	o = h.decode(0x65)
	assert.Equal(t, []byte("è"), o)
}

func TestSTLCharacterHandlerUmlaut(t *testing.T) {
	h, err := newSTLCharacterHandler(stlCharacterCodeTableNumberLatin)
	assert.NoError(t, err)

	o := h.decode(0xc8)
	assert.Equal(t, []byte(nil), o)
	o = h.decode(0x61)
	assert.Equal(t, []byte("ä"), o)

	o = h.decode(0xc8)
	assert.Equal(t, []byte(nil), o)
	o = h.decode(0x41)
	assert.Equal(t, []byte("Ä"), o)

	o = h.decode(0xc8)
	assert.Equal(t, []byte(nil), o)
	o = h.decode(0x6f)
	assert.Equal(t, []byte("ö"), o)

	o = h.decode(0xc8)
	assert.Equal(t, []byte(nil), o)
	o = h.decode(0x4f)
	assert.Equal(t, []byte("Ö"), o)

	o = h.decode(0xc8)
	assert.Equal(t, []byte(nil), o)
	o = h.decode(0x75)
	assert.Equal(t, []byte("ü"), o)

	o = h.decode(0xc8)
	assert.Equal(t, []byte(nil), o)
	o = h.decode(0x55)
	assert.Equal(t, []byte("Ü"), o)

	o = h.decode(0xc8)
	assert.Equal(t, []byte(nil), o)
	o = h.decode(0x65)
	assert.Equal(t, []byte("ë"), o)

	o = h.decode(0xc8)
	assert.Equal(t, []byte(nil), o)
	o = h.decode(0x45)
	assert.Equal(t, []byte("Ë"), o)

	o = h.decode(0xc8)
	assert.Equal(t, []byte(nil), o)
	o = h.decode(0x69)
	assert.Equal(t, []byte("ï"), o)

	o = h.decode(0xc8)
	assert.Equal(t, []byte(nil), o)
	o = h.decode(0x49)
	assert.Equal(t, []byte("Ï"), o)
}

func TestSTLStyler(t *testing.T) {
	// Parse spacing attributes
	s := newSTLStyler()
	s.parseSpacingAttribute(0x80)
	assert.Equal(t, stlStyler{italics: astikit.BoolPtr(true)}, *s)
	s.parseSpacingAttribute(0x81)
	assert.Equal(t, stlStyler{italics: astikit.BoolPtr(false)}, *s)
	s = newSTLStyler()
	s.parseSpacingAttribute(0x82)
	assert.Equal(t, stlStyler{underline: astikit.BoolPtr(true)}, *s)
	s.parseSpacingAttribute(0x83)
	assert.Equal(t, stlStyler{underline: astikit.BoolPtr(false)}, *s)
	s = newSTLStyler()
	s.parseSpacingAttribute(0x84)
	assert.Equal(t, stlStyler{boxing: astikit.BoolPtr(true)}, *s)
	s.parseSpacingAttribute(0x85)
	assert.Equal(t, stlStyler{boxing: astikit.BoolPtr(false)}, *s)

	// Has been set
	s = newSTLStyler()
	assert.False(t, s.hasBeenSet())
	s.boxing = astikit.BoolPtr(true)
	assert.True(t, s.hasBeenSet())
	s = newSTLStyler()
	s.italics = astikit.BoolPtr(true)
	assert.True(t, s.hasBeenSet())
	s = newSTLStyler()
	s.underline = astikit.BoolPtr(true)
	assert.True(t, s.hasBeenSet())

	// Has changed
	s = newSTLStyler()
	sa := &StyleAttributes{}
	assert.False(t, s.hasChanged(sa))
	s.boxing = astikit.BoolPtr(true)
	assert.True(t, s.hasChanged(sa))
	sa.STLBoxing = s.boxing
	assert.False(t, s.hasChanged(sa))
	s.italics = astikit.BoolPtr(true)
	assert.True(t, s.hasChanged(sa))
	sa.STLItalics = s.italics
	assert.False(t, s.hasChanged(sa))
	s.underline = astikit.BoolPtr(true)
	assert.True(t, s.hasChanged(sa))
	sa.STLUnderline = s.underline
	assert.False(t, s.hasChanged(sa))

	// Update
	s = newSTLStyler()
	sa = &StyleAttributes{}
	s.update(sa)
	assert.Equal(t, StyleAttributes{}, *sa)
	s.boxing = astikit.BoolPtr(true)
	s.update(sa)
	assert.Equal(t, StyleAttributes{STLBoxing: s.boxing}, *sa)
	s.italics = astikit.BoolPtr(true)
	s.update(sa)
	assert.Equal(t, StyleAttributes{STLBoxing: s.boxing, STLItalics: s.italics}, *sa)
	s.underline = astikit.BoolPtr(true)
	s.update(sa)
	assert.Equal(t, StyleAttributes{STLBoxing: s.boxing, STLItalics: s.italics, STLUnderline: s.underline}, *sa)
}
