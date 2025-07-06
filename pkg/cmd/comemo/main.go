package main

import (
	"context"
	"log"
	"os"

	"comemo/pkg/cli"
)

func main() {
	app := cli.CreateApp()
	
	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}