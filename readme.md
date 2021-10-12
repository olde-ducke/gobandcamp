 # gobandcamp

 Barebones terminal player for bandcamp, uses [Beep](https://github.com/faiface/beep/) package to play actual sound, [tcell](https://github.com/gdamore/tcell) to display metadata and handle controls, and [image2ascii](https://github.com/qeesung/image2ascii) to convert album cover to colored ASCII-art.
 
 WIP

 Only album pages (e.g. artistname.bandcamp.com/album/albumname) are supported.

## Controls

| Shortcut                | Description                                                        |
|-------------------------|--------------------------------------------------------------------|
|     <kbd>Space</kbd>    | play/pause                                                         |
|       <kbd>P</kbd>      | stop                                                               |
|       <kbd>M</kbd>      | mute                                                               |
|       <kbd>S</kbd>      | lower volume                                                       |
|       <kbd>W</kbd>      | raise volume                                                       |
|       <kbd>A</kbd>      | rewind                                                             |
|       <kbd>D</kbd>      | fast forward                                                       |
|       <kbd>F</kbd>      | next track                                                         |
|       <kbd>B</kbd>      | previous track                                                     |
|       <kbd>R</kbd>      | change playback mode                                               |
|       <kbd>O</kbd>      | goes back to terminal, after entering new link new album will play |
|       <kbd>T</kbd>      | light/dark mode                                                    |
|       <kbd>I</kbd>      | switch art drawing method                                          |
|      <kbd>Esc</kbd>     | quit                                                               |
