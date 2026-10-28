package main

import "testing"

func TestSingleInstanceRefusesSecondLock(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	if !singleInstance() {
		t.Fatal("the first launch should own the lock")
	}
	if singleInstance() {
		t.Fatal("a second launch should be refused")
	}
}
