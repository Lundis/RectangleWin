# RectangleWin

A minimalistic Windows rewrite of macOS
[Rectangle.app](https://rectangleapp.com)/[Spectacle.app](https://www.spectacleapp.com/).
([Why?](#why))

A hotkey-oriented window snapping and resizing tool for Windows.

This animation illustrates how RectangleWin helps me move windows to edges
and corners (and cycle through half, one-thirds or two thirds width or height)
only using hotkeys:

![RectangleWin demo](./assets/RectangleWin-demo.gif)

## Install

1. Go to [Releases](https://github.com/ahmetb/RectangleWin/releases) and
   download the suitable binary for your architecture (typically x64).

2. Launch the `.exe` file. Now the program icon should be visible on system
   tray!

3. Click on the icon and mark as "Run on startup" to make sure you don't have
   to run it every time you reboot your PC.

## Keyboard Bindings

- <kbd>Win</kbd> + <kbd>&larr;</kbd> / <kbd>&rarr;</kbd>: snap to the left/right half of
  the current monitor. Pressing again only moves to another monitor if one sits
  directly beside it (a monitor stacked above/below is never chosen).
- <kbd>Win</kbd> + <kbd>&uarr;</kbd> / <kbd>&darr;</kbd>: move the window to the monitor
  above/below, keeping its relative position, size and maximized state.
- <kbd>Win</kbd> + <kbd>Enter</kbd>: toggle maximized.
- <kbd>Win</kbd> + <kbd>Alt</kbd> + <kbd>C</kbd>: center window.
- <kbd>Win</kbd> + <kbd>Alt</kbd> + <kbd>A</kbd>: toggle always on top.

These replace the built-in Windows Snap shortcuts while RectangleWin is running
(they are captured with a low-level keyboard hook, since Windows does not let
other programs register Win+Arrow as hotkeys).

## Why?

It seems that no window snapping utility for Windows is capable of letting
user snap windows to edges or corners in {half, two-thirds, one-third} sizes
using configurable **shortcut keys**, and center windows in a screen like
Rectangle.app does, so I wrote this small utility for myself.

I've tried the native Windows shortcuts and PowerToys FancyZones and they
are not supporting corners, alternating between half and one/two thirds, and
are not offering enough hotkey support.

## Roadmap

- Configurable shortcuts: I don't need these and it will likely require a pop-up
  UI, so I will probably not get to this.

## Development (Install from source)

With Go 1.17+ installed, clone this repository and run:

```sh
go generate
GOOS=windows go build -ldflags -H=windowsgui .
```

The `RectangleWin.exe` will be available in the same directory.

## License

This project is distributed as-is under the Apache 2.0 license.
See [LICENSE](./LICENSE).

If you see bugs, please open issues. I can't promise any fixes.
