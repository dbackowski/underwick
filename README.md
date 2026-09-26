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
If your body dies you have 3 turns as a bare spark to find a new one.

Bodies: the goblin archer shoots anything in a straight line up to 5 tiles away, the rat moves twice per turn, and the orc hits hard but rots twice as fast.

`go run . -shot frame.png` saves one rendered frame and exits, for checking rendering.

## Credits

Art by [Oryx Design Lab](https://www.oryxdesignlab.com).
