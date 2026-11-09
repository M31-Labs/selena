package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTakeFirstInstanceLocation(t *testing.T) {
	rest, n, err := takeFirstInstanceLocation([]string{"emit", "wgsl", "--first-instance-location", "9", "m.sel"})
	if err != nil || n != 9 || !reflect.DeepEqual(rest, []string{"emit", "wgsl", "m.sel"}) {
		t.Fatalf("rest %v n %d err %v", rest, n, err)
	}
	rest, n, err = takeFirstInstanceLocation([]string{"inspect", "m.sel", "--first-instance-location=3"})
	if err != nil || n != 3 || !reflect.DeepEqual(rest, []string{"inspect", "m.sel"}) {
		t.Fatalf("rest %v n %d err %v", rest, n, err)
	}
	for _, bad := range [][]string{{"emit", "--first-instance-location"}, {"emit", "--first-instance-location=x"}, {"emit", "--first-instance-location", "-1"}} {
		if _, _, err := takeFirstInstanceLocation(bad); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
}

// selena artifact honours --first-instance-location in both dash forms and
// writes the shifted locations into the file; subcommands that cannot use the
// flag fail instead of ignoring it.
func TestArtifactHonoursFirstInstanceLocation(t *testing.T) {
	src := filepath.Join("..", "..", "testdata", "conformance", "instance-quad.sel")
	for _, flagArgs := range [][]string{
		{"-first-instance-location", "9"},
		{"--first-instance-location", "9"},
		{"--first-instance-location=9"},
	} {
		out := filepath.Join(t.TempDir(), "a.json")
		args := append([]string{"artifact", "-targets", "wgsl", "-o", out}, flagArgs...)
		args = append(args, src)
		if err := run(args); err != nil {
			t.Fatalf("%v: %v", flagArgs, err)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "@location(9)") {
			t.Errorf("%v: artifact lacks @location(9)", flagArgs)
		}
		if strings.Contains(string(data), "@location(0) rect") {
			t.Errorf("%v: artifact still places rect at 0", flagArgs)
		}
	}
	// The default is unchanged.
	out := filepath.Join(t.TempDir(), "a.json")
	if err := run([]string{"artifact", "-targets", "wgsl", "-o", out, src}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	if strings.Contains(string(data), "@location(9)") {
		t.Error("default artifact uses location 9")
	}
}

func TestFirstInstanceLocationRejectedElsewhere(t *testing.T) {
	src := filepath.Join("..", "..", "examples", "tinted.sel")
	for _, args := range [][]string{
		{"check", "--first-instance-location", "9", src},
		{"check", "-first-instance-location=9", src},
		{"graph", "--first-instance-location=9", src},
		{"doctor", "--first-instance-location", "9"},
	} {
		err := run(args)
		if err == nil || !strings.Contains(err.Error(), "first-instance-location") {
			t.Errorf("%v: err = %v, want a first-instance-location error", args, err)
		}
	}
}
