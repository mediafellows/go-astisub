package astisub_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	astisub "github.com/asticode/go-astisub"
)

func positionDocument(parameters, head, body string) string {
	return `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:tts="http://www.w3.org/ns/ttml#styling" xmlns:ttp="http://www.w3.org/ns/ttml#parameter" ` + parameters + `><head>` + head + `</head><body>` + body + `</body></tt>`
}

func positionVTT(t *testing.T, source string) (*astisub.Subtitles, string) {
	t.Helper()
	subs, err := astisub.ReadFromTTML(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := subs.WriteToWebVTTWithOptions(&output, astisub.WebVTTOptions{TTMLExactAlignment: true}); err != nil {
		t.Fatal(err)
	}
	return subs, output.String()
}

func TestTTMLPositionNinePlaces(t *testing.T) {
	for _, vertical := range []struct{ align, line string }{{"before", "10%,start"}, {"center", "50%,center"}, {"after", "90%,end"}} {
		for _, horizontal := range []struct{ align, position string }{{"left", "10%,line-left"}, {"center", "50%,center"}, {"right", "90%,line-right"}} {
			t.Run(vertical.align+"_"+horizontal.align, func(t *testing.T) {
				source := positionDocument("", fmt.Sprintf(`<layout><region xml:id="r" tts:origin="10%% 10%%" tts:extent="80%% 80%%" tts:displayAlign="%s" tts:textAlign="%s"/></layout>`, vertical.align, horizontal.align), `<div><p region="r" begin="1s" end="3s">Invented first line<br/>Invented second line</p></div>`)
				subs, output := positionVTT(t, source)
				want := fmt.Sprintf("00:00:01.000 --> 00:00:03.000 align:%s line:%s position:%s size:80%%\nInvented first line\nInvented second line", horizontal.align, vertical.line, horizontal.position)
				if !strings.Contains(output, want) {
					t.Fatalf("missing %q in\n%s", want, output)
				}
				if strings.Contains(output, "REGION") || strings.Contains(output, "region:") {
					t.Fatal("TTML output must not depend on WebVTT regions")
				}
				if subs.Items[0].Region == nil {
					t.Fatal("original region was lost")
				}
				again, err := astisub.ReadFromWebVTT(strings.NewReader(output))
				if err != nil {
					t.Fatal(err)
				}
				cue := again.Items[0]
				if cue.StartAt != time.Second || cue.EndAt != 3*time.Second || cue.InlineStyle.WebVTTLine != vertical.line || cue.InlineStyle.WebVTTPosition.String() != horizontal.position {
					t.Fatalf("roundtrip changed cue: %+v", cue)
				}
				var buffer bytes.Buffer
				if err := again.WriteToWebVTT(&buffer); err != nil {
					t.Fatal(err)
				}
				if buffer.String() != output {
					t.Fatal("WebVTT read/write changed placement")
				}
			})
		}
	}
}

func TestTTMLPositionGeometry(t *testing.T) {
	tests := []struct{ name, parameters, attrs, want string }{
		{"asymmetric percent", "", `tts:origin="12.5% 23.25%" tts:extent="60% 40%" tts:displayAlign="after" tts:textAlign="center"`, `align:center line:63.25%,end position:42.5%,center size:60%`},
		{"pixels", `tts:extent="1920px 1080px"`, `tts:origin="192px 108px" tts:extent="1536px 864px" tts:displayAlign="after" tts:textAlign="center"`, `align:center line:90%,end position:50%,center size:80%`},
		{"default cells", "", `tts:origin="4c 3c" tts:extent="24c 9c"`, `align:left line:20%,start position:12.5%,line-left size:75%`},
		{"custom cells", `ttp:cellResolution="40 20"`, `tts:origin="4c 2c" tts:extent="32c 16c"`, `align:left line:10%,start position:10%,line-left size:80%`},
		{"mixed units", `tts:extent="1000px 500px"`, `tts:origin="10% 50px" tts:extent="800px 80%"`, `align:left line:10%,start position:10%,line-left size:80%`},
		{"region defaults", "", ``, `align:left line:0%,start position:0%,line-left size:100%`},
		{"auto defaults", "", `tts:origin="auto" tts:extent="auto"`, `align:left line:0%,start position:0%,line-left size:100%`},
		{"whitespace", "", `tts:origin="  10%   20% " tts:extent="70% 60%"`, `align:left line:20%,start position:10%,line-left size:70%`},
		{"padding relative to region", "", `tts:origin="10% 20%" tts:extent="80% 60%" tts:padding="10%"`, `align:left line:26%,start position:18%,line-left size:64%`},
		{"zero pixel padding without canvas", "", `tts:padding="0px"`, `align:left line:0%,start position:0%,line-left size:100%`},
		{"padding one", "", `tts:padding="5%"`, `align:left line:5%,start position:5%,line-left size:90%`},
		{"padding two", "", `tts:padding="5% 10%"`, `align:left line:5%,start position:10%,line-left size:80%`},
		{"padding three", "", `tts:padding="5% 10% 15%" tts:displayAlign="after"`, `align:left line:85%,end position:10%,line-left size:80%`},
		{"padding four", "", `tts:padding="5% 10% 15% 20%" tts:displayAlign="center"`, `align:left line:45%,center position:20%,line-left size:70%`},
		{"RTL start", "", `tts:writingMode="rltb" tts:textAlign="start" tts:origin="10% 20%" tts:extent="70% 60%"`, `align:right line:20%,start position:80%,line-right size:70%`},
		{"RTL end", "", `tts:writingMode="rl" tts:textAlign="end"`, `align:left line:0%,start position:0%,line-left size:100%`},
		{"explicit direction", "", `tts:direction="rtl" tts:textAlign="start"`, `align:right line:0%,start position:100%,line-right size:100%`},
		{"vertical lr before", "", `tts:writingMode="tblr" tts:origin="10% 20%" tts:extent="60% 70%"`, `align:left line:10%,start position:20%,line-left size:70% vertical:lr`},
		{"vertical lr after", "", `tts:writingMode="tblr" tts:origin="10% 20%" tts:extent="60% 70%" tts:displayAlign="after"`, `align:left line:70%,end position:20%,line-left size:70% vertical:lr`},
		{"vertical rl before", "", `tts:writingMode="tbrl" tts:origin="10% 20%" tts:extent="60% 70%"`, `align:left line:30%,start position:20%,line-left size:70% vertical:rl`},
		{"vertical rl center", "", `tts:writingMode="tb" tts:origin="10% 20%" tts:extent="60% 70%" tts:displayAlign="center"`, `align:left line:60%,center position:20%,line-left size:70% vertical:rl`},
		{"vertical rl after", "", `tts:writingMode="tbrl" tts:origin="10% 20%" tts:extent="60% 70%" tts:displayAlign="after"`, `align:left line:90%,end position:20%,line-left size:70% vertical:rl`},
		{"vertical padding", "", `tts:writingMode="tblr" tts:padding="5% 10% 15% 20%"`, `align:left line:5%,start position:20%,line-left size:70% vertical:lr`},
		{"RTL padding", "", `tts:writingMode="rltb" tts:padding="5% 10% 15% 20%"`, `align:right line:5%,start position:80%,line-right size:70%`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, output := positionVTT(t, positionDocument(tt.parameters, `<layout><region xml:id="r" `+tt.attrs+`/></layout>`, `<div region="r"><p begin="1s" end="2s">Geometry</p></div>`))
			if !strings.Contains(output, " --> 00:00:02.000 "+tt.want+"\nGeometry") {
				t.Fatalf("want %s in\n%s", tt.want, output)
			}
		})
	}
}

