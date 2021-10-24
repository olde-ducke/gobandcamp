# gobandcamp

![screenshot](/assets/screenshot.png)

Terminal player for bandcamp, uses [Beep](https://github.com/faiface/beep/) package to play actual sound, [tcell](https://github.com/gdamore/tcell) to display metadata and handle controls, and [image2ascii](https://github.com/qeesung/image2ascii) to convert album cover to colored ASCII-art. WIP
 
Placeholder image source: https://github.com/egonelbre/gophers

Features:
- Playback of media from band/album/track pages (by pasting link into input field)
- Tag search (WIP, for now only plays one random item from first results page)

Tag search:

    -t sometag anothertag third-tag -s random -f cd

or

    --tag sometag --sort date --tag another three tags --format vinyl

Sorting methods (optional):

    ""           - popular
    "random"     - random
    "date"       - sort by date
    "highlights" - search in highlights tab of first tag/genre

Formats (optional):

    ""           - any
    "cassette"
    "cd"
    "vinyl"


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
|       <kbd>T</kbd>      | switch theme                                                       |
|       <kbd>I</kbd>      | switch art drawing method                                          |
|      <kbd>Tab</kbd>     | enable input                                                       |
|      <kbd>Esc</kbd>     | quit                                                               |
