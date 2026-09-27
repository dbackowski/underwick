# Ninedeep

Turn-based roguelike built with Go and [Ebitengine](https://ebitengine.org).

## Setup

The sprites are from Oryx Design Lab's [8-Bit Remaster](https://www.oryxdesignlab.com/products/p/lofi-fantasy-remaster) pack and are not in this repo (license). Copy them in before building:

    cp -R path/to/oryx_8-bit_remaster/Sliced assets

## Run

    go run .

Arrow keys or WASD move and attack, space waits a turn, R restarts after death.

You are a spark in a borrowed body (outlined in blue). The body rots 1 HP every few turns.
Beat a monster down to a third of its HP (it blinks) and walk into it to take it over.
If your body dies the spark tears free, breaking every monster next to it, and moves twice per turn.
You have 3 turns to take a new body. A broken monster that nobody takes falls apart after 6 turns.

Monsters within 6 tiles notice you only if they can see you, then hunt you down to where they last saw you.

Each floor is generated. Take the stairs down to reach the next one; the ninth is the bottom.

Floors 3, 6 and 9 each hold a boss, three of six per run: dragon, beholder, lord, cyclops, demon or reaper.
Its stairs stay sealed until it dies. A boss can't be broken or possessed, but every body that dies next
to it scorches it for a quarter of its HP, and it keeps three minions close to take over mid-fight.

Bodies: the goblin archer shoots anything in a straight line up to 5 tiles away, the rat moves twice per turn, and the orc hits hard but rots twice as fast.

`go run . -shot frame.png` saves one rendered frame and exits, for checking rendering.

`NINEDEEP_SIM=1 go test -run Balance -v` plays 300 seeded runs with three bots and prints win rates, for tuning.

## Credits

Art by [Oryx Design Lab](https://www.oryxdesignlab.com).
