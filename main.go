// Command febuildergba-mygo starts the native Fire Emblem ROM editor.
package main

import (
	"log"
	"os"

	"github.com/woxQAQ/febuildergba-mygo/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
