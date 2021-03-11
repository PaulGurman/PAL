package palexer

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
)

type State int

const (
	START     State = 0
	END       State = 1
	COMMENT   State = 2
	DUMP      State = 3
	GROUP     State = 4
	BUILDCOMM State = 5
)

type Lexer struct {
	Current_State       State
	BeginningChar       rune
	BuiltString         string
	Index               int
	Parameters          []uint32
	ParametersIndex     int
	CurrentInstruction  uint32
	LexemesIndex        int
	NumParams           int8
	Line                int
	LabelToIndex        map[string]int   // Map of labels to the index they appear in the data
	LabelToInstructions map[string][]int // Map of instructions waiting on this label to be recorded into data
}

//LabelToIndex := make(map[string]int)

/*
	This function accepts the .palsm file as a long string and lexes through it.
	The lexer starts in the START state and will stop running once it hits the end of the string.
	It returns back the lexed array.
*/
func (lexer *Lexer) Lex(data string) []uint32 {

	data += "\n" // Add in one-last newline just in case

	lexemes := make([]uint32, len(data))

	lexer.LabelToIndex = make(map[string]int)
	lexer.LabelToInstructions = make(map[string][]int)

	if len(data) == 0 {
		return lexemes
	}
	lexer.LexemesIndex = 0
	dumping := false
	lexer.Line++
	for {
		if lexer.Index >= len(data) {
			lexer.Current_State = END
		}
		switch lexer.Current_State {
		case START: // Beginning of a new command, consume all whitespace until a comment is hit or a command
			lexer.BuiltString = ""
			if IsWhitespace(data[lexer.Index]) { // Is it whitespace?
				// Do nothing
			} else if (data[lexer.Index] == '/' && data[lexer.Index+1] == '/') || (data[lexer.Index] == '/' && data[lexer.Index+1] == '*') { // Is it a comment?
				if data[lexer.Index+1] == '*' {
					lexer.BeginningChar = '*'
				}
				lexer.Index++
				lexer.Current_State = COMMENT
			} else { // It must be a command
				lexer.BuiltString += string(data[lexer.Index])
				lexer.Current_State = BUILDCOMM
			}
			break
		case COMMENT: // You're in a comment line, Consume until you hit end of line
			if lexer.BeginningChar == '*' && data[lexer.Index:lexer.Index+2] == "*/" {
				lexer.Current_State = START
				lexer.BeginningChar = 0
				lexer.Index++
			} else if lexer.BeginningChar != '*' && (data[lexer.Index] == '\n' || (lexer.Index < len(data)-1 && data[lexer.Index:lexer.Index+2] == "\r\n")) {
				lexer.Current_State = START
			}
			break
		case BUILDCOMM: // Build command/Parameter
			if IsWhitespace(data[lexer.Index]) || data[lexer.Index] == ':' { // You hit end of command or label, stop reading characters
				if data[lexer.Index] == ':' {
					lexer.BuiltString += ":"
				}
				lexer.Current_State = DUMP
				dumping = true
			} else {
				lexer.BuiltString += string(data[lexer.Index])
				if lexer.Index == len(data)-1 {
					lexer.Current_State = DUMP
					dumping = true
				}
			}
			break
		case DUMP: // Ready to interpret the built string as a command/Parameter
			lexer.GetCommand(&lexemes)
			lexer.Current_State = START
			break
		case END:
			lexer.DumpCommand(&lexemes)
			lexer.VerifyLabelResolution()
			lexemes[lexer.LexemesIndex] = 0x40000000
			return lexemes
		}
		if dumping {
			dumping = false
		} else {
			if lexer.Index < len(data)-1 && data[lexer.Index:lexer.Index+2] == "\r\n" {
				lexer.Line++
				lexer.Index++
			} else if data[lexer.Index] == '\n' {
				lexer.Line++
			}
			lexer.Index++
		}
	}
}

func (lexer *Lexer) VerifyLabelResolution() {
	if len(lexer.LabelToInstructions) > 0 {
		fmt.Println("ERROR: Unresolved labels exist in your code, they go as follows:")
		for label, _ := range lexer.LabelToInstructions {
			fmt.Printf("  \"%s\"\n", label)
		}
		os.Exit(1)
	}
}

