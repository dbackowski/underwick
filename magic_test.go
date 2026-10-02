package main

import (
	"math/rand/v2"
	"slices"
	"testing"
)

// runAs starts a run as a class on a hand-drawn level.
func runAs(c *Class, level ...string) *World {
	w := newRun(rand.New(rand.NewPCG(1, 0)), c)
	w.load(level)
	return w
}

func classNamed(name string) *Class {
	return classes[slices.IndexFunc(classes, func(c *Class) bool { return c.Name == name })]
}

func TestClasses(t *testing.T) {
	for _, c := range classes {
		w := runAs(c, "###", "#@#", "###")
		if len(w.Inventory) != len(c.Start) || len(w.Spells) != len(c.Spells) || w.Mana != c.Mana {
			t.Fatalf("%s should start with %v, spells %v and %d mana", c.Name, c.Start, c.Spells, c.Mana)
		}
		for _, it := range w.Inventory {
			if !it.Worn {
				t.Fatalf("%s should start with its %s in use", c.Name, it.Name)
			}
		}
	}
}

func TestBow(t *testing.T) {
	w := runAs(classNamed("archer"), "#######", "#@...o#", "#######")
	orc := w.Monsters[0]
	orc.MaxHP, orc.HP = 1000, 1000
	if w.Player.Range != 5 {
		t.Fatalf("an archer with a bow should reach 5 tiles, got %d", w.Player.Range)
	}
	for range 20 {
		w.Player.HP = w.Player.MaxHP
		w.Step(1, 0)
	}
	if orc.HP == 1000 || w.Player.X != 1 {
		t.Fatal("moving toward a monster in line should shoot it, not walk")
	}
}

func TestSpells(t *testing.T) {
	w := runAs(classNamed("mage"), "#######", "#@...o#", "#######")
	orc := w.Monsters[0]
	w.Cast(0, 1, 0)
	if orc.HP >= orc.MaxHP || w.Mana != 15-3 || len(w.Shots) != 1 {
		t.Fatalf("fire bolt should hit the orc for 3 mana, got orc %d/%d, mana %d", orc.HP, orc.MaxHP, w.Mana)
	}

	w = runAs(classNamed("cleric"), "###", "#@#", "###")
	w.Player.HP, w.Player.Poison = 3, 5
	w.Cast(0, 0, 0)
	if w.Player.HP <= 3 || w.Player.Poison != 0 {
		t.Fatalf("heal should restore HP and cure poison, got HP %d poison %d", w.Player.HP, w.Player.Poison)
	}

	w = runAs(classNamed("mage"), "###", "#@#", "###")
	w.Mana = 1
	w.Cast(0, 1, 0)
	if w.Mana != 1 || w.Log[0] != "You don't have the mana for that." {
		t.Fatal("casting without the mana should do nothing")
	}
}

func TestSleep(t *testing.T) {
	w := NewWorld([]string{
		"######",
		"#@.o.#",
		"######",
	})
	orc := w.Monsters[0]
	w.lull(8)
	w.Step(0, 0)
	w.Step(0, 0)
	if orc.X != 3 || orc.Sleep == 0 {
		t.Fatal("a sleeping orc should stay put")
	}
	if hitChance(w.Player, orc) != 100 {
		t.Fatal("a sleeper should always be hit")
	}
	w.damage(orc, 1)
	if orc.Sleep != 0 {
		t.Fatal("being hit should wake it")
	}

	// A monster asleep on its own wakes only by chance, and only once it could see the hero.
	w = NewWorld([]string{
		"#########",
		"#@..#..o#",
		"#########",
	})
	w.Monsters[0].Sleep = -1
	for range 50 {
		w.Step(0, 0)
	}
	if w.Monsters[0].Sleep != -1 {
		t.Fatal("a sleeper that can't see the hero should never wake")
	}
}

func TestStatuses(t *testing.T) {
	w := NewWorld([]string{
		"###",
		"#@#",
		"###",
	})
	w.Player.Poison = 3
	hp := w.Player.HP
	for range 5 {
		w.Step(0, 0)
	}
	if w.Player.Poison != 0 || w.Player.HP != hp-3 {
		t.Fatalf("3 turns of poison should cost 3 HP, got %d", hp-w.Player.HP)
	}
}

func TestTomesAndMana(t *testing.T) {
	w := NewWorld([]string{
		"###",
		"#@#",
		"###",
	})
	w.Inventory = append(w.Inventory, &Item{ItemKind: kindNamed("tome of heal")})
	w.Use(1)
	if len(w.Spells) != 1 || w.Spells[0].Name != "heal" || w.MaxMana == 0 {
		t.Fatal("a warrior reading a tome should learn its spell and gain some mana")
	}
	w.Mana = 0
	for range manaEvery * 2 {
		w.Step(0, 0)
	}
	if w.Mana != 2 {
		t.Fatalf("mana should come back 1 every %d turns, got %d", manaEvery, w.Mana)
	}
	max := w.MaxMana
	w.gainXP(xpFor(1))
	if w.MaxMana != max+2 {
		t.Fatal("levelling should raise a caster's mana")
	}
}