func TestTTMLPositionStyleResolution(t *testing.T) {
	head := `<styling>
 <style xml:id="base" tts:origin="10% 20%" tts:extent="80% 60%"/>
 <style xml:id="top" style="base" tts:displayAlign="before"/>
 <style xml:id="bottom" style="base" tts:displayAlign="after"/>
 <style xml:id="middle" tts:displayAlign="center"/>
 <style xml:id="left" tts:textAlign="left"/>
 <style xml:id="right" tts:textAlign="right"/>
 </styling><layout>
 <region xml:id="a" style="top"/>
 <region xml:id="b" style="bottom"/>
 <region xml:id="c" style="bottom middle"><style tts:textAlign="center"/></region>
 <region xml:id="d" style="top"><style style="middle" tts:displayAlign="after"/></region>
 <region xml:id="e" style="bottom" tts:displayAlign="before"><style tts:displayAlign="center"/></region>
 </layout>`
	body := `<div region="a" style="left" tts:textAlign="right">
 <p begin="1s" end="2s">Inherited inline</p>
 <div region="b"><p begin="2s" end="3s" style="left">Nested</p></div>
 <p begin="3s" end="4s" region="c" style="left right" tts:textAlign="center">Inline wins</p>
 <p begin="4s" end="5s" region="d" style="right left">Nested region</p>
 <p begin="5s" end="6s" region="e">Region inline wins</p>
 </div>`
	subs, output := positionVTT(t, positionDocument("", head, body))
	wants := []string{
		"align:right line:20%,start position:90%,line-right size:80%\nInherited inline",
		"align:left line:80%,end position:10%,line-left size:80%\nNested",
		"align:center line:50%,center position:50%,center size:80%\nInline wins",
		"align:left line:80%,end position:10%,line-left size:80%\nNested region",
		"align:right line:20%,start position:90%,line-right size:80%\nRegion inline wins",
	}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q in\n%s", want, output)
		}
	}
	if len(subs.Items) != 5 {
		t.Fatalf("lost cues: %d", len(subs.Items))
	}
	for i, cue := range subs.Items {
		if cue.StartAt != time.Duration(i+1)*time.Second {
			t.Fatalf("document order changed at %d", i)
		}
	}
	// Geometry must survive the library's normal cue-fragmentation operation.
	subs.Fragment(time.Second / 2)
	var fragmented bytes.Buffer
	if err := subs.WriteToWebVTTWithOptions(&fragmented, astisub.WebVTTOptions{TTMLExactAlignment: true}); err != nil {
		t.Fatal(err)
	}
	if strings.Count(fragmented.String(), wants[0]) != 2 {
		t.Fatal("fragmentation lost resolved layout")
	}
}

