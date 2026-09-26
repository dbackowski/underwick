package main

import "testing"

// One run through the core loop: break a rat, possess it, let the body rot, fade as a spark.
func TestPossessionLoop(t *testing.T) {
	w := NewWorld([]string{
		"#####",
		"#@r.#",
		"#####",
	})

	w.Step(1, 0) // goblin archer hits the adjacent rat for 2: 3 HP -> 1, broken
	if rat := w.Monsters[0]; rat.HP != 1 || !rat.Broken() {
		t.Fatalf("rat should be broken at 1 HP, got %+v", rat)
	}

	w.Step(1, 0) // bump the broken rat: possess it
	if w.Player.Name != "rat" || w.Player.HP != 3 || len(w.Monsters) != 0 {
		t.Fatalf("should possess a healed rat, got %+v, monsters %d", w.Player, len(w.Monsters))
	}

	w.Player.HP = 1
	for range w.Player.Decay {
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

func TestBodies(t *testing.T) {
	level := []string{
		"##########",
		"#@......o#",
		"##########",
	}

	// Archer: range 5, so it walks until the orc is in reach, then shoots.
	w := NewWorld(level)
	w.Step(1, 0) // orc is out of range and sight: archer walks
	if w.Player.X != 2 || len(w.Shots) != 0 {
		t.Fatalf("archer should walk when nothing is in range, got x=%d shots=%d", w.Player.X, len(w.Shots))
	}
	w.Step(1, 0) // orc now at distance 5 after chasing: shoot it
	orc := w.Monsters[0]
	if w.Player.X != 2 || len(w.Shots) != 1 || orc.HP != 8 {
		t.Fatalf("archer should shoot the orc in line, got x=%d shots=%d orc=%+v", w.Player.X, len(w.Shots), orc)
	}

	// Rat: two moves before the monsters act.
	w = NewWorld(level)
	w.Player.Kind = kinds['r']
	w.Step(1, 0)
	if orc := w.Monsters[0]; orc.X != 8 {
		t.Fatalf("monsters should wait for the rat's second move, orc at %d", orc.X)
	}
	w.Step(1, 0)
	if w.Player.X != 3 || w.Monsters[0].X != 7 {
		t.Fatalf("rat should be at 3 and orc chasing to 7, got rat=%d orc=%d", w.Player.X, w.Monsters[0].X)
	}

	// Orc: rots every 2 turns instead of 4.
	w = NewWorld(level)
	w.Player.Kind = kinds['o']
	w.Player.HP = 10
	w.Step(0, 0)
	w.Step(0, 0)
	if w.Player.HP != 9 {
		t.Fatalf("orc body should lose 1 HP after 2 turns, got %d", w.Player.HP)
	}
}
