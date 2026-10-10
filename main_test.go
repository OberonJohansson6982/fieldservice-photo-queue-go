package main

import "testing"

func TestShouldQueueHeavyPhoto(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int
		want bool
	}{{"small", 100, false}, {"boundary", 120000, false}, {"heavy", 120001, true}} {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldQueue(string(make([]byte, tc.size)))
			if got != tc.want {
				t.Fatalf("size %d: got %v want %v", tc.size, got, tc.want)
			}
		})
	}
}
