package svggen

import (
	"testing"
)

func TestParsePathDataSupportsClosedGrammar(t *testing.T) {
	source := "M1 2 l3 4 H9 v2 C1 2 3 4 5 6 s2 3 4 5 Q1 2 3 4 t2 3 A5 6 20 0 1 9 10 z"
	commands, err := ParsePathData(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) < 10 || commands[0].Verb != PathMove || commands[len(commands)-1].Verb != PathClose {
		t.Fatalf("commands = %+v", commands)
	}
	for _, command := range commands {
		if command.Verb == PathCubic && command.P2.X == 9 && command.P2.Y == 10 {
			return
		}
	}
	t.Fatal("arc was not converted to cubic commands")
}

func TestParsePathDataRejectsMalformedAndExtremeData(t *testing.T) {
	for _, source := range []string{"", "M", "M 1 nope", "M0 0 A1 1 0 2 0 1 1", "M 1e100 0"} {
		if _, err := ParsePathData(source); err == nil {
			t.Fatalf("ParsePathData(%q) returned nil error", source)
		}
	}
}

func FuzzParsePathData(f *testing.F) {
	f.Add("M0 0 C1 2 3 4 5 6 Z")
	f.Add("M0 0 A10 10 0 1 1 20 0")
	f.Add("M1e999 0")
	f.Fuzz(func(t *testing.T, source string) {
		_, _ = ParsePathData(source)
	})
}
