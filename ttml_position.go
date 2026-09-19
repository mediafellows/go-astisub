package astisub

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// TTML1 sections 8.4 and 9: resolve references before applying a property's
// inheritance/applicability rules. Later references win; inline values win last.
type ttmlStyleResolver struct {
	definitions map[string]TTMLInStyle
	styles      map[string]*StyleAttributes
	visiting    map[string]bool
}

func newTTMLStyleResolver(definitions []TTMLInStyle) (*ttmlStyleResolver, error) {
	r := &ttmlStyleResolver{make(map[string]TTMLInStyle), make(map[string]*StyleAttributes), make(map[string]bool)}
	for _, definition := range definitions {
		r.definitions[definition.ID] = definition
	}
	for _, definition := range definitions {
		if _, err := r.resolve(definition.ID); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *ttmlStyleResolver) resolve(id string) (*StyleAttributes, error) {
	if style := r.styles[id]; style != nil {
		return style, nil
	}
	definition, ok := r.definitions[id]
	if !ok {
		return nil, fmt.Errorf("astisub: TTML style %q does not exist", id)
	}
	if r.visiting[id] {
		return nil, fmt.Errorf("astisub: cyclic TTML style reference %q", id)
	}
	r.visiting[id] = true
	style, err := r.header(definition.TTMLInHeader)
	if err != nil {
		return nil, err
	}
	delete(r.visiting, id)
	r.styles[id] = style
	return style, nil
}

func (r *ttmlStyleResolver) header(h TTMLInHeader) (*StyleAttributes, error) {
	style := h.TTMLInStyleAttributes.styleAttributes()
	for i := len(h.Styles) - 1; i >= 0; i-- {
		nested, err := r.header(h.Styles[i].TTMLInHeader)
		if err != nil {
			return nil, err
		}
		style.merge(nested)
	}
	referenced, err := r.attributes(TTMLInStyleAttributes{}, h.Style)
	if err != nil {
		return nil, err
	}
	style.merge(referenced)
	return style, nil
}

func (r *ttmlStyleResolver) attributes(inline TTMLInStyleAttributes, references string) (*StyleAttributes, error) {
	style := inline.styleAttributes()
	refs := strings.Fields(references)
	for i := len(refs) - 1; i >= 0; i-- {
		parent, err := r.resolve(refs[i])
		if err != nil {
			return nil, err
		}
		style.merge(parent)
	}
	return style, nil
}

// flatten preserves document order, including interleaved paragraphs/divisions.
// It flattens placement inheritance only; it does not reinterpret cue timing.
func (r *ttmlStyleResolver) flatten(body TTMLInBody) ([]TTMLInBodyDiv, error) {
	inherited, err := r.attributes(body.TTMLInStyleAttributes, body.Style)
	if err != nil {
		return nil, err
	}
	var out []TTMLInBodyDiv
	var visit func(TTMLInBodyDiv, *StyleAttributes, string, string) error
	visit = func(div TTMLInBodyDiv, parent *StyleAttributes, region, styleID string) error {
		effective, err := r.attributes(div.TTMLInStyleAttributes, div.Style)
		if err != nil {
			return err
		}
		effective.merge(parent)
		if div.Region == "" {
			div.Region = region
		}
		if div.Style == "" {
			div.Style = styleID
		}
		div.resolvedStyle = effective
		appendParagraph := func(p TTMLInSubtitle) {
			leaf := div
			leaf.Divs, leaf.Content = nil, ""
			leaf.Subtitles = []TTMLInSubtitle{p}
			out = append(out, leaf)
		}
		decoder := xml.NewDecoder(strings.NewReader(div.Content))
		pi, di := 0, 0
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			start, ok := token.(xml.StartElement)
			if !ok {
				continue
			}
			switch start.Name.Local {
			case "p":
				appendParagraph(div.Subtitles[pi])
				pi++
			case "div":
				if err := visit(div.Divs[di], effective, div.Region, div.Style); err != nil {
					return err
				}
				di++
			}
			if err := decoder.Skip(); err != nil {
				return err
			}
		}
		return nil
	}
	for _, div := range body.Divs {
		if err := visit(div, inherited, body.Region, body.Style); err != nil {
			return nil, err
		}
	}
	return out, nil
}

