 # gobandcamp

 Barebones terminal player for bandcamp, uses [Beep](https://github.com/faiface/beep/) package to play actual sound, [tcell](https://github.com/gdamore/tcell) to display metadata and handle controls, and [image2ascii](https://github.com/qeesung/image2ascii) to convert album cover to colored ASCII-art.
 
 WIP

 Only album pages (e.g. artistname.bandcamp.com/album/albumname) are supported.

## Controls

| Shortcut                | Description                                                        |
|-------------------------|--------------------------------------------------------------------|
|     <kbd>Space</kbd>    | play/pause                                                         |
|       <kbd>M</kbd>      | mute                                                               |
|       <kbd>S</kbd>      | lower volume                                                       |
|       <kbd>W</kbd>      | raise volume                                                       |
|       <kbd>A</kbd>      | go back 2 seconds                                                  |
|       <kbd>D</kbd>      | go forward 2 seconds                                               |
|       <kbd>F</kbd>      | next track                                                         |
|       <kbd>B</kbd>      | previous track                                                     |
|       <kbd>R</kbd>      | change playback mode                                               |
|       <kbd>O</kbd>      | goes back to terminal, entering new valid link will play new album |
|      <kbd>Esc</kbd>     | quit                                                               |
