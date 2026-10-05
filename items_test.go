package main

import (
	"image"
	"image/png"
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
	w := NewGame(3, classes[0])
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
	names = append(names, "World/"+gold.Sprite, "World/"+goldKey.Sprite, "World/"+blueKey.Sprite,
		"Character/"+hero.Name+"_idle_d_1", "FX/arrow_x", "FX/arrow_y", "Character/dwarf_idle_d_1", "Character/dwarf_idle_d_2")
	for _, c := range "#.>+/1425~=%^&0" {
		names = append(names, "World/"+tileSprite(byte(c), themes[0], 1))
	}
	for _, th := range themes {
		names = append(names, "World/floor_"+th.floor, "World/wall_block_"+th.wall, "World/stair_down_"+th.wall)
	}
	for _, n := range names {
		if _, err := assets.Open("assets/" + n + ".png"); err != nil {
			t.Errorf("missing sprite %s", n)
		}
	}
}

func TestShop(t *testing.T) {
	w := NewWorld([]string{
		"######",
		"#@$..#",
		"######",
	})
	if len(w.Wares) != wares {
		t.Fatalf("a merchant should have %d wares, got %d", wares, len(w.Wares))
	}
	w.Buy(0)
	if len(w.Wares) != wares || len(w.Inventory) != len(classes[0].Start) {
		t.Fatal("buying without the gold should get nothing")
	}

	w.Gold, w.Found = 1000, 1000
	it, price := w.Wares[0], w.Wares[0].Price()
	w.Buy(0)
	if w.Gold != 1000-price || w.Inventory[len(w.Inventory)-1] != it || len(w.Wares) != wares-1 {
		t.Fatalf("buying should take %d gold and hand over the ware, have %d gold", price, w.Gold)
	}
	if w.Actions[len(w.Actions)-1] != (Action{Do: 'b', I: 0}) {
		t.Fatal("buying should be recorded for the save")
	}
	if w.Score() != 100+w.Found {
		t.Fatal("gold spent should still count for the score")
	}

	w.Gold, w.Player.HP, w.Player.Poison = 1000, 1, 3
	w.Buy(buyHeal)
	if w.Player.HP != w.Player.MaxHP || w.Player.Poison != 0 || w.Gold != 1000-2*(w.Player.MaxHP-1) {
		t.Fatal("the merchant should heal and cure for 2 gold an HP")
	}
	heal := kindNamed("potion of healing")
	w.Inventory = append(w.Inventory, &Item{ItemKind: heal}, &Item{ItemKind: heal})
	gold := w.Gold
	w.Buy(buyIdentify)
	if !w.known[heal] || w.Gold != gold-25 {
		t.Fatal("the merchant should name an unknown kind for 25 gold")
	}

	w.Player.X = 4
	gold = w.Gold
	w.Buy(0)
	if w.Gold != gold {
		t.Fatal("buying away from the merchant should do nothing")
	}
}

// TestWalkFrames checks walkFrames against the art: each hero's walking frames for a direction should look
// most like its idle frame facing that way.
func TestWalkFrames(t *testing.T) {
	load := func(name string) image.Image {
		f, err := assets.Open("assets/Character/" + name + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		img, err := png.Decode(f)
		if err != nil {
			t.Fatal(err)
		}
		return img
	}
	differ := func(a, b image.Image) int {
		n := 0
		for y := range tile {
			for x := range tile {
				if a.At(x, y) != b.At(x, y) {
					n++
				}
			}
		}
		return n
	}
	for _, c := range classes {
		for dir, frames := range walkFrames {
			for _, f := range frames {
				walk := load(c.Name + "_walk_" + f)
				closest := slices.MinFunc([]string{"d", "r", "l", "u"}, func(a, b string) int {
					return differ(walk, load(c.Name+"_idle_"+a+"_1")) - differ(walk, load(c.Name+"_idle_"+b+"_1"))
				})
				if closest != dir {
					t.Errorf("%s walking %s shows walk_%s, which faces %s", c.Name, dir, f, closest)
				}
			}
		}
	}
}

func TestBonuses(t *testing.T) {
	for _, c := range []struct {
		it   *Item
		want string
	}{
		{&Item{ItemKind: kindNamed("sword"), Plus: 2}, "+6 dmg"},
		{&Item{ItemKind: kindNamed("metal shield"), Plus: 1}, "+4 arm -1 acc"},
		{&Item{ItemKind: kindNamed("amulet of life")}, "+10 hp"},
		{&Item{ItemKind: kindNamed("potion of healing")}, ""},
	} {
		if got := c.it.Bonuses(); got != c.want {
			t.Errorf("%s +%d: got %q, want %q", c.it.Name, c.it.Plus, got, c.want)
		}
	}
}