type ttmlCanvas struct{ width, height, columns, rows float64 }

func ttmlDocumentCanvas(doc TTMLIn) ttmlCanvas {
	c := ttmlCanvas{columns: 32, rows: 15}
	if doc.Extent != nil {
		values := strings.Fields(*doc.Extent)
		if len(values) == 2 {
			c.width, _ = ttmlPositivePixels(values[0])
			c.height, _ = ttmlPositivePixels(values[1])
		}
	}
	if doc.CellResolution != "" {
		values := strings.Fields(doc.CellResolution)
		// Invalid declared resolutions must not silently use the default grid.
		c.columns, c.rows = 0, 0
		if len(values) == 2 {
			columns, e1 := strconv.Atoi(values[0])
			rows, e2 := strconv.Atoi(values[1])
			if e1 == nil && e2 == nil && columns > 0 && rows > 0 {
				c.columns, c.rows = float64(columns), float64(rows)
			}
		}
	}
	return c
}

func ttmlPositivePixels(value string) (float64, bool) {
	if !strings.HasSuffix(value, "px") {
		return 0, false
	}
	n, err := strconv.ParseFloat(strings.TrimSuffix(value, "px"), 64)
	return n, err == nil && !math.IsNaN(n) && !math.IsInf(n, 0) && n > 0
}

func (c ttmlCanvas) length(value string, horizontal bool) (float64, bool) {
	base, cells := c.height, c.rows
	if horizontal {
		base, cells = c.width, c.columns
	}
	multiplier := 1.0
	switch {
	case strings.HasSuffix(value, "%"):
		value = strings.TrimSuffix(value, "%")
	case strings.HasSuffix(value, "px"):
		if base <= 0 || math.IsNaN(base) || math.IsInf(base, 0) {
			n, err := strconv.ParseFloat(strings.TrimSuffix(value, "px"), 64)
			return 0, err == nil && n == 0
		}
		value = strings.TrimSuffix(value, "px")
		multiplier = 100 / base
	case strings.HasSuffix(value, "c"):
		if cells <= 0 {
			return 0, false
		}
		value = strings.TrimSuffix(value, "c")
		multiplier = 100 / cells
	default:
		if value != "0" {
			return 0, false
		}
	}
	n, err := strconv.ParseFloat(value, 64)
	n *= multiplier
	return n, err == nil && !math.IsNaN(n) && !math.IsInf(n, 0)
}

func (c ttmlCanvas) pair(value *string, defaultX, defaultY float64) (float64, float64, bool) {
	if value == nil || strings.TrimSpace(*value) == "auto" {
		return defaultX, defaultY, true
	}
	parts := strings.Fields(*value)
	if len(parts) != 2 {
		return 0, 0, false
	}
	x, okX := c.length(parts[0], true)
	y, okY := c.length(parts[1], false)
	return x, y, okX && okY
}

// Padding's one-to-four values are before/end/after/start in the writing mode,
// not physical CSS top/right/bottom/left for vertical or right-to-left regions.
func (c ttmlCanvas) padding(value *string, mode string, width, height float64) ([4]float64, bool) {
	var physical [4]float64 // top, right, bottom, left
	if value == nil {
		return physical, true
	}
	parts := strings.Fields(*value)
	var logical [4]string
	switch len(parts) {
	case 1:
		logical = [4]string{parts[0], parts[0], parts[0], parts[0]}
	case 2:
		logical = [4]string{parts[0], parts[1], parts[0], parts[1]}
	case 3:
		logical = [4]string{parts[0], parts[1], parts[2], parts[1]}
	case 4:
		copy(logical[:], parts)
	default:
		return physical, false
	}
	mapping := [4]int{0, 1, 2, 3}
	switch mode {
	case "rltb":
		mapping = [4]int{0, 3, 2, 1}
	case "tblr":
		mapping = [4]int{3, 2, 1, 0}
	case "tbrl":
		mapping = [4]int{1, 2, 3, 0}
	}
	for i, value := range logical {
		side := mapping[i]
		n, ok := c.length(value, side == 1 || side == 3)
		if strings.HasSuffix(value, "%") {
			if side == 1 || side == 3 {
				n *= width / 100
			} else {
				n *= height / 100
			}
		}
		if !ok || n < 0 {
			return physical, false
		}
		physical[side] = n
	}
	return physical, true
}

