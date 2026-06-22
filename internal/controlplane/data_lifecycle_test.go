package controlplane

import "testing"

func TestDataNodeInitializationMarksLaterBootsAsReturning(t *testing.T) {
	dir := t.TempDir()
	returning, err := MarkDataNodeInitialized(dir)
	if err != nil || returning {
		t.Fatalf("first boot: returning=%v err=%v", returning, err)
	}
	returning, err = MarkDataNodeInitialized(dir)
	if err != nil || !returning {
		t.Fatalf("later boot: returning=%v err=%v", returning, err)
	}
}
