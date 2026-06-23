package controlplane

import (
	"errors"
	"testing"
)

func TestFenceStoreRejectsStaleAndConflictingIndexesAndPersists(t *testing.T) {
	directory := t.TempDir()
	store, err := OpenFenceStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	first := OperationDigest("POST", "/cluster/promote", []byte(`{"slot":1}`), "7")
	replay, release, err := store.Begin(7, first)
	if err != nil || replay {
		t.Fatalf("first acceptance: replay=%v err=%v", replay, err)
	}
	release()

	reopened, err := OpenFenceStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	replay, release, err = reopened.Begin(7, first)
	if err != nil || !replay {
		t.Fatalf("exact replay: replay=%v err=%v", replay, err)
	}
	release()
	conflict := OperationDigest("POST", "/cluster/promote", []byte(`{"slot":2}`), "7")
	if _, _, err := reopened.Begin(7, conflict); !errors.Is(err, ErrControlIndexConflict) {
		t.Fatalf("conflict error = %v", err)
	}
	if _, _, err := reopened.Begin(6, first); !errors.Is(err, ErrStaleControlIndex) {
		t.Fatalf("stale error = %v", err)
	}
}

func TestRequestSignature(t *testing.T) {
	secret := []byte("secret")
	body := []byte(`{"slot":3}`)
	signature := Sign(secret, "POST", "/cluster/promote", body, "9")
	if !Verify(secret, "POST", "/cluster/promote", body, "9", signature) {
		t.Fatal("valid signature was rejected")
	}
	if Verify(secret, "POST", "/cluster/promote", []byte(`{"slot":4}`), "9", signature) {
		t.Fatal("forged body was accepted")
	}
}
