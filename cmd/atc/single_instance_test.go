package main

import (
	"fmt"
	"os"
	"runtime"
	"testing"
)

func TestInstanceKeySeparatesRoles(t *testing.T) {
	srs := "localhost:5008"
	keys := map[string]string{
		"batumi tower":  instanceKey("tower", "UGSB", "caucasus", "blue", srs),
		"deckboss":      instanceKey("deckboss", "UGSB", "caucasus", "blue", srs),
		"blue command":  instanceKey("command-only", "OMDM", "caucasus", "blue", srs),
		"red command":   instanceKey("command-only", "OMDM", "caucasus", "red", srs),
		"syria command": instanceKey("command-only", "OMDM", "syria", "blue", srs),
		"other server":  instanceKey("tower", "UGSB", "caucasus", "blue", "localhost:5002"),
	}
	seen := map[string]string{}
	for role, k := range keys {
		if other, dup := seen[k]; dup {
			t.Errorf("%s and %s share key %q", role, other, k)
		}
		seen[k] = role
	}
	if instanceKey("tower", "UGKS", "caucasus", "blue", srs) != instanceKey("tower", "ugks", "Caucasus", "Blue", srs) {
		t.Error("the same tower keyed two ways")
	}
}

// Day 2026-10-23: a second Senaki Tower must not start beside the first.
func TestSecondCopyRefused(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the lock is Windows-only")
	}
	key := instanceKey("tower", "UGKS", "caucasus", "blue", fmt.Sprintf("test-%d", os.Getpid()))
	if ok, err := claimInstance(key); !ok || err != nil {
		t.Fatalf("first copy refused: %v %v", ok, err)
	}
	if ok, err := claimInstance(key); ok || err != nil {
		t.Fatalf("second copy allowed: %v %v", ok, err)
	}
	if ok, _ := claimInstance(key + "-kutaisi"); !ok {
		t.Fatal("a different role was refused")
	}
}
