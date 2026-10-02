# Underwick

A classic turn-based roguelike built with Go and [Ebitengine](https://ebitengine.org). Go down into the
dungeon below Underwick as far as you can. Death is permanent, and the dungeon has no bottom.

## Setup

The sprites and font are from Oryx Design Lab's [8-Bit Remaster](https://www.oryxdesignlab.com/products/p/lofi-fantasy-remaster) pack and are not in this repo (license). Copy them in before building:

    cp -R path/to/oryx_8-bit_remaster/Sliced assets
    cp path/to/oryx_8-bit_remaster/oryx-simplex.ttf assets/

## Run

    go run .

Arrow keys or WASD move and attack, space waits a turn, R starts a new run after death.
G picks up what you stand on, I opens your pack (a letter uses, wears or takes off an item), X drops
an item, and Escape closes the pack.

Walk into a monster to attack it. A hit lands 70% of the time, plus 5% for each point of your accuracy over
its armour, and deals 1 up to your weapon's damage. You regain 1 HP every 8 turns.

Monsters within 6 tiles notice you only if they can see you, then hunt you down to where they last saw you.
Twenty kinds live at different depths, from rats and bats on floor 1 to flayers and demons below floor 12;
each floor holds those that first appear there or up to 6 floors above. Some are fast, some shoot along
straight lines, and every 4 floors below their first appearance they grow a little stronger.

Kills give experience. Going from level l to the next takes 5 x l x l, and each level gives 5 max HP and
1 accuracy, plus 1 damage every 2nd level and 1 armour every 3rd.

Items lie about the floors, better ones deeper: weapons (dagger, sword, staff, spear, axe, hammer), armour
for the head, body, hands and feet, shields, rings, an amulet, potions and scrolls. You start with a
sword. Weapons and armour can be enchanted (sword +2). Potions and scrolls hide what they are: each run
gives the potions their colours and the scrolls their labels anew, and you learn a kind by using one.
You can carry 20 items. Gold is picked up as you walk over it.

Each floor is generated, 40x30 tiles, and you only see what you can see: explored parts stay on the map,
dimmed. Take the stairs down to reach the next floor. Every 5th floor a boss guards the stairs, which open
when it dies: dragon, beholder, lord, cyclops, demon or reaper, stronger each time round.

Your score is 100 per floor reached, 10 per kill, 50 per level gained, and your gold.

## Development

`go run . -shot frame.png` saves one rendered frame and exits, for checking rendering.

`UNDERWICK_SIM=1 go test -run Balance -v` plays 300 seeded runs with two bots and prints how deep they got,
for tuning. The bots see the whole floor.

## Credits

Art by [Oryx Design Lab](https://www.oryxdesignlab.com).
