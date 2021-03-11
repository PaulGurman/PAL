package main

import (
	"fmt"
	"os"
	"palsm/palexer"
	palsm "palsm/palsm_h"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: ./palsm <file.palsm>")
		os.Exit(1)
	}

	data := palsm.ReadFile(os.Args[1])

	lexer := palexer.Lexer{Current_State: palexer.START, Index: 0}
	instructions := lexer.Lex(data)

	if len(instructions) == 0 {
		os.Exit(0)
	}

	palsm.WriteBinaryFile(os.Args[1], instructions)
}
