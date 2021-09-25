 # gobandcamp

 Barebones terminal player for bandcamp, uses [Beep](https://github.com/faiface/beep/) package to play actual sound.
 Very WIP, will likely crash. Clears terminal window to display metadata

 At the start waits for user to input valid bandcamp album link, parses info from page, plays full album.
 After playback starts, displays parsed metadata and album cover as ASCII art, then waits for user to input one of the following comands: 

	M - mute
	S - lower volume
    W - raise volume
	A - go back 2 seconds
	D - go forward 2 seconds
	P - play/pause
	F - next track
    B - previous track
    R - change playback mode (not implemented)
    Q - quit
    Any other string will just update info on screen