func TestTTMLPositionBodyRegionAndStyles(t *testing.T) {
	source := positionDocument("", `<styling><style xml:id="s" tts:textAlign="right"/></styling><layout><region xml:id="r" tts:displayAlign="after"/></layout>`, `<div><p begin="1s" end="2s">Body</p></div>`)
	source = strings.Replace(source, "<body>", `<body region="r" style="s">`, 1)
	_, output := positionVTT(t, source)
	if !strings.Contains(output, "align:right line:100%,end position:100%,line-right size:100%") {
		t.Fatal(output)
	}
}

func TestTTMLPositionFallback(t *testing.T) {
	for _, tt := range []struct{ name, parameters, attrs string }{
		{"pixels without canvas", "", `tts:origin="100px 200px"`},
		{"zero canvas", `tts:extent="0px 0px"`, `tts:origin="10px 20px"`},
		{"invalid canvas", `tts:extent="NaNpx Infinitypx"`, `tts:origin="10px 20px"`},
		{"invalid cells", `ttp:cellResolution="0 0"`, `tts:origin="1c 2c"`},
		{"font relative units", "", `tts:origin="1em 2em"`},
		{"non finite", "", `tts:origin="NaN% 20%"`},
		{"infinite", "", `tts:extent="Inf% 20%"`},
		{"overflow", "", `tts:origin="1e999% 20%"`},
		{"negative extent", "", `tts:extent="-10% 20%"`},
		{"empty extent", "", `tts:extent="0% 20%"`},
		{"outside viewport", "", `tts:origin="90% 90%" tts:extent="20% 20%"`},
		{"negative origin", "", `tts:origin="-10% 20%"`},
		{"one coordinate", "", `tts:origin="10%"`},
		{"bad padding", "", `tts:padding="bad"`},
		{"oversized padding", "", `tts:padding="60%"`},
		{"negative padding", "", `tts:padding="-1%"`},
		{"unknown writing mode", "", `tts:writingMode="unknown"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, output := positionVTT(t, positionDocument(tt.parameters, `<layout><region xml:id="r" tts:textAlign="center" `+tt.attrs+`/></layout>`, `<div><p region="r" begin="1s" end="2s">Still readable</p></div>`))
			if !strings.Contains(output, "00:00:01.000 --> 00:00:02.000 align:center\nStill readable") {
				t.Fatal(output)
			}
			for _, bad := range []string{" line:", " position:", " size:", "region:"} {
				if strings.Contains(output, bad) {
					t.Fatalf("unsafe fallback: %s", output)
				}
			}
		})
	}
}

func TestTTMLPositionReferenceErrors(t *testing.T) {
	for _, tt := range []struct{ name, head, body, want string }{
		{"missing style", `<styling><style xml:id="s" style="missing"/></styling>`, `<div><p begin="1s" end="2s">Text</p></div>`, `does not exist`},
		{"cycle", `<styling><style xml:id="a" style="b"/><style xml:id="b" style="a"/></styling>`, ``, `cyclic`},
		{"self cycle", `<styling><style xml:id="a" style="a"/></styling>`, ``, `cyclic`},
		{"missing region", "", `<div><p region="missing" begin="1s" end="2s">Text</p></div>`, `doesn't exist`},
		{"missing nested style", `<layout><region xml:id="r"><style style="missing"/></region></layout>`, ``, `does not exist`},
		{"missing div style", "", `<div style="missing"><p begin="1s" end="2s">Text</p></div>`, `does not exist`},
		{"missing p style", "", `<div><p style="missing" begin="1s" end="2s">Text</p></div>`, `does not exist`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := astisub.ReadFromTTML(strings.NewReader(positionDocument("", tt.head, tt.body)))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestTTMLPositionNoRegionAndTiming(t *testing.T) {
	_, output := positionVTT(t, positionDocument(`ttp:timeBase="smpte" ttp:frameRate="24" ttp:frameRateMultiplier="1000 1001"`, "", `<div><p begin="00:00:02:00" end="00:00:03:00">Unpositioned</p></div>`))
	if !strings.Contains(output, "00:00:02.002 --> 00:00:03.003\nUnpositioned") {
		t.Fatal(output)
	}
}

func TestTTMLPositionCompatibleOutput(t *testing.T) {
	for _, align := range []struct{ input, line string }{{"before", "10%"}, {"center", "50%"}, {"after", "90%"}} {
		t.Run(align.input, func(t *testing.T) {
			source := positionDocument("", `<layout><region xml:id="r" tts:origin="10% 10%" tts:extent="80% 80%" tts:displayAlign="`+align.input+`" tts:textAlign="center"/></layout>`, `<div><p region="r" begin="1s" end="2s">First<br/>Second</p></div>`)
			subs, exact := positionVTT(t, source)
			var compatible bytes.Buffer
			if err := subs.WriteToWebVTT(&compatible); err != nil {
				t.Fatal(err)
			}
			want := "align:center line:" + align.line + " position:50% size:80%\nFirst\nSecond"
			if !strings.Contains(compatible.String(), want) {
				t.Fatalf("missing %q in %s", want, compatible.String())
			}
			var again bytes.Buffer
			if err := subs.WriteToWebVTTWithOptions(&again, astisub.WebVTTOptions{TTMLExactAlignment: true}); err != nil {
				t.Fatal(err)
			}
			if exact != again.String() {
				t.Fatal("default writer mutated the stored exact geometry")
			}
		})
	}
}
