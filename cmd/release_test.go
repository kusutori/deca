package cmd

import (
	"reflect"
	"testing"
)

func TestCandidateTags(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    []string
	}{
		{
			name:    "empty",
			version: "",
			want:    nil,
		},
		{
			name:    "plain",
			version: "1.2.3",
			want:    []string{"1.2.3", "v1.2.3"},
		},
		{
			name:    "with-v",
			version: "v2.0.0",
			want:    []string{"v2.0.0", "2.0.0"},
		},
		{
			name:    "trim-spaces",
			version: " 0.9.5 ",
			want:    []string{"0.9.5", "v0.9.5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := candidateTags(tt.version)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("candidateTags(%q) = %#v, want %#v", tt.version, got, tt.want)
			}
		})
	}
}