func IsNumeric(num string) bool {
	for i, char := range num {
		if i == 0 && char == '-' {
			continue
		} else {
			if char < '0' || char > '9' {
				return false
			}
		}
	}
	return true
}

func ValidateNumParameter(command uint32, paramIndex int) bool {
	switch command {
	case 0x40000009:
		return false
	case 0x4000000A:
		if paramIndex == 0 {
			return false
		}
	case 0x40000011:
		return false
	}
	return true
}

func (lexer *Lexer) HandleLabelDecleration(lexemes *[]uint32) {
	if len(lexer.BuiltString) == 1 {
		fmt.Printf("ERROR: Label decleration on line %d cannot be empty.\n", lexer.Line)
		os.Exit(1)
	}

	lexer.BuiltString = lexer.BuiltString[:len(lexer.BuiltString)-1]

	if _, ok := lexer.LabelToIndex[lexer.BuiltString]; ok {
		fmt.Printf("ERROR: Label '%s' is declared more than once.\n", lexer.BuiltString)
		os.Exit(1)
	} else {
		lexer.LabelToIndex[lexer.BuiltString] = lexer.LexemesIndex
	}

	if val, ok := lexer.LabelToInstructions[lexer.BuiltString]; ok {
		for _, i := range val {
			(*lexemes)[i] = uint32(lexer.LexemesIndex)
		}
		delete(lexer.LabelToInstructions, lexer.BuiltString)
	}
}

func (lexer *Lexer) HandleLabelParameter(lexemes *[]uint32) {
	if len(lexer.BuiltString) == 1 {
		fmt.Printf("ERROR: Label reference on line %d cannot be empty.\n", lexer.Line)
		os.Exit(1)
	}

	if indexOfLabel, ok := lexer.LabelToIndex[lexer.BuiltString]; ok {
		lexer.Parameters[lexer.ParametersIndex] = uint32(indexOfLabel)
	} else {
		lexer.Parameters[lexer.ParametersIndex] = 0
		if instructions, ok := lexer.LabelToInstructions[lexer.BuiltString]; ok {
			lexer.LabelToInstructions[lexer.BuiltString] = append(instructions, lexer.LexemesIndex)
		} else {
			lexer.LabelToInstructions[lexer.BuiltString] = []int{lexer.LexemesIndex}
		}
	}
	lexer.ParametersIndex++
	lexer.NumParams++
}

func (lexer *Lexer) DumpCommand(lexemes *[]uint32) {
	if lexer.Parameters != nil {
		if len(lexer.Parameters) != int(lexer.NumParams) {
			fmt.Printf("ERROR: Command on line %d was expecting %d parameters, received %d.\n", lexer.Line, len(lexer.Parameters), lexer.NumParams)
			os.Exit(1)
		}
		for _, param := range lexer.Parameters {
			(*lexemes)[lexer.LexemesIndex] = param
			lexer.LexemesIndex++
		}
		(*lexemes)[lexer.LexemesIndex] = lexer.CurrentInstruction
		lexer.LexemesIndex++
	}
	lexer.ParametersIndex = 0
	lexer.NumParams = 0
	lexer.CurrentInstruction = 0
}

