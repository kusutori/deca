package main

import (
	"os"

	"github.com/kusutori/deca/cmd"
)

var exit = os.Exit

func main() {
	exit(cmd.Execute())
}
