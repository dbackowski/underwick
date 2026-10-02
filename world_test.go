package main

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

func TestCombat(t *testing.T) {
	a, d := newEntity(hero, 0, 0), newEntity(kinds['o'], 0, 0)
	if got := hitChance(a, d); got != 70 { // 70 + 5 * (2 accuracy - 2 armour)
		t.Fatalf("hero vs orc should hit 70%% of the time, got %d", got)
	}
	a.Atk, a.Def, d.Def = 100, 100, 0
	if hitChance(a, d) != 95 || hitChance(d, a) != 5 {
		t.Fatal("hit chance should stay between 5% and 95%")
	}

	// Hitting until the rat dies: every hit deals 1 to the hero's damage (sword included), and the kill
	// is counted.
	w := NewWorld([]string{
		"####",
		"#@r#",
		"####",
	})
	rat := w.Monsters[0]
	rat.MaxHP, rat.HP = 1000, 1000
	for range 200 {
		before := rat.HP
		w.Player.HP = w.Player.MaxHP // keep it alive for the test
		w.Step(1, 0)
		if lost := before - rat.HP; lost < 0 || lost > w.Player.Dmg {
			t.Fatalf("a hit should deal 1 to %d, dealt %d", w.Player.Dmg, lost)
		}
	}
	rat.HP = 1
	for i := 0; len(w.Monsters) > 0; i++ {
		if i > 100 {
			t.Fatal("the hero never lands a hit")
		}
		w.Player.HP = w.Player.MaxHP // keep it alive for the test
		w.Step(1, 0)
	}
	if w.Kills != 1 || !slices.Contains(w.Log, "The rat dies.") {
		t.Fatalf("the kill should be counted and logged, got %d kills, log %q", w.Kills, w.Log)
	}
}

func TestRegenAndDeath(t *testing.T) {
	w := NewWorld([]string{
		"####",
		"#@.#",
		"####",
	})
	w.Player.HP = 10
	for range regenEvery {
		w.Step(0, 0)
	}
	if w.Player.HP != 11 {
		t.Fatalf("the hero should regain 1 HP every %d turns, got %d", regenEvery, w.Player.HP)
	}
	w.damage(w.Player, 100)
	if !w.Over || w.Score() != 100 {
		t.Fatalf("death should end the run with depth 1 scoring 100, got over=%v score=%d", w.Over, w.Score())
	}
}

func TestStairs(t *testing.T) {
	w := NewWorld([]string{
		"####",
		"#@>#",
		"####",
	})
	body := w.Player
	w.Step(1, 0)
	if w.Depth != 2 || w.Player != body || w.Level[body.Y][body.X] != '.' {
		t.Fatalf("stairs should take the hero to the start of floor 2, got depth %d at %d,%d", w.Depth, body.X, body.Y)
	}
}

func TestBosses(t *testing.T) {
	w := NewGame(7, classes[0])
	if len(w.bosses) != len(bossKinds) {
		t.Fatalf("a run should meet all %d bosses in turn, got %q", len(bossKinds), w.bosses)
	}
	w.Depth = bossEvery - 1
	w.descend()
	var boss *Entity
	for _, m := range w.Monsters {
		if m.Boss {
			boss = m
		}
	}
	if boss == nil || boss.Name != kinds[w.bosses[0]].Name || strings.Contains(strings.Join(w.Level, ""), ">") {
		t.Fatalf("floor %d should hold the run's first boss and no open stairs", bossEvery)
	}

	// Killing a boss opens the stairs where it stood.
	w = NewWorld([]string{
		"#####",
		"#@D.#",
		"#####",
	})
	w.damage(w.Monsters[0], 1000)
	if len(w.Monsters) != 0 || w.Level[1][2] != '>' {
		t.Fatalf("killing the boss should open the stairs, got %q", w.Level[1])
	}
}

// Every generated floor must be walled in, have one start and one stairs, and be fully connected.
func TestGenerate(t *testing.T) {
	for seed := range uint64(200) {
		rng := rand.New(rand.NewPCG(seed, 0))
		for depth := 1; depth <= 2*bossEvery; depth++ {
			var boss byte
			stairs := 1
			if depth%bossEvery == 0 {
				boss, stairs = 'D', 0
			}
			level := generate(rng, depth, boss)
			all := strings.Join(level, "")
			if len(level) != mapH || len(all) != mapW*mapH ||
				strings.Count(all, "@") != 1 || strings.Count(all, ">") != stairs || strings.Count(all, "D") != 1-stairs ||
				level[0] != strings.Repeat("#", mapW) || level[mapH-1] != level[0] {
				t.Fatalf("seed %d depth %d: malformed floor\n%s", seed, depth, strings.Join(level, "\n"))
			}
			for _, c := range all { // every monster belongs on this floor
				if k, ok := kinds[byte(c)]; ok && !k.Boss && !slices.Contains(spawnable(depth), byte(c)) {
					t.Fatalf("seed %d depth %d: %s doesn't belong here", seed, depth, k.Name)
				}
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

func TestFOV(t *testing.T) {
	w := NewWorld([]string{
		"#########",
		"#@..#...#",
		"#########",
	})
	if !w.Visible[1][3] || !w.Visible[1][4] || w.Visible[1][5] {
		t.Fatalf("should see up to and including the wall, not past it, got %v", w.Visible[1])
	}
	w.Player.X = 7 // as if it walked around
	w.updateFOV()
	if w.Visible[1][2] || !w.Seen[1][2] {
		t.Fatal("tiles out of sight should stay remembered")
	}
}

func TestMonstersByDepth(t *testing.T) {
	names := func(depth int) (ns []string) {
		for _, c := range spawnable(depth) {
			ns = append(ns, kinds[c].Name)
		}
		return ns
	}
	if got := names(1); !slices.Contains(got, "rat") || slices.Contains(got, "orc") {
		t.Fatalf("floor 1 should have rats but no orcs, got %v", got)
	}
	if got := names(9); slices.Contains(got, "rat") || !slices.Contains(got, "orc") {
		t.Fatalf("floor 9 should have orcs but no rats, got %v", got)
	}
	if deep := names(100); len(deep) < 5 || !slices.Contains(deep, "demon") {
		t.Fatalf("the deep should keep a mix of the deepest monsters, got %v", deep)
	}

	rat := kinds['r'].at(9) // two steps of 4 floors below floor 1
	if rat.MaxHP != 6 || rat.Atk != 2 || rat.Dmg != 4 || rat.XP != 4 {
		t.Fatalf("a rat on floor 9 should be stronger, got %+v", rat)
	}
}

func TestLevels(t *testing.T) {
	w := NewWorld([]string{
		"###",
		"#@#",
		"###",
	})
	was := w.Player.Kind
	w.gainXP(xpFor(1) + xpFor(2)) // straight to level 3
	p := w.Player
	if w.ExpLevel != 3 || w.XP != 0 || p.MaxHP != was.MaxHP+10 || p.Atk != was.Atk+2 || p.Dmg != was.Dmg+1 || p.Def != was.Def+1 {
		t.Fatalf("level 3 should add 10 HP, 2 accuracy, 1 damage and 1 armour, got level %d %+v", w.ExpLevel, p.Kind)
	}
}
