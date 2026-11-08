package main

import "testing"

func TestRequestComesFirst(t *testing.T) {
	if !requestComesFirst(3, 2, 4, 1) {
		t.Fatal("an earlier timestamp should come first")
	}

	if !requestComesFirst(3, 1, 3, 2) {
		t.Fatal("the lower node ID should win when timestamps are equal")
	}

	if requestComesFirst(3, 2, 3, 1) {
		t.Fatal("the higher node ID should not win when timestamps are equal")
	}
}