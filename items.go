package main

import (
	"fmt"
	"math/rand/v2"
	"slices"
)

// ItemKind is a type of item. Worn items add their bonuses to the hero; potions and scrolls do something
// when used, and hide what until the hero has used one of the kind.
type ItemKind struct {
	Name   string // its true name
	Sprite string // under World/; potions get theirs from the run's colours
	Slot   string // where it is worn; "" for potions and scrolls, "gold" for gold

	Atk, Def, Dmg, MaxHP int // bonuses while worn

	Class  byte // 'p' potion or 's' scroll: unidentified until used
	Depth  int  // the shallowest floor it appears on
	Weight int  // how common it is among the items that can appear

	use func(w *World) // what a potion or scroll does
}

var gold = &ItemKind{Name: "gold", Sprite: "object_gold", Slot: "gold"}

var itemKinds = []*ItemKind{
	{Name: "dagger", Sprite: "object_dagger", Slot: "weapon", Dmg: 2, Atk: 1, Depth: 1, Weight: 3},
	{Name: "sword", Sprite: "object_sword", Slot: "weapon", Dmg: 4, Depth: 1, Weight: 3},
	{Name: "staff", Sprite: "object_staff", Slot: "weapon", Dmg: 3, Def: 1, Depth: 2, Weight: 1},
	{Name: "spear", Sprite: "object_spear", Slot: "weapon", Dmg: 5, Depth: 3, Weight: 2},
	{Name: "axe", Sprite: "object_axe", Slot: "weapon", Dmg: 6, Atk: -1, Depth: 4, Weight: 2},
	{Name: "hammer", Sprite: "object_hammer", Slot: "weapon", Dmg: 8, Atk: -2, Depth: 6, Weight: 2},

	{Name: "helm", Sprite: "object_helm", Slot: "head", Def: 1, Depth: 1, Weight: 3},
	{Name: "wooden shield", Sprite: "object_shield_wood", Slot: "shield", Def: 1, Depth: 1, Weight: 3},
	{Name: "round shield", Sprite: "object_shield_round", Slot: "shield", Def: 2, Depth: 3, Weight: 2},
	{Name: "metal shield", Sprite: "object_shield_metal", Slot: "shield", Def: 3, Atk: -1, Depth: 6, Weight: 2},
	{Name: "robe", Sprite: "object_robe", Slot: "body", Def: 1, Depth: 1, Weight: 2},
	{Name: "cape", Sprite: "object_cape", Slot: "body", Def: 2, Depth: 3, Weight: 2},
	{Name: "cloak", Sprite: "object_cloak", Slot: "body", Def: 3, Depth: 6, Weight: 2},
	{Name: "boots", Sprite: "object_boots", Slot: "feet", Def: 1, Depth: 2, Weight: 2},
	{Name: "gloves", Sprite: "object_gloves", Slot: "hands", Atk: 1, Depth: 2, Weight: 2},
	{Name: "ring of protection", Sprite: "object_ring", Slot: "ring", Def: 2, Depth: 4, Weight: 1},
	{Name: "ring of accuracy", Sprite: "object_ring", Slot: "ring", Atk: 2, Depth: 4, Weight: 1},
	{Name: "amulet of life", Sprite: "object_amulet", Slot: "amulet", MaxHP: 10, Depth: 6, Weight: 1},

	{Name: "potion of healing", Class: 'p', Depth: 1, Weight: 8, use: func(w *World) {
		w.Player.HP = min(w.Player.MaxHP, w.Player.HP+20)
		w.say("You feel much better.")
	}},
	{Name: "potion of vigour", Class: 'p', Depth: 1, Weight: 2, use: func(w *World) {
		w.base.MaxHP += 5
		w.recalc()
		w.Player.HP += 5
		w.say("You feel more robust.")
	}},
	{Name: "potion of poison", Class: 'p', Depth: 1, Weight: 3, use: func(w *World) {
		w.Player.HP = max(1, w.Player.HP-5-w.Depth/2)
		w.say("You feel sick.")
	}},

	{Name: "scroll of mapping", Sprite: "object_scroll", Class: 's', Depth: 1, Weight: 2, use: func(w *World) {
		for y := range w.Seen {
			for x := range w.Seen[y] {
				w.Seen[y][x] = true
			}
		}
		w.say("A map of the floor forms in your mind.")
	}},
	{Name: "scroll of teleport", Sprite: "object_scroll", Class: 's', Depth: 1, Weight: 2, use: func(w *World) {
		var spots [][2]int
		for y, row := range w.Level {
			for x := range row {
				if row[x] != '#' && row[x] != '>' && w.free(x, y) {
					spots = append(spots, [2]int{x, y})
				}
			}
		}
		s := spots[w.rng.IntN(len(spots))]
		w.Player.X, w.Player.Y = s[0], s[1]
		w.say("You are somewhere else.")
	}},
	{Name: "scroll of enchant weapon", Sprite: "object_scroll", Class: 's', Depth: 1, Weight: 2, use: func(w *World) {
		w.enchant(func(it *Item) bool { return it.Slot == "weapon" }, "weapon")
	}},
	{Name: "scroll of enchant armour", Sprite: "object_scroll", Class: 's', Depth: 1, Weight: 2, use: func(w *World) {
		w.enchant(func(it *Item) bool { return it.Def > 0 && it.Slot != "ring" }, "armour")
	}},
}

