//go:build !js

package main

import (
	"fmt"
	"testing"
)

// fingerprint sums up a run's state, to tell whether a replay ended where the run did.
func fingerprint(w *World) string {
	s := fmt.Sprintf("depth %d at %d,%d HP %d mana %d gold %d kills %d level %d xp %d over %v next %d |",
		w.Depth, w.Player.X, w.Player.Y, w.Player.HP, w.Mana, w.Gold, w.Kills, w.ExpLevel, w.XP, w.Over, w.rng.IntN(1000))
	for _, it := range w.Inventory {
		s += " " + w.ItemName(it)
	}
	for _, m := range w.Monsters {
		s += fmt.Sprintf(" %s@%d,%d:%d", m.Name, m.X, m.Y, m.HP)
	}
	return s
}

func TestSaveAndContinue(t *testing.T) {
	dataDir = t.TempDir()
	tested := 0
	for _, c := range classes {
		var w *World
		for seed := uint64(42); seed < 82; seed++ { // a seed this class survives a stretch of play on
			w = NewGame(seed, c)
			for range 400 { // items and spells included
				if !w.Over && !w.botItems() {
					w.Step(tactician(w))
				}
			}
			if !w.Over {
				break
			}
		}
		if w.Over {
			continue // a dead hero's run isn't saved
		}
		if err := w.Save(); err != nil {
			t.Fatal(err)
		}
		got, err := Continue()
		if err != nil {
			t.Fatal(err)
		}
		if want, have := fingerprint(w), fingerprint(got); want != have {
			t.Fatalf("%s: continuing should replay to the same state\nwant %s\nhave %s", c.Name, want, have)
		}
		if HasSave() {
			t.Fatal("continuing should delete the save, so a run can't be loaded twice")
		}
		tested++
	}
	if tested < len(classes) {
		t.Fatalf("only %d classes survived a stretch of play to be saved; try more seeds", tested)
	}

	writeData(saveFile, []byte(`{"Version": 0, "Seed": 1, "Class": "warrior"}`))
	if _, err := Continue(); err == nil || HasSave() {
		t.Fatal("a save from an older version should be refused and thrown away")
	}
}

func TestScores(t *testing.T) {
	dataDir = t.TempDir()
	w := NewWorld([]string{"###", "#@#", "###"})
	for depth := 1; depth <= maxScores+2; depth++ {
		w.Depth = depth
		if place := w.RecordScore(); place != 0 {
			t.Fatalf("each deeper run should top the list, got place %d", place)
		}
	}
	w.Depth = 1
	ss := LoadScores()
	if len(ss) != maxScores || ss[0].Depth != maxScores+2 || w.RecordScore() != -1 {
		t.Fatalf("the list should keep the best %d, best first, got %d entries", maxScores, len(ss))
	}
}

func TestCauseOfDeath(t *testing.T) {
	w := NewWorld([]string{"####", "#@r#", "####"})
	w.Player.HP = 1
	for !w.Over {
		w.Step(0, 0)
	}
	if w.Cause != "a rat" {
		t.Fatalf("death by a rat should say so, got %q", w.Cause)
	}
}

func TestBossLoot(t *testing.T) {
	w := NewWorld([]string{
		"#####",
		"#...#",
		"#@D.#",
		"#...#",
		"#####",
	})
	w.Depth = 5
	w.damage(w.Monsters[0], 1000)
	if w.Level[2][2] != '>' || len(w.Floor) != 3 || len(w.ItemsAt(2, 2)) != 0 {
		t.Fatalf("a boss should leave 3 things around its stairs, not on them, got %d", len(w.Floor))
	}
}
