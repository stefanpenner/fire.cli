package loadsm

import "testing"

func TestApply_DisplayGuard(t *testing.T) {
	if !Apply(2, 2, 5, 5) {
		t.Fatal("current view + latest gen should apply")
	}
	if Apply(1, 2, 5, 5) {
		t.Fatal("wrong view must not display")
	}
	if Apply(2, 2, 4, 5) {
		t.Fatal("superseded generation must not display")
	}
}

func TestFresh_StoreGuard(t *testing.T) {
	if !Fresh(5, 5) {
		t.Fatal("latest generation is fresh (cacheable off-view)")
	}
	if Fresh(4, 5) {
		t.Fatal("superseded generation is not fresh")
	}
}
