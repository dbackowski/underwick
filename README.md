# Underwick

A classic turn-based roguelike built with Go and [Ebitengine](https://ebitengine.org). Go down into the
dungeon below Underwick as far as you can. Death is permanent, and the dungeon has no bottom.

## Setup

The sprites are from Oryx Design Lab's [8-Bit Remaster](https://www.oryxdesignlab.com/products/p/lofi-fantasy-remaster) pack and are not in this repo (license). Copy them in before building:

    cp -R path/to/oryx_8-bit_remaster/Sliced assets

## Run

    go run .

Arrow keys or WASD move and attack, space waits a turn, R starts a new run after death.

Walk into a monster to attack it. A hit lands 70% of the time, plus 5% for each point of your accuracy over
its armour, and deals 1 up to your weapon's damage. You regain 1 HP every 8 turns.

Monsters within 6 tiles notice you only if they can see you, then hunt you down to where they last saw you.
Goblin archers shoot along straight lines; rats move twice per turn.

Each floor is generated, 40x30 tiles, and you only see what you can see: explored parts stay on the map,
dimmed. Take the stairs down to reach the next floor. Every 5th floor a boss guards the stairs, which open
when it dies: dragon, beholder, lord, cyclops, demon or reaper, stronger each time round.

Your score is 100 per floor reached plus 10 per kill.

## Development

`go run . -shot frame.png` saves one rendered frame and exits, for checking rendering.

`UNDERWICK_SIM=1 go test -run Balance -v` plays 300 seeded runs with two bots and prints how deep they got,
for tuning. The bots see the whole floor.

## Credits

Art by [Oryx Design Lab](https://www.oryxdesignlab.com).
