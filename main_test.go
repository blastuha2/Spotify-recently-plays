package main

import (
	"testing"
	"time"
)

func TestParseFrom(t *testing.T) {
	from, err := parseFrom("2026-04-11")
	if err != nil {
		t.Fatalf("parseFrom: %v", err)
	}
	if from.Location() != time.UTC || from.Hour() != 0 || from.Day() != 11 {
		t.Fatalf("unexpected date: %#v", from)
	}
	if _, err := parseFrom("11.04.2026"); err == nil {
		t.Fatal("expected invalid date error")
	}
}

func TestBearerValue(t *testing.T) {
	for input, want := range map[string]string{
		"token":        "token",
		" Bearer abc ": "abc",
		"bearer xyz":   "xyz",
		"":             "",
	} {
		if got := bearerValue(input); got != want {
			t.Errorf("bearerValue(%q) = %q, want %q", input, got, want)
		}
	}
}
