package main

import "testing"

// One run through the core loop: break a rat, possess it, let the body rot, fade as a spark.
func TestPossessionLoop(t *testing.T) {
	w := NewWorld([]string{
		"#####",
		"#@r.#",
		"#####",
	})

	w.Step(1, 0) // goblin warrior hits the rat for 2: 3 HP -> 1, broken
	if rat := w.Monsters[0]; rat.HP != 1 || !rat.Broken() {
		t.Fatalf("rat should be broken at 1 HP, got %+v", rat)
	}

	w.Step(1, 0) // bump the broken rat: possess it
	if w.Player.Kind != "rat" || w.Player.HP != 3 || len(w.Monsters) != 0 {
		t.Fatalf("should possess a healed rat, got %+v, monsters %d", w.Player, len(w.Monsters))
	}

	w.Player.HP = 1
	for range decayEvery {
		w.Step(0, 0)
	}
	if !w.IsSpark() || w.SparkLeft != sparkTurns {
		t.Fatalf("rotted body should eject the spark, got %+v", w.Player)
	}

	for range sparkTurns {
		w.Step(0, 0)
	}
	if !w.Over {
		t.Fatal("spark without a body should fade")
	}
}
