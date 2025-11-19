package banner

import (
	"fmt"
)

// prints the version message
const version = "v0.0.6"

func PrintVersion() {
	fmt.Printf("Current vulntechfinder version %s\n", version)
}

// Prints the Colorful banner
func PrintBanner() {
	banner := `
                __        __               __     ____ _             __           
 _   __ __  __ / /____   / /_ ___   _____ / /_   / __/(_)____   ____/ /___   _____
| | / // / / // // __ \ / __// _ \ / ___// __ \ / /_ / // __ \ / __  // _ \ / ___/
| |/ // /_/ // // / / // /_ /  __// /__ / / / // __// // / / // /_/ //  __// /    
|___/ \__,_//_//_/ /_/ \__/ \___/ \___//_/ /_//_/  /_//_/ /_/ \__,_/ \___//_/
`
	fmt.Printf("%s\n%70s\n\n", banner, "Current vulntechfinder version "+version)
}
