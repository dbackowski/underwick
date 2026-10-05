# Underwick

A classic turn-based roguelike built with Go and [Ebitengine](https://ebitengine.org). Go down into the
dungeon below Underwick as far as you can. Death is permanent, and the dungeon has no bottom.

**Play it in your browser: https://dbackowski.github.io/underwick/** (keyboard needed), or build it to run on your
desktop as below.

## Setup

The sprites and font are from Oryx Design Lab's [8-Bit Remaster](https://www.oryxdesignlab.com/products/p/lofi-fantasy-remaster) pack and are not in this repo (license). Copy them in before building:

    cp -R path/to/oryx_8-bit_remaster/Sliced assets
    cp path/to/oryx_8-bit_remaster/oryx-simplex.ttf assets/

## Run

    go run .

The title screen starts a new game, continues a saved one, or shows the high scores and the keys. Pick a
hero to start: the warrior (tough, with a sword), the archer (shoots with a bow), the mage (frail,
casts Fire Bolt), the thief (quiet, so sleeping monsters wake half as often) or the cleric (armoured,
casts Heal).

Arrow keys or WASD move and attack (hold one to keep walking, up to a wall or a warning), space waits a turn, R chooses a new hero after death. G picks up
what you stand on, I opens your pack (a letter uses, wears or takes off an item), X drops an item, C
casts a spell (a letter picks it, then a direction aims it if it needs aiming), L looks around (move
the cursor to see what is on a tile, and how a monster's fight with you would go), M shows the last 100 messages, V turns the music off, then all sound, then both back on, and Escape closes a
panel or opens the menu, where you can see the keys and high scores or save and quit.

Quitting, or closing the window, saves the run; continuing it deletes the save, and dying ends it for good.
The top 10 runs are kept with their class, depth and cause of death. Both live in your user config folder
(`~/Library/Application Support/underwick` on macOS); in the browser, in its local storage, with the run saved
after every turn. A save is replayed from your moves, so one made before an update that changes the rules is
refused.

Walk into a monster to attack it. A hit lands 70% of the time, plus 5% for each point of your accuracy over
its armour, and deals 1 up to your weapon's damage. HP never comes back on its own: only
potions, the Heal spell and the merchant restore it, and gaining a level restores all of it.

Monsters within 6 tiles notice you only if they can see you, then hunt you down to where they last saw you.
Every 150 turns or so another monster wanders onto the floor out of sight and comes looking for you, so
waiting has a price. A third of the floor's own monsters start asleep, waking only by chance once they could see you, or when hit; a sleeper is
always hit. Snakes and spiders can poison you (1 HP a turn), and imps and ghosts can confuse you (half
your moves go astray).
Twenty kinds live at different depths, from rats and bats on floor 1 to flayers and demons below floor 12;
each floor holds those that first appear there or up to 6 floors above. Some are fast, some shoot along
straight lines, and every 4 floors below their first appearance they grow a little stronger.

Kills give experience. Going from level l to the next takes 5 x l x l, and each level gives 5 max HP and
1 accuracy, plus 1 damage every 2nd level and 1 armour every 3rd.

With a bow in hand, moving toward a monster in a straight line within 5 tiles shoots it. Spells cost mana,
which comes back 1 every 5 turns and grows with levels: Fire Bolt hits the first monster in a line, Heal
restores HP and cures poison, and Sleep puts the monsters you can see to sleep. Tomes teach spells, and
give a hero without magic some mana.

Items lie about the floors, better ones deeper: weapons (dagger, sword, staff, spear, axe, hammer, bow), armour
for the head, body, hands and feet, shields, rings, an amulet, potions, scrolls and tomes. Each hero starts
with its own gear in use. The pack (I) shows what each piece adds (dmg, arm for armour, acc for accuracy, hp)
and your totals; armour makes monsters miss more, 5% a point. Weapons and armour can be enchanted (sword +2). Potions and scrolls hide what they are: each run
gives the potions their colours and the scrolls their labels anew, and you learn a kind by using one.
You can carry 20 items. Gold is picked up as you walk over it.

Each floor is generated, 40x30 tiles, and you only see what you can see: explored parts stay on the map,
dimmed. Walk into a door to open it; closed doors block sight, and monsters open them too. A dead-end
room may be a vault behind an iron door (it needs the gold key) or a magic door (the blue key), with the
key lying elsewhere on the floor and a chest of better loot inside.

Some rooms hold pools that monsters never enter: deep water slows you, giving monsters an extra turn;
acid burns each turn you stand in it; lava burns badly, so you are warned and must move again to step
in; and a pit drops you to the next floor, with a fall. Take the stairs down to reach the next floor. Every 5th floor a boss guards the stairs, which open
when it dies: dragon, beholder, lord, cyclops, demon or reaper, stronger each time round. Its hoard, two
fine items and gold, spills around the stairs.

Each floor plays one of five tracks in turn, with its own track while a boss lives and another once you die.

On the floor before each boss's (4, 9, 14 and so on) a merchant, the dwarf, keeps shop in one of the rooms.
Walk into the merchant to buy one of five wares, as good as a vault's, or pay to be healed (2 gold an HP, poison
cured too) or to have your potions and scrolls named (25 gold a kind). Shopping takes no time.

Your score is 100 per floor reached, 10 per kill, 50 per level gained, and all the gold you found, spent or not.

## Browser

The browser version is on GitHub Pages: https://dbackowski.github.io/underwick/

To build it and run it locally:

    GOOS=js GOARCH=wasm go build -o web/underwick.wasm .
    cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/
    python3 -m http.server -d web

then open http://localhost:8000.

## Development

`go run . -shot frame.png` saves one rendered frame and exits, for checking rendering.

`./web/deploy.sh` builds the browser version and publishes it to the `gh-pages` branch, which GitHub Pages
serves. It runs locally because the build needs the sprites, which never go to GitHub.

`UNDERWICK_SIM=1 go test -run Balance -v` plays 300 seeded runs with bots, a stairs-rushing warrior and a
careful player as every class, and prints how deep they got, for tuning. The bots see the whole floor.

## Credits

Art by [Oryx Design Lab](https://www.oryxdesignlab.com). Sound effects and music by Juhani Junkala, released
under CC0 and kept in `audio/`: [The Essential Retro Video Game Sound Effects Collection](https://opengameart.org/content/512-sound-effects-8-bit-style),
[Chiptune Adventures](https://opengameart.org/content/4-chiptunes-adventure) and the
[Retro Game Music Pack](https://opengameart.org/content/5-chiptunes-action).
