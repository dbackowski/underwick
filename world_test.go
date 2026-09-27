package main

import (
	"math/rand/v2"
	"strings"
	"testing"
)

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

	// Orc: rots twice as fast as the others.
	w = NewWorld(level)
	w.Player.Kind = kinds['o']
	w.Player.HP = 10
	for range kinds['a'].Decay / 2 {
		w.Step(0, 0)
	}
	if w.Player.HP != 9 {
		t.Fatalf("orc body should lose 1 HP in half the usual time, got %d", w.Player.HP)
	}
}

// Every generated floor must be walled in, have one start and one stairs, and be fully connected.
func TestGenerate(t *testing.T) {
	for seed := range uint64(200) {
		rng := rand.New(rand.NewPCG(seed, 0))
		for depth := 1; depth <= floors; depth++ {
			var boss byte
			stairs := 1
			if depth%3 == 0 {
				boss, stairs = 'D', 0
			}
			level := generate(rng, depth, boss)
			all := strings.Join(level, "")
			if len(level) != mapH || len(all) != mapW*mapH ||
				strings.Count(all, "@") != 1 || strings.Count(all, ">") != stairs || strings.Count(all, "D") != 1-stairs ||
				level[0] != strings.Repeat("#", mapW) || level[mapH-1] != level[0] {
				t.Fatalf("seed %d depth %d: malformed floor\n%s", seed, depth, strings.Join(level, "\n"))
			}
			if boss != 0 { // minions stand within 2 steps of the boss
				b, near := strings.IndexByte(all, boss), 0
				for i, c := range all {
					if _, ok := kinds[byte(c)]; ok && i != b && abs(i%mapW-b%mapW)+abs(i/mapW-b/mapW) <= 2 {
						near++
					}
				}
				if near < minions {
					t.Fatalf("seed %d depth %d: %d monsters near the boss\n%s", seed, depth, near, strings.Join(level, "\n"))
				}
			}
			for _, row := range level {
				if row[0] != '#' || row[mapW-1] != '#' {
					t.Fatalf("seed %d depth %d: open side wall\n%s", seed, depth, strings.Join(level, "\n"))
				}
			}

			// Flood fill from the start must reach every open tile.
			start := strings.IndexByte(all, '@')
			seen := map[int]bool{start: true}
			for queue := []int{start}; len(queue) > 0; queue = queue[1:] {
				for _, n := range []int{queue[0] - 1, queue[0] + 1, queue[0] - mapW, queue[0] + mapW} {
					if all[n] != '#' && !seen[n] {
						seen[n] = true
						queue = append(queue, n)
					}
				}
			}
			if open := len(all) - strings.Count(all, "#"); len(seen) != open {
				t.Fatalf("seed %d depth %d: reached %d of %d open tiles\n%s", seed, depth, len(seen), open, strings.Join(level, "\n"))
			}
		}
	}
}

func TestStairs(t *testing.T) {
	w := NewWorld([]string{
		"####",
		"#@>#",
		"####",
	})
	w.rng, w.Depth = rand.New(rand.NewPCG(1, 0)), 1
	body := w.Player
	w.Step(1, 0)
	if w.Depth != 2 || w.Player != body || w.Level[body.Y][body.X] != '@' {
		t.Fatalf("stairs should take the same body to the start of floor 2, got depth %d at %d,%d", w.Depth, body.X, body.Y)
	}

	w = NewWorld([]string{
		"####",
		"#@>#",
		"####",
	})
	w.Depth = floors
	w.Step(1, 0)
	if !w.Over || !w.Won {
		t.Fatal("stairs on the last floor should win the run")
	}
}

func TestSightAndPaths(t *testing.T) {
	// An orc behind a wall doesn't notice the player, though it is well within range.
	w := NewWorld([]string{
		"#######",
		"#@.#.o#",
		"#######",
	})
	w.Step(0, 0)
	if orc := w.Monsters[0]; orc.X != 5 || orc.hunting {
		t.Fatalf("orc without line of sight should stay put, got %+v", orc)
	}

	// Once hunting, a monster walks around a wall even when that means first moving away.
	w = NewWorld([]string{
		"#######",
		"#@....#",
		"#####.#",
		"#o....#",
		"#######",
	})
	orc := w.Monsters[0]
	orc.hunting, orc.goalX, orc.goalY = true, 1, 1
	w.Step(0, 0)
	if orc.X != 2 || orc.Y != 3 {
		t.Fatalf("orc should head right, around the wall, got %d,%d", orc.X, orc.Y)
	}
}

func TestBurstAndCrumble(t *testing.T) {
	// A body killed in melee breaks its killer, which the spark can take right away.
	w := NewWorld([]string{
		"#####",
		"#@o.#",
		"#####",
	})
	w.Player.HP = 1
	w.Step(0, 0) // the orc kills the archer
	orc := w.Monsters[0]
	if !w.IsSpark() || !orc.Broken() {
		t.Fatalf("spark should tear free and break the orc, got player %+v orc %+v", w.Player, orc)
	}
	w.Step(1, 0)
	if w.Player != orc {
		t.Fatalf("spark should possess the broken orc, got %+v", w.Player)
	}

	// A broken monster nobody takes falls apart.
	w = NewWorld([]string{
		"#######",
		"#@...r#",
		"#######",
	})
	w.Monsters[0].HP = 1
	for range crumble + 1 {
		w.Step(0, 0)
	}
	if len(w.Monsters) != 0 {
		t.Fatalf("broken rat should crumble after %d turns", crumble)
	}
}

func TestBosses(t *testing.T) {
	w := NewGame(7)
	if len(w.bosses) != 3 || w.bosses[0] == w.bosses[1] || w.bosses[1] == w.bosses[2] || w.bosses[0] == w.bosses[2] {
		t.Fatalf("a run should face 3 different bosses, got %q", w.bosses)
	}

	// A boss is never broken, even at 1 HP, and killing it opens the stairs where it stood.
	w = NewWorld([]string{
		"#####",
		"#@D.#",
		"#####",
	})
	dragon := w.Monsters[0]
	dragon.HP = 1
	if dragon.Broken() {
		t.Fatal("a boss should never be broken")
	}
	w.Step(1, 0)
	if len(w.Monsters) != 0 || w.Level[1][2] != '>' {
		t.Fatalf("killing the boss should open the stairs, got %q", w.Level[1])
	}

	// The spark's burst can't break a boss, so it scorches it for a quarter of its HP.
	w = NewWorld([]string{
		"#####",
		"#@C.#",
		"#####",
	})
	w.Player.HP = 1
	w.Step(0, 0) // the cyclops kills the archer
	if cyclops := w.Monsters[0]; !w.IsSpark() || cyclops.Broken() || cyclops.HP != cyclops.MaxHP-cyclops.MaxHP/4 {
		t.Fatalf("burst should scorch the boss for a quarter of its HP, got %+v", cyclops)
	}
}