// The faces unidentified potions and scrolls wear; each run shuffles which kind gets which.
var (
	potionColours = []string{"red", "blue", "green"}
	scrollLabels  = []string{"ZELGO MER", "FOOBIE BLETCH", "XIXAXA", "VERR YED HORRE", "ELAM EBOW", "DAIYEN FOOELS"}
)

const maxItems = 20 // how many items the hero can carry

type Item struct {
	*ItemKind
	X, Y   int // where it lies, while on the floor
	Plus   int // enchantment: adds to a weapon's damage or a piece of armour's defence
	Amount int // for gold
	Worn   bool
}

func kindNamed(name string) *ItemKind {
	i := slices.IndexFunc(itemKinds, func(k *ItemKind) bool { return k.Name == name })
	return itemKinds[i]
}

// shuffleFaces gives this run's potions their colours and scrolls their labels.
func (w *World) shuffleFaces() {
	w.faces = map[*ItemKind]string{}
	colours, labels := slices.Clone(potionColours), slices.Clone(scrollLabels)
	w.rng.Shuffle(len(colours), func(i, j int) { colours[i], colours[j] = colours[j], colours[i] })
	w.rng.Shuffle(len(labels), func(i, j int) { labels[i], labels[j] = labels[j], labels[i] })
	for _, k := range itemKinds {
		switch k.Class {
		case 'p':
			w.faces[k], colours = colours[0], colours[1:]
		case 's':
			w.faces[k], labels = labels[0], labels[1:]
		}
	}
}

// ItemName is what the hero calls an item: its true name once its kind is known.
func (w *World) ItemName(it *Item) string {
	switch {
	case it.ItemKind == gold:
		return fmt.Sprintf("%d gold", it.Amount)
	case it.Class == 'p' && !w.known[it.ItemKind]:
		return w.faces[it.ItemKind] + " potion"
	case it.Class == 's' && !w.known[it.ItemKind]:
		return "scroll labelled " + w.faces[it.ItemKind]
	case it.Plus > 0:
		return fmt.Sprintf("%s +%d", it.Name, it.Plus)
	}
	return it.Name
}

// ItemSprite is the sprite an item is drawn with, under World/.
func (w *World) ItemSprite(it *Item) string {
	if it.Class == 'p' {
		return "object_potion_" + w.faces[it.ItemKind]
	}
	return it.Sprite
}

// rollItem makes a random item fit for a floor: deeper floors allow better kinds and enchantments.
func rollItem(rng *rand.Rand, depth int) *Item {
	var pool []*ItemKind
	total := 0
	for _, k := range itemKinds {
		if k.Depth <= depth {
			pool = append(pool, k)
			total += k.Weight
		}
	}
	n := rng.IntN(total)
	for _, k := range pool {
		if n -= k.Weight; n < 0 {
			it := &Item{ItemKind: k}
			if k.Dmg > 0 || k.Def > 0 && k.Slot != "ring" {
				it.Plus = rng.IntN(depth/4 + 1)
			}
			return it
		}
	}
	panic("unreachable")
}

// stockItems rolls an item, or gold, for each spot the generator marked on this floor.
func (w *World) stockItems(spots [][2]int) {
	for _, s := range spots {
		it := &Item{ItemKind: gold, Amount: (5 + w.rng.IntN(11)) * w.Depth}
		if w.rng.IntN(3) > 0 {
			it = rollItem(w.rng, w.Depth)
		}
		it.X, it.Y = s[0], s[1]
		w.Floor = append(w.Floor, it)
	}
}

