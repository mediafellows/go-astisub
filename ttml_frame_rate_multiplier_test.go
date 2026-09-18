package astisub_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asticode/go-astisub"
)

func TestTTMLFrameRateMultiplierFixtures(t *testing.T) {
	tests := []struct {
		name       string
		fixture    string
		wantCues   int
		wantTiming []string
	}{
		{
			name:     "SMPTE non-drop counts every nominal frame",
			fixture:  "frame-rate-multiplier-smpte.dfxp",
			wantCues: 3,
			wantTiming: []string{
				"00:00:02.002 --> 00:00:03.003",
				"00:10:00.600 --> 00:10:01.601",
				"01:53:50.615 --> 01:53:51.616",
			},
		},
		{
			name:     "media time scales only frame parts and frame offsets",
			fixture:  "frame-rate-multiplier-media.dfxp",
			wantCues: 5,
			wantTiming: []string{
				"01:53:43.792 --> 01:53:44.792",
				"00:00:04.170 --> 00:00:04.587",
				"00:00:00.104 --> 00:00:00.145",
				"00:00:01.500 --> 00:00:02.000",
				"00:01:02.250 --> 00:01:03.250",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", tt.fixture))
			if err != nil {
				t.Fatal(err)
			}
			output := writeTTMLAsWebVTT(t, string(input))
			if got := strings.Count(output, " --> "); got != tt.wantCues {
				t.Fatalf("VTT has %d cues, want %d:\n%s", got, tt.wantCues, output)
			}
			for _, timing := range tt.wantTiming {
				if !strings.Contains(output, timing) {
					t.Errorf("VTT missing %q:\n%s", timing, output)
				}
			}
		})
	}
}

func TestTTMLFrameRateMultiplierDefaultsAndMarkerMode(t *testing.T) {
	input, err := os.ReadFile(filepath.Join("testdata", "frame-rate-multiplier-smpte.dfxp"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		xml  string
		want string
	}{
		{
			name: "one to one",
			xml:  strings.Replace(string(input), "1000 1001", "1 1", 1),
			want: "01:53:43.791 --> 01:53:44.791",
		},
		{
			name: "legacy colon-separated one to one",
			xml:  strings.Replace(string(input), "1000 1001", "1:1", 1),
			want: "01:53:43.791 --> 01:53:44.791",
		},
		{
			name: "legacy colon-separated fractional rate",
			xml:  strings.Replace(string(input), "1000 1001", "1000:1001", 1),
			want: "01:53:50.615 --> 01:53:51.616",
		},
		{
			name: "absent multiplier defaults to one to one",
			xml:  strings.Replace(string(input), ` ttp:frameRateMultiplier="1000 1001"`, "", 1),
			want: "01:53:43.791 --> 01:53:44.791",
		},
		{
			name: "explicit continuous marker mode",
			xml:  strings.Replace(string(input), `ttp:timeBase="smpte"`, `ttp:timeBase="smpte" ttp:markerMode="continuous"`, 1),
			want: "01:53:50.615 --> 01:53:51.616",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := writeTTMLAsWebVTT(t, tt.xml)
			if !strings.Contains(output, tt.want) {
				t.Errorf("VTT missing %q:\n%s", tt.want, output)
			}
		})
	}

	mediaInput, err := os.ReadFile(filepath.Join("testdata", "frame-rate-multiplier-media.dfxp"))
	if err != nil {
		t.Fatal(err)
	}
	withoutTimeBase := strings.Replace(string(mediaInput), `ttp:timeBase="media" `, "", 1)
	if output := writeTTMLAsWebVTT(t, withoutTimeBase); !strings.Contains(output, "01:53:43.792 --> 01:53:44.792") {
		t.Errorf("default media time base scales only frames:\n%s", output)
	}
	withoutFrameRate := strings.Replace(syntheticTTML("30", "1000 1001", "media", "nonDrop", "00:00:00:15", "00:00:01:00"), `ttp:frameRate="30" `, "", 1)
	if output := writeTTMLAsWebVTT(t, withoutFrameRate); !strings.Contains(output, "00:00:00.500 --> 00:00:01.000") {
		t.Errorf("default frame rate is not 30 fps:\n%s", output)
	}
}

func TestTTMLFrameRateMultiplierDropModes(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		begin   string
		want    string
		invalid string
	}{
		{"NTSC", "dropNTSC", "00:01:00:02", "00:01:00.060", "00:01:00:01"},
		{"PAL", "dropPAL", "00:02:00:04", "00:02:00.120", "00:02:00:03"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := syntheticTTML("30", "1000 1001", "smpte", tt.mode, tt.begin, tt.begin)
			output := writeTTMLAsWebVTT(t, input)
			if !strings.Contains(output, tt.want+" --> "+tt.want) {
				t.Errorf("VTT missing %q:\n%s", tt.want, output)
			}
			if _, err := astisub.ReadFromTTML(strings.NewReader(syntheticTTML("30", "1000 1001", "smpte", tt.mode, tt.invalid, tt.begin))); err == nil {
				t.Errorf("invalid dropped-frame code %s accepted", tt.invalid)
			}
		})
	}
}

func TestTTMLFrameRateMultiplierInvalid(t *testing.T) {
	for _, multiplier := range []string{"", "0 1001", "1000 0", "1000", "1000 1001 1", "foo 1001", "-1 2", "999999999999999999999 1"} {
		t.Run(multiplier, func(t *testing.T) {
			_, err := astisub.ReadFromTTML(strings.NewReader(syntheticTTML("24", multiplier, "smpte", "nonDrop", "00:00:01:00", "00:00:02:00")))
			if err == nil || !strings.Contains(err.Error(), "frameRateMultiplier") {
				t.Errorf("multiplier %q error = %v, want frameRateMultiplier error", multiplier, err)
			}
		})
	}
	_, err := astisub.ReadFromTTML(strings.NewReader(syntheticTTML("1", "1 1", "media", "nonDrop", "100000000000f", "100000000001f")))
	if err == nil {
		t.Error("frame offset overflowing time.Duration was accepted")
	}
}

func syntheticTTML(frameRate, multiplier, timeBase, dropMode, begin, end string) string {
	return fmt.Sprintf(`<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttp="http://www.w3.org/ns/ttml#parameter" ttp:frameRate="%s" ttp:frameRateMultiplier="%s" ttp:timeBase="%s" ttp:dropMode="%s"><body><div><p begin="%s" end="%s">A synthetic cue.</p></div></body></tt>`, frameRate, multiplier, timeBase, dropMode, begin, end)
}

func writeTTMLAsWebVTT(t *testing.T, input string) string {
	t.Helper()
	subs, err := astisub.ReadFromTTML(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := subs.WriteToWebVTT(&output); err != nil {
		t.Fatal(err)
	}
	return output.String()
}
