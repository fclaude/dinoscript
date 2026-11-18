package main

import (
	"log"

	"github.com/fclaude/dinoscript/internal/desktop"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison"
)

func main() {
	closeDisplay, err := prepareDisplay()
	if err != nil {
		log.Fatal(err)
	}
	defer closeDisplay()
	xos.AppName = "Dinoscript"
	xos.AppIdentifier = "com.fclaude.dinoscript"
	unison.SetFocusForReading(true)
	unison.Start(
		unison.QuitAfterLastWindowClosedCallback(func() bool { return true }),
		unison.QuittingCallback(closeDisplay),
		unison.StartupFinishedCallback(func() {
			if err := desktop.Open(); err != nil {
				closeDisplay()
				log.Fatal(err)
			}
		}),
	)
}
