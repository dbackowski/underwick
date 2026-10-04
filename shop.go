package main

import "slices"

const (
	shopEvery = 3 // a merchant keeps shop every 3rd floor from the 2nd, though not beside a boss
	wares     = 5 // what a merchant has for sale

	// Buy takes these for the merchant's services, in place of a ware's index.
	buyHeal     = -1
	buyIdentify = -2
)

func shopFloor(depth int, boss byte) bool { return boss == 0 && depth%shopEvery == 2 }

// stockWares fills the merchant's shop, with items as good as a vault's chest holds.
func (w *World) stockWares() {
	for range wares {
		w.Wares = append(w.Wares, rollItem(w.rng, w.Depth+2))
	}
}

// Price is what the merchant asks for an item: more for deeper kinds and enchantments, and tomes most.
// ponytail: a flat formula; tune it once played runs show how much gold the hero has at each shop
func (it *Item) Price() int {
	p := 10 * (it.Depth + 1 + 2*it.Plus)
	if it.Class == 't' {
		p += 50
	}
	return p
}

// HealPrice is what the merchant asks to heal the hero fully and cure its poison: 2 gold for each HP.
func (w *World) HealPrice() int { return 2 * (w.Player.MaxHP - w.Player.HP) }

// IdentifyPrice is what the merchant asks to name every potion and scroll the hero carries: 25 gold for
// each kind it doesn't know yet.
func (w *World) IdentifyPrice() int { return 25 * len(w.unknownCarried()) }

func (w *World) unknownCarried() []*ItemKind {
	var ks []*ItemKind
	for _, it := range w.Inventory {
		if (it.Class == 'p' || it.Class == 's') && !w.known[it.ItemKind] && !slices.Contains(ks, it.ItemKind) {
			ks = append(ks, it.ItemKind)
		}
	}
	return ks
}

// nextToShop reports whether the hero stands beside the merchant.
func (w *World) nextToShop() bool {
	p := w.Player
	return slices.ContainsFunc(dirs, func(d [2]int) bool { return w.Level[p.Y+d[1]][p.X+d[0]] == '$' })
}

// Buy pays the merchant beside the hero for a ware, by its index, or for buyHeal or buyIdentify. It takes
// no time: the monsters wait while the hero shops.
func (w *World) Buy(i int) {
	w.record(Action{Do: 'b', I: i})
	var price int
	switch {
	case !w.nextToShop():
		return
	case i == buyHeal:
		if price = w.HealPrice(); price == 0 && w.Player.Poison == 0 {
			w.Log = []string{"You need no healing."}
			return
		}
	case i == buyIdentify:
		if price = w.IdentifyPrice(); price == 0 {
			w.Log = []string{"You know everything you carry."}
			return
		}
	case i >= 0 && i < len(w.Wares):
		if price = w.Wares[i].Price(); len(w.Inventory) >= maxItems {
			w.Log = []string{"You can't carry any more."}
			return
		}
	default:
		return
	}
	if w.Gold < price {
		w.Log = []string{"You can't afford that."}
		return
	}
	w.Gold -= price
	w.Log = nil
	switch i {
	case buyHeal:
		w.Player.HP, w.Player.Poison = w.Player.MaxHP, 0
		w.say("The merchant patches you up.")
	case buyIdentify:
		for _, k := range w.unknownCarried() {
			w.known[k] = true
		}
		w.say("The merchant tells you what you carry.")
	default:
		it := w.Wares[i]
		w.Wares = slices.Delete(w.Wares, i, i+1)
		w.Inventory = append(w.Inventory, it)
		w.say("You buy the %s for %d gold.", w.ItemName(it), price)
	}
}
