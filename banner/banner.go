package banner

import (
	"fmt"
)

// prints the version message
const version = "v0.0.6"

func PrintVersion() {
	fmt.Printf("Current vulntechx version %s\n", version)
}

// Prints the Colorful banner
func PrintBanner() {
	banner := `
                __        __               __         
 _   __ __  __ / /____   / /_ ___   _____ / /_   _  __
| | / // / / // // __ \ / __// _ \ / ___// __ \ | |/_/
| |/ // /_/ // // / / // /_ /  __// /__ / / / /_>  <  
|___/ \__,_//_//_/ /_/ \__/ \___/ \___//_/ /_//_/|_|  
`
	fmt.Printf("%s\n%60s\n\n", banner, "Current vulntechx version "+version)
}