// ItemsAt lists the items lying on a tile, the top one last.
func (w *World) ItemsAt(x, y int) []*Item {
	var its []*Item
	for _, it := range w.Floor {
		if it.X == x && it.Y == y {
			its = append(its, it)
		}
	}
	return its
}

// pickUpGold collects any gold on the hero's tile; walking over it is enough.
func (w *World) pickUpGold() {
	p := w.Player
	w.Floor = slices.DeleteFunc(w.Floor, func(it *Item) bool {
		if it.ItemKind == gold && it.X == p.X && it.Y == p.Y {
			w.Gold += it.Amount
			w.say("You pick up %d gold.", it.Amount)
			return true
		}
		return false
	})
}

// PickUp takes the top item from the hero's tile. It takes a turn if there was anything to take.
func (w *World) PickUp() {
	its := w.ItemsAt(w.Player.X, w.Player.Y)
	switch {
	case len(its) == 0:
		w.Log = []string{"There is nothing here."}
	case len(w.Inventory) >= maxItems:
		w.Log = []string{"You can't carry any more."}
	default:
		w.turn(func() {
			it := its[len(its)-1]
			w.Floor = slices.DeleteFunc(w.Floor, func(f *Item) bool { return f == it })
			w.Inventory = append(w.Inventory, it)
			w.say("You pick up the %s.", w.ItemName(it))
		})
	}
}

// Drop puts an inventory item on the hero's tile, taking it off first if worn. It takes a turn.
func (w *World) Drop(i int) {
	if i < 0 || i >= len(w.Inventory) {
		return
	}
	w.turn(func() {
		it := w.Inventory[i]
		w.Inventory = slices.Delete(w.Inventory, i, i+1)
		it.Worn = false
		w.recalc()
		it.X, it.Y = w.Player.X, w.Player.Y
		w.Floor = append(w.Floor, it)
		w.say("You drop the %s.", w.ItemName(it))
	})
}

// Use applies an inventory item: wears or takes off gear, drinks a potion, reads a scroll. It takes a turn.
func (w *World) Use(i int) {
	if i < 0 || i >= len(w.Inventory) {
		return
	}
	w.turn(func() {
		it := w.Inventory[i]
		if it.Slot != "" {
			w.wear(it)
			return
		}
		w.Inventory = slices.Delete(w.Inventory, i, i+1)
		was := w.ItemName(it)
		w.known[it.ItemKind] = true
		if it.Class == 'p' {
			w.say("You drink the %s.", was)
		} else {
			w.say("You read the %s.", was)
		}
		it.use(w)
		if was != it.Name {
			w.say("It was a %s.", it.Name)
		}
	})
}

// wear puts an item on, taking off whatever else was in its slot, or takes it off if already worn.
func (w *World) wear(it *Item) {
	if it.Worn {
		it.Worn = false
		w.say("You take off the %s.", w.ItemName(it))
	} else {
		for _, o := range w.Inventory {
			if o.Worn && o.Slot == it.Slot {
				o.Worn = false
			}
		}
		it.Worn = true
		w.say("You are now using the %s.", w.ItemName(it))
	}
	w.recalc()
}

// recalc sets the hero's stats: its base, which levels raise, plus everything it wears.
func (w *World) recalc() {
	k := w.base
	for _, it := range w.Inventory {
		if !it.Worn {
			continue
		}
		k.Atk += it.Atk
		k.Def += it.Def
		k.Dmg += it.Dmg
		k.MaxHP += it.MaxHP
		if it.Slot == "weapon" {
			k.Dmg += it.Plus
		} else {
			k.Def += it.Plus
		}
	}
	p := w.Player
	p.Kind = k
	p.HP = min(p.HP, p.MaxHP)
}

// enchant adds 1 to a random worn item that matches.
func (w *World) enchant(match func(*Item) bool, what string) {
	var worn []*Item
	for _, it := range w.Inventory {
		if it.Worn && match(it) {
			worn = append(worn, it)
		}
	}
	if len(worn) == 0 {
		w.say("You feel a faint tingle, and nothing else.")
		return
	}
	it := worn[w.rng.IntN(len(worn))]
	it.Plus++
	w.recalc()
	w.say("Your %s glows blue.", it.Name)
}
