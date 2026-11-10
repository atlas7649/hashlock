package main

import (
	"flag"
	"reflect"
	"testing"
)

func TestReorderArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "flags after positional arguments",
			args: []string{"./site", "-o", "out.json"},
			want: []string{"-o", "out.json", "./site"},
		},
		{
			name: "flags before positional arguments",
			args: []string{"-o", "out.json", "./site"},
			want: []string{"-o", "out.json", "./site"},
		},
		{
			name: "inline flag value",
			args: []string{"./site", "-o=out.json"},
			want: []string{"-o=out.json", "./site"},
		},
		{
			name: "boolean flag after positionals",
			args: []string{"./site", "manifest.json", "--json"},
			want: []string{"--json", "./site", "manifest.json"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.String("o", "-", "output")
			fs.Bool("json", false, "json")
			got := reorderArgs(fs, tc.args)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("reorderArgs(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}