func (lexer *Lexer) GetCommand(lexemes *[]uint32) {

	// Check if label decleration -- if so, handle it
	LabelDecleration := false
	if lexer.BuiltString[len(lexer.BuiltString)-1] == ':' {
		lexer.DumpCommand(lexemes)
		lexer.HandleLabelDecleration(lexemes)
		lexer.Parameters = nil
		LabelDecleration = true
		return
	}
	// Check if it's an integer
	if num, err := strconv.Atoi(lexer.BuiltString); err == nil {
		if num > 1073741823 || num < -1073741823 {
			fmt.Printf("ERROR: Max absolute int value is '%d'.", 1073741823)
			os.Exit(1)
		}
		num = num & 0xBFFFFFFF

		if lexer.ParametersIndex == len(lexer.Parameters) {
			fmt.Printf("ERROR: Command on line %d was expecting %d parameters, received %d.\n", lexer.Line, len(lexer.Parameters), lexer.NumParams+1)
			os.Exit(1)
		}

		if !ValidateNumParameter(lexer.CurrentInstruction, lexer.ParametersIndex) {
			// Throw error if it's a register command instruction
			fmt.Printf("ERROR: Command on line %d was expecting a register as it's parameter.\n", lexer.Line)
			os.Exit(1)
		}

		lexer.Parameters[lexer.ParametersIndex] = uint32(num)
		lexer.ParametersIndex++

		lexer.NumParams++

		return
	} else if IsNumeric(lexer.BuiltString) {
		fmt.Printf("ERROR: Max absolute int value is '%d'.", 1073741823)
		os.Exit(1)
	}

	//Check if it's a register
	if res, _ := regexp.MatchString("^R[0-9]$", lexer.BuiltString); res {
		if lexer.BuiltString[1] == '0' {
			fmt.Println("ERROR: R0 is not a valid register")
			os.Exit(1)
		}

		if lexer.CurrentInstruction == 0x40000011 || lexer.CurrentInstruction == 0x40000012 {
			fmt.Printf("ERROR: A valid label was expected on line %d.\n", lexer.Line)
			os.Exit(1)
		}

		if lexer.ParametersIndex == len(lexer.Parameters) {
			fmt.Printf("ERROR: Command on line %d was expecting %d parameters, received %d.\n", lexer.Line, len(lexer.Parameters), lexer.NumParams)
			os.Exit(1)
		}

		reg := uint32((lexer.BuiltString[1] - '1') & 15)
		lexer.Parameters[lexer.ParametersIndex] = uint32(uint32(0b11<<30) | reg)
		lexer.ParametersIndex++

		lexer.NumParams++

		return
	}

	// Check if it's the parameter trying to be passed in is to a jump-variant command
	if (lexer.CurrentInstruction == 0x40000011 || lexer.CurrentInstruction == 0x40000012) && lexer.NumParams == 0 {
		lexer.HandleLabelParameter(lexemes)
		return
	}

	// Start new command, dump previous command if it exists
	lexer.DumpCommand(lexemes)

	var numParams int
	switch lexer.BuiltString {
	case "HALT":
		lexer.CurrentInstruction = 0x40000000
		numParams = 0
	case "PEEK":
		lexer.CurrentInstruction = 0x40000001
		numParams = 0
	case "ADD":
		lexer.CurrentInstruction = 0x40000002
		numParams = 2
	case "SUB":
		lexer.CurrentInstruction = 0x40000003
		numParams = 2
	case "MUL":
		lexer.CurrentInstruction = 0x40000004
		numParams = 2
	case "DIV":
		lexer.CurrentInstruction = 0x40000005
		numParams = 2
	case "AND":
		lexer.CurrentInstruction = 0x40000006
		numParams = 2
	case "OR":
		lexer.CurrentInstruction = 0x40000007
		numParams = 2
	case "PUSH":
		lexer.CurrentInstruction = 0x40000008
		numParams = 1
	case "POP":
		lexer.CurrentInstruction = 0x40000009
		numParams = 1
	case "MOV":
		lexer.CurrentInstruction = 0x4000000A
		numParams = 2
	case "EQ":
		lexer.CurrentInstruction = 0x4000000B
		numParams = 2
	case "NEQ":
		lexer.CurrentInstruction = 0x4000000C
		numParams = 2
	case "GT":
		lexer.CurrentInstruction = 0x4000000D
		numParams = 2
	case "LT":
		lexer.CurrentInstruction = 0x4000000E
		numParams = 2
	case "GTE":
		lexer.CurrentInstruction = 0x4000000F
		numParams = 2
	case "LTE":
		lexer.CurrentInstruction = 0x40000010
		numParams = 2
	case "JMP":
		lexer.CurrentInstruction = 0x40000011
		numParams = 1
	case "JMPF":
		lexer.CurrentInstruction = 0x40000012
		numParams = 1
	default:
		if !LabelDecleration {
			fmt.Printf("ERROR: Unrecognized command '%s'.", lexer.BuiltString)
			os.Exit(1)
		}
	}
	if LabelDecleration {
		lexer.Parameters = nil
	} else {
		lexer.Parameters = make([]uint32, numParams) // Max number of parameters a command can have
	}
}

func IsWhitespace(char byte) bool {
	switch char {
	case ' ', '\t', '\n', '\f', '\r', '\v':
		return true
	default:
		return false
	}
}
