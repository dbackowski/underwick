package main

import (
	"slices"
	"strings"
	"testing"
)

func roomWith(items ...*Item) *World {
	w := NewWorld([]string{
		"#####",
		"#@..#",
		"#####",
	})
	for _, it := range items {
		it.X, it.Y = w.Player.X, w.Player.Y
		w.Floor = append(w.Floor, it)
	}
	return w
}

func TestWearing(t *testing.T) {
	w := roomWith(&Item{ItemKind: kindNamed("axe"), Plus: 1}, &Item{ItemKind: kindNamed("helm")})
	if p := w.Player; p.Dmg != hero.Dmg+4 || !w.Inventory[0].Worn {
		t.Fatalf("the hero should start wielding a sword, got damage %d", p.Dmg)
	}
	w.PickUp() // the helm, on top
	w.PickUp() // the axe
	if len(w.Inventory) != 3 || len(w.Floor) != 0 {
		t.Fatalf("both items should be carried, got %d carried, %d on the floor", len(w.Inventory), len(w.Floor))
	}
	w.Use(2) // wield the axe +1, which puts the sword away
	w.Use(1) // put on the helm
	p := w.Player
	if w.Inventory[0].Worn || p.Dmg != hero.Dmg+6+1 || p.Atk != hero.Atk-1 || p.Def != hero.Def+1 {
		t.Fatalf("axe +1 and helm should give damage +7, accuracy -1, armour +1, got %+v", p.Kind)
	}
	w.Drop(2)
	if p.Dmg != hero.Dmg || len(w.ItemsAt(p.X, p.Y)) != 1 {
		t.Fatalf("dropping the axe should leave the hero unarmed and the axe on the floor, got damage %d", p.Dmg)
	}
}

func TestIdentifying(t *testing.T) {
	heal := kindNamed("potion of healing")
	w := roomWith()
	w.Inventory = append(w.Inventory, &Item{ItemKind: heal}, &Item{ItemKind: heal})
	name := w.ItemName(w.Inventory[1])
	if !strings.HasSuffix(name, " potion") || name == "potion of healing" {
		t.Fatalf("an unknown potion should go by its colour, got %q", name)
	}
	w.Player.HP = 1
	w.Use(1)
	if w.Player.HP != w.Player.MaxHP || !slices.Contains(w.Log, "It was a potion of healing.") {
		t.Fatalf("drinking should heal to full (up to 20) and reveal the potion, got HP %d, log %q", w.Player.HP, w.Log)
	}
	if got := w.ItemName(w.Inventory[1]); got != "potion of healing" {
		t.Fatalf("the other potion of its kind should now be known, got %q", got)
	}
}

func TestScrolls(t *testing.T) {
	w := roomWith()
	w.Inventory = append(w.Inventory, &Item{ItemKind: kindNamed("scroll of enchant weapon")})
	w.Use(1)
	if w.Inventory[0].Plus != 1 || w.Player.Dmg != hero.Dmg+4+1 {
		t.Fatalf("enchanting should make the sword +1, got %+v", w.Inventory[0])
	}
}

func TestGold(t *testing.T) {
	w := NewWorld([]string{
		"#####",
		"#@..#",
		"#####",
	})
	w.Floor = []*Item{{ItemKind: gold, Amount: 25, X: 2, Y: 1}}
	before := w.Score()
	w.Step(1, 0)
	if w.Gold != 25 || len(w.Floor) != 0 || w.Score() != before+25 {
		t.Fatalf("walking onto gold should take it and add to the score, got %d gold", w.Gold)
	}
}

func TestStocking(t *testing.T) {
	w := NewGame(3)
	if len(w.Floor) == 0 {
		t.Fatal("a generated floor should have items on it")
	}
	for _, it := range w.Floor {
		if w.Level[it.Y][it.X] == '#' || it.ItemKind != gold && it.Depth > w.Depth {
			t.Fatalf("%s should lie on open floor and suit depth %d", w.ItemName(it), w.Depth)
		}
	}
}

// Every sprite the game names must exist in the embedded assets, or drawing it panics mid-game.
func TestSpritesExist(t *testing.T) {
	var names []string
	for _, k := range kinds {
		folder := "Character"
		if k.Boss {
			folder = "Bosses"
		}
		names = append(names, folder+"/"+k.Name+"_idle_d_1")
		if k.Missile != "" && k.Missile != "arrow" {
			names = append(names, "FX/"+k.Missile)
		}
	}
	w := NewWorld([]string{"@"})
	for _, k := range itemKinds {
		names = append(names, "World/"+w.ItemSprite(&Item{ItemKind: k}))
	}
	names = append(names, "World/"+gold.Sprite, "Character/"+hero.Name+"_idle_d_1", "FX/arrow_x", "FX/arrow_y")
	for _, th := range themes {
		names = append(names, "World/floor_"+th.floor, "World/wall_block_"+th.wall, "World/stair_down_"+th.wall)
	}
	for _, n := range names {
		if _, err := assets.Open("assets/" + n + ".png"); err != nil {
			t.Errorf("missing sprite %s", n)
		}
	}
}
