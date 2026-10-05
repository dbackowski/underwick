package main

import (
	"math/rand/v2"
	"strings"
	"testing"
)

func TestDoors(t *testing.T) {
	w := NewWorld([]string{
		"#######",
		"#@+..r#",
		"#######",
	})
	if w.Visible[1][3] {
		t.Fatal("a closed door should block sight")
	}
	w.Step(1, 0)
	if w.Level[1][2] != '/' || w.Player.X != 1 {
		t.Fatal("walking into a door should open it, without going through")
	}
	if !w.Visible[1][3] {
		t.Fatal("an open door should let the hero see through")
	}

	// A locked door needs its own key, which the lock keeps.
	w = NewWorld([]string{
		"#####",
		"#@1.#",
		"#####",
	})
	w.Step(1, 0)
	if w.Level[1][2] != '1' {
		t.Fatal("an iron door should stay locked without a gold key")
	}
	w.Inventory = append(w.Inventory, &Item{ItemKind: blueKey}, &Item{ItemKind: goldKey})
	w.Step(1, 0)
	if w.Level[1][2] != '4' || len(w.Inventory) != 2 || w.Inventory[1].ItemKind != blueKey {
		t.Fatalf("the gold key should unlock the iron door and be used up, got %q, %d items", w.Level[1], len(w.Inventory))
	}
}

func TestChest(t *testing.T) {
	w := NewWorld([]string{
		"####",
		"#@&#",
		"####",
	})
	w.Step(1, 0)
	if w.Level[1][2] != '0' || len(w.ItemsAt(2, 1)) != 3 {
		t.Fatalf("opening the chest should spill 3 items, got %d", len(w.ItemsAt(2, 1)))
	}
}

func TestHazards(t *testing.T) {
	// Lava asks before the hero steps in, then burns.
	w := NewWorld([]string{
		"#####",
		"#@=.#",
		"#####",
	})
	w.Player.HP = w.Player.MaxHP
	w.Step(1, 0)
	if w.Player.X != 1 || w.Player.HP != w.Player.MaxHP {
		t.Fatal("the first step toward lava should only warn")
	}
	w.Step(1, 0)
	if w.Player.X != 2 || w.Player.HP != w.Player.MaxHP-11 {
		t.Fatalf("the second step should go in and burn for 11 on floor 1, got x=%d HP %d", w.Player.X, w.Player.HP)
	}

	// Acid burns every turn the hero stands in it.
	w = NewWorld([]string{
		"####",
		"#@%#",
		"####",
	})
	w.Step(1, 0)
	w.Step(0, 0)
	if w.Player.HP != w.Player.MaxHP-6 {
		t.Fatalf("two turns in acid should burn 6, got HP %d", w.Player.HP)
	}

	// Wading gives the monsters a second turn.
	w = NewWorld([]string{
		"#########",
		"#@~....z#",
		"#########",
	})
	zombie := w.Monsters[0]
	zombie.hunting, zombie.goalX, zombie.goalY = true, 1, 1
	w.Step(1, 0)
	if zombie.X != 5 {
		t.Fatalf("the zombie should get two moves while the hero wades, got to %d", zombie.X)
	}

	// A pit drops the hero to the next floor.
	w = NewWorld([]string{
		"####",
		"#@^#",
		"####",
	})
	w.Step(1, 0)
	if w.Depth != 2 {
		t.Fatalf("falling through a pit should land on floor 2, got %d", w.Depth)
	}
}

func TestMonstersAvoidHazards(t *testing.T) {
	w := NewWorld([]string{
		"#######",
		"#.....#",
		"#@=~%o#",
		"#######",
	})
	orc := w.Monsters[0]
	orc.hunting, orc.goalX, orc.goalY = true, 1, 2
	w.Step(0, 0)
	if orc.X != 5 || orc.Y != 1 {
		t.Fatalf("the orc should go around the pool, got %d,%d", orc.X, orc.Y)
	}
}

// Every generated floor must let the hero reach the stairs (or boss) and every key without crossing
// a hazard or a locked door, and every locked door must have its key on the floor.
func TestGeneratedFloorsArePlayable(t *testing.T) {
	for seed := range uint64(200) {
		rng := rand.New(rand.NewPCG(seed, 0))
		for depth := 1; depth <= 2*bossEvery; depth++ {
			var boss byte
			if depth%bossEvery == 0 {
				boss = 'D'
			}
			level := generate(rng, depth, boss)
			all := strings.Join(level, "")
			fail := func(why string) {
				t.Fatalf("seed %d depth %d: %s\n%s", seed, depth, why, strings.Join(level, "\n"))
			}
			if strings.Count(all, "1") != strings.Count(all, "(") || strings.Count(all, "2") != strings.Count(all, ")") {
				fail("a locked door without its key")
			}
			if boss != 0 && strings.Contains(all, "^") {
				fail("a pit on a boss floor")
			}
			start := strings.IndexByte(all, '@')
			seen := map[int]bool{start: true}
			for queue := []int{start}; len(queue) > 0; queue = queue[1:] {
				for _, n := range []int{queue[0] - 1, queue[0] + 1, queue[0] - mapW, queue[0] + mapW} {
					if c := all[n]; !seen[n] && !hazard(c) && !strings.ContainsRune("#12&", rune(c)) {
						seen[n] = true
						queue = append(queue, n)
					}
				}
			}
			goal := strings.IndexAny(all, ">D")
			if !seen[goal] {
				fail("the stairs are out of reach")
			}
			for i, c := range all {
				if (c == '(' || c == ')') && !seen[i] {
					fail("a key is out of reach")
				}
				if c == '$' && !seen[i] {
					fail("the merchant is out of reach")
				}
			}
			if want := shopFloor(depth); want != (strings.Count(all, "$") == 1) || strings.Count(all, "$") > 1 {
				fail("a shop floor should have one merchant, and others none")
			}
		}
	}
}

func TestWanderers(t *testing.T) {
	w := NewWorld([]string{
		"##################",
		"#@...............#",
		"#.......#........#",
		"#.......#........#",
		"##################",
	})
	for range wanderEvery - 1 {
		w.Step(0, 0)
	}
	if len(w.Monsters) != 0 {
		t.Fatal("no monster should wander in before its time")
	}
	w.Step(0, 0)
	if len(w.Monsters) != 1 {
		t.Fatalf("a monster should wander in after %d turns", wanderEvery)
	}
	m := w.Monsters[0]
	if !m.hunting || abs(m.X-w.Player.X)+abs(m.Y-w.Player.Y) < 8 {
		t.Fatalf("the wanderer should arrive at least 8 tiles off, already hunting, got %+v", m)
	}
}
