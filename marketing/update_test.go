package main

import "testing"

func TestInitReturnsEmptyModel(t *testing.T) {
	got, _ := Init()
	if got != (Model{}) {
		t.Fatalf("Init = %+v, want empty Model", got)
	}
}

func TestUpdateLeavesModelUnchanged(t *testing.T) {
	seed, _ := Init()
	next, _ := Update(seed, struct{}{})
	if next != seed {
		t.Fatalf("Update mutated the seed: %+v", next)
	}
}
