package main

import (
	"reflect"
	"testing"
)

func TestExtractDirectDownloadArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantArgs []string
		wantOk   bool
	}{
		{
			name:     "bare song link",
			input:    "https://music.apple.com/us/album/style/1440933849?i=1440933854",
			wantArgs: []string{"https://music.apple.com/us/album/style/1440933849?i=1440933854"},
			wantOk:   true,
		},
		{
			name:     "album link with hyphenated flags",
			input:    "https://music.apple.com/us/album/1989/1440933849 -atmos -tgz",
			wantArgs: []string{"https://music.apple.com/us/album/1989/1440933849", "-atmos", "-tgz"},
			wantOk:   true,
		},
		{
			name:     "album link with bare flags",
			input:    "https://music.apple.com/us/album/1989/1440933849 aac go",
			wantArgs: []string{"https://music.apple.com/us/album/1989/1440933849", "aac", "go"},
			wantOk:   true,
		},
		{
			name:     "flags before link",
			input:    "-flac https://music.apple.com/us/album/1989/1440933849",
			wantArgs: []string{"-flac", "https://music.apple.com/us/album/1989/1440933849"},
			wantOk:   true,
		},
		{
			name:     "share sheet title prefix",
			input:    "Listen to Style by Taylor Swift on Apple Music: https://music.apple.com/us/album/style/1440933849?i=1440933854",
			wantArgs: []string{"https://music.apple.com/us/album/style/1440933849?i=1440933854"},
			wantOk:   true,
		},
		{
			name:     "conversational sentence with flag",
			input:    "Check out this album https://music.apple.com/us/album/123 -atmos",
			wantArgs: []string{"https://music.apple.com/us/album/123", "-atmos"},
			wantOk:   true,
		},
		{
			name:     "conversational sentence with ordinary word go (not treated as flag)",
			input:    "Here you go: https://music.apple.com/us/album/123",
			wantArgs: []string{"https://music.apple.com/us/album/123"},
			wantOk:   true,
		},
		{
			name:     "brackets and trailing sentence punctuation",
			input:    "Listen to (<https://music.apple.com/us/album/123>).",
			wantArgs: []string{"https://music.apple.com/us/album/123"},
			wantOk:   true,
		},
		{
			name:     "url without scheme",
			input:    "music.apple.com/us/album/123",
			wantArgs: []string{"https://music.apple.com/us/album/123"},
			wantOk:   true,
		},
		{
			name:     "multiple links",
			input:    "https://music.apple.com/us/album/1 https://music.apple.com/us/album/2 -atmos",
			wantArgs: []string{"https://music.apple.com/us/album/1", "https://music.apple.com/us/album/2", "-atmos"},
			wantOk:   true,
		},
		{
			name:     "apple.co short link",
			input:    "https://apple.co/3xyz",
			wantArgs: []string{"https://apple.co/3xyz"},
			wantOk:   true,
		},
		{
			name:     "regular chat message without link",
			input:    "Hey everyone, how is it going?",
			wantArgs: nil,
			wantOk:   false,
		},
		{
			name:     "spotify link ignored",
			input:    "https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT",
			wantArgs: nil,
			wantOk:   false,
		},
		{
			name:     "empty message",
			input:    "",
			wantArgs: nil,
			wantOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotArgs, gotOk := extractDirectDownloadArgs(tt.input)
			if gotOk != tt.wantOk {
				t.Fatalf("extractDirectDownloadArgs(%q) gotOk = %v, wantOk = %v", tt.input, gotOk, tt.wantOk)
			}
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Fatalf("extractDirectDownloadArgs(%q) gotArgs = %v, wantArgs = %v", tt.input, gotArgs, tt.wantArgs)
			}
		})
	}
}