func ttmlString(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return strings.TrimSpace(*value)
}

func ttmlPercent(n float64) string {
	// Bound the serialized precision and avoid emitting negative zero.
	if math.Abs(n) < 0.0000005 {
		n = 0
	}
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(n, 'f', 6, 64), "0"), ".") + "%"
}

// ttmlCuePosition maps TTML1 region layout to standalone WebVTT cue settings.
// See https://www.w3.org/TR/ttml1/#styling-attribute-displayAlign and
// https://www.w3.org/TR/webvtt1/#webvtt-cue-settings . Geometry is all-or-nothing:
// unresolved units must not accidentally place text at the top-left corner.
func ttmlCuePosition(doc TTMLIn, region, paragraph *StyleAttributes) *StyleAttributes {
	out := &StyleAttributes{}
	effective := *paragraph
	if region != nil {
		effective.merge(region)
	}
	align := ttmlString(effective.TTMLTextAlign, "start")
	direction := ttmlString(effective.TTMLDirection, "ltr")
	mode := "lrtb"
	if region != nil {
		mode = ttmlString(region.TTMLWritingMode, "lrtb")
	}
	switch mode {
	case "lr":
		mode = "lrtb"
	case "rl":
		mode = "rltb"
	case "tb":
		mode = "tbrl"
	}
	if mode == "rltb" && effective.TTMLDirection == nil {
		direction = "rtl"
	}
	// Resolve logical text alignment, so Latin text inside an RTL region is not
	// accidentally positioned using the browser's first-strong-character guess.
	switch align {
	case "start":
		if direction == "rtl" {
			align = "right"
		} else {
			align = "left"
		}
	case "end":
		if direction == "rtl" {
			align = "left"
		} else {
			align = "right"
		}
	case "left", "right", "center":
	default:
		align = "left"
	}
	// Documents without an explicit region retain the player's default placement.
	if region == nil {
		if effective.TTMLTextAlign != nil {
			out.WebVTTAlign = align
		}
		return out
	}
	out.WebVTTAlign = align
	switch mode {
	case "lrtb", "rltb":
	case "tblr":
		out.WebVTTVertical = "lr"
	case "tbrl":
		out.WebVTTVertical = "rl"
	default:
		return out
	}
	canvas := ttmlDocumentCanvas(doc)
	x, y, ok := canvas.pair(region.TTMLOrigin, 0, 0)
	if !ok {
		return out
	}
	w, h, ok := canvas.pair(region.TTMLExtent, 100, 100)
	if !ok || w <= 0 || h <= 0 {
		return out
	}
	padding, ok := canvas.padding(region.TTMLPadding, mode, w, h)
	if !ok {
		return out
	}
	x += padding[3]
	y += padding[0]
	w -= padding[1] + padding[3]
	h -= padding[0] + padding[2]
	if x < 0 || y < 0 || w <= 0 || h <= 0 || x+w > 100.0000001 || y+h > 100.0000001 {
		return out
	}
	// Along the block progression axis, before/center/after anchor the whole
	// cue (including all its lines), without estimating font or line heights.
	blockOrigin, blockExtent, inlineOrigin, inlineExtent := y, h, x, w
	if mode == "tblr" {
		blockOrigin, blockExtent, inlineOrigin, inlineExtent = x, w, y, h
	}
	if mode == "tbrl" {
		blockOrigin, blockExtent, inlineOrigin, inlineExtent = 100-x-w, w, y, h
	}
	anchor := "start"
	switch ttmlString(region.TTMLDisplayAlign, "before") {
	case "center":
		blockOrigin += blockExtent / 2
		anchor = "center"
	case "after":
		blockOrigin += blockExtent
		anchor = "end"
	}
	out.WebVTTLine = ttmlPercent(blockOrigin) + "," + anchor
	position, positionAnchor := inlineOrigin, "line-left"
	if align == "center" {
		position += inlineExtent / 2
		positionAnchor = "center"
	}
	if align == "right" {
		position += inlineExtent
		positionAnchor = "line-right"
	}
	out.WebVTTPosition = &WebVTTPosition{XPosition: ttmlPercent(position), Alignment: positionAnchor}
	out.WebVTTSize = ttmlPercent(inlineExtent)
	return out
}
