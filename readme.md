 # gobandcamp

 Barebones terminal player for bandcamp, uses [Beep](https://github.com/faiface/beep/) package to play actual sound, [tcell](https://github.com/gdamore/tcell) to display metadata and handle controls, and [image2ascii](https://github.com/qeesung/image2ascii) to convert album cover to colored ASCII-art.
 
 WIP, will likely crash.
 Only album pages (e.g. artistname.bandcamp.com) are supported.

 Controls:

	Space - play/pause
	  M	  - mute
	  S	  - lower volume
      W	  - raise volume
	  A	  - go back 2 seconds
	  D   - go forward 2 seconds
	  F	  - next track
      B	  - previous track
      R	  - change playback mode
	  O	  - goes back to terminal, entering new valid link will play new album
     Esc  - quit
