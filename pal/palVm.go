package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"palsm/palexer"
	palsm "palsm/palsm_h"
	"path/filepath"
)

// Data fields
var deleteBin bool
var MemRegisters [9]int32 // Registers R0-R9
var FlagRegister bool

// Type definitions
type MemStack struct {
	sp    uint32
	bp    uint32
	stack []int32
	ip    int // instruction pointer
}

func (stack *MemStack) push(val int32) {
	stack.stack[stack.sp] = val
	stack.sp++
}

func (stack *MemStack) pop() int32 {
	if stack.sp < stack.bp {
		return 0
	}
	stack.sp--
	val := stack.stack[stack.sp]
	return val
}

func (stack *MemStack) peek() int32 {
	if stack.sp == stack.bp {
		return 0
	}
	return stack.stack[stack.sp-1]
}

type ArithmeticOperation int
type BooleanOperation int

// Constants
const (
	ADD  ArithmeticOperation = 0
	SUB  ArithmeticOperation = 1
	MUL  ArithmeticOperation = 2
	DIV  ArithmeticOperation = 3
	AND  BooleanOperation    = 0
	OR   BooleanOperation    = 1
	PUSH ArithmeticOperation = 6
	POP  ArithmeticOperation = 7
	MOV  ArithmeticOperation = 8
	EQ   BooleanOperation    = 2
	NEQ  BooleanOperation    = 3
	GT   BooleanOperation    = 4
	LT   BooleanOperation    = 5
	GTE  BooleanOperation    = 6
	LTE  BooleanOperation    = 7
)

// Initialize MemStack
func InitMemStack(pointer uint32, size uint64) MemStack {
	memStack := MemStack{bp: pointer, sp: pointer, stack: make([]int32, size)}
	return memStack
}

// VMExecute function
/*
	Run through each 32-bit instruction, with two most significant bits reserved to represent data as such:
		0 -> positive int
		1 -> OP_Code
		2 -> negative int
		3 -> register
	This leaves bits 20 through 0 (big-endian form) to be understood as the actual instruction
*/
func VMExecute(dataStream []uint32, memStack *MemStack) {
	for d := 0; d < len(dataStream); d++ {
		dataType := (dataStream[d] & 0xC0000000) >> 30
		data := int32(dataStream[d] & 0x3FFFFFFF)
		if dataType%2 == 0 { // It's an int, add back in the datatype to the left-most bits to make it pos/neg
			if dataType == 2 {
				dataType = 3 // Make it negative (from 0xBFFFFFFF to 0xFFFFFFFF)
			}
			memStack.push((int32(dataType) << 30) | data)
			//fmt.Printf("Pushed %d to the stack\n", data)
		} else if dataType == 3 { // It's a register
			memStack.push((int32(dataType) << 30) | data)
		} else {
			reassignIndex := VMExecuteOpCode(uint32(data), memStack)
			if reassignIndex {
				d = memStack.ip - 1
			} else {
				memStack.ip++
			}
		}
	}
}

/*
	Register help functions
*/
// Check if value is a register (bytes 31 and 30 are 0b11)
func CheckIfRegister(val uint32) bool {
	return (val&0xC0000000)>>30 == 3
}

// Store value in appropriate register
func StoreInRegister(reg int32, val int32) {
	MemRegisters[reg] = val
}

// ArithmeticHelp
func ExecuteArithmatic(val1 int32, val2 int32, op ArithmeticOperation, memStack *MemStack) int32 {
	switch op {
	case ADD:
		return val1 + val2
	case SUB:
		return val1 - val2
	case MUL:
		return val1 * val2
	case DIV:
		return val1 / val2
	case PUSH:
		return val2
	case POP:
		return memStack.pop()
	}
	return 0
}

func ArithmeticOperationHelper(val1 int32, val2 int32, op ArithmeticOperation, memStack *MemStack) {
	if CheckIfRegister(uint32(val1)) {
		reg := val1 & 0x3FFFFFFF           // Get register address
		val1 = MemRegisters[reg]           // Change val1 to be the value stored in it's register
		if CheckIfRegister(uint32(val2)) { // Determine if the second parameter is a register
			val2 = MemRegisters[val2&0x3FFFFFFF]
		}
		StoreInRegister(reg, ExecuteArithmatic(val1, val2, op, memStack))
	} else {
		if CheckIfRegister(uint32(val2)) { // Determine if the second parameter is a register
			val2 = MemRegisters[val2&0x3FFFFFFF]
		}
		memStack.push(ExecuteArithmatic(val1, val2, op, memStack))
	}
}

// Boolean Operation Helper functions
func ExecuteBooleanOperation(val1 int32, val2 int32, op BooleanOperation) bool {
	switch op {
	case AND:
		return (val1 & val2) != 0
	case OR:
		return (val1 | val2) != 0
	case GT:
		return val1 > val2
	case LT:
		return val1 < val2
	case GTE:
		return val1 >= val2
	case LTE:
		return val1 <= val2
	case EQ:
		return val1 == val2
	case NEQ:
		return val1 != val2
	}
	return false
}
func BooleanOperationHelper(val1 int32, val2 int32, op BooleanOperation) {
	if CheckIfRegister(uint32(val1)) {
		val1 = MemRegisters[val1&0x3FFFFFFF]
	}
	if CheckIfRegister(uint32(val2)) {
		val2 = MemRegisters[val2&0x3FFFFFFF]
	}

	FlagRegister = ExecuteBooleanOperation(val1, val2, op)
}

// VMExecuteOpCode function
/*
	OPCodes:
		0 -> Halt
		1 -> Peek stack
		2 -> Addition
		3 -> Subtraction
		4 -> Multiplication
		5 -> Division
		6 -> AND
		7 -> OR
		8 -> PUSH
		9 -> POP
*/
func VMExecuteOpCode(instruction uint32, memStack *MemStack) bool {
	switch instruction {
	case 0: // HALT
		fmt.Printf("[0x%X] Halt", memStack.ip)
		if deleteBin {
			if err := os.Remove(os.Args[1][0:len(os.Args[1])-5] + "bin"); err != nil {
				fmt.Printf("\n%s", err.Error())
				os.Exit(1)
			}
		}
		os.Exit(0)
		break
	case 1: // PEEK
		fmt.Printf("[0x%X] Top of stack is: %d\n", memStack.ip, memStack.peek())
	case 2: // ADD
		val2, val1 := memStack.pop(), memStack.pop()
		ArithmeticOperationHelper(val1, val2, ADD, memStack)
	case 3: // SUB
		val2, val1 := memStack.pop(), memStack.pop()
		ArithmeticOperationHelper(val1, val2, SUB, memStack)
	case 4: // MUL
		val2, val1 := memStack.pop(), memStack.pop()
		ArithmeticOperationHelper(val1, val2, MUL, memStack)
	case 5: // DIV
		val2, val1 := memStack.pop(), memStack.pop()
		ArithmeticOperationHelper(val1, val2, DIV, memStack)
	case 6: // AND
		val2, val1 := memStack.pop(), memStack.pop()
		BooleanOperationHelper(val1, val2, AND)
	case 7: // OR
		val2, val1 := memStack.pop(), memStack.pop()
		BooleanOperationHelper(val1, val2, OR)
	case 8: // PUSH
		val := memStack.pop()
		ArithmeticOperationHelper(0, val, PUSH, memStack)
	case 9: // POP
		val := memStack.pop()
		ArithmeticOperationHelper(val, 0, POP, memStack)
	case 10: // MOV
		val2, val1 := memStack.pop(), memStack.pop()
		StoreInRegister(val1&0x3FFFFFFF, val2)
	case 11: // EQ
		val2, val1 := memStack.pop(), memStack.pop()
		BooleanOperationHelper(val1, val2, EQ)
	case 12: // NEQ
		val2, val1 := memStack.pop(), memStack.pop()
		BooleanOperationHelper(val1, val2, EQ)
	case 13: // GT
		val2, val1 := memStack.pop(), memStack.pop()
		BooleanOperationHelper(val1, val2, EQ)
	case 14: // LT
		val2, val1 := memStack.pop(), memStack.pop()
		BooleanOperationHelper(val1, val2, EQ)
	case 15: // GTE
		val2, val1 := memStack.pop(), memStack.pop()
		BooleanOperationHelper(val1, val2, EQ)
	case 16: // LTE
		val2, val1 := memStack.pop(), memStack.pop()
		BooleanOperationHelper(val1, val2, EQ)
	case 17: // JMP
		index := memStack.pop()
		memStack.ip = int(index)
		return true
	case 18: // JMP
		index := memStack.pop()
		if FlagRegister {
			memStack.ip = int(index)
			return true
		}
		return false
	}
	return false
}

// Read binary file
/*
	Use for reading binary file.
	Break down .bin file into bytes, and then for each 4 bytes store as uint32 into a
	list to return.
*/
func ReadBinaryFile(fileName string) []uint32 {
	file, err := os.Open(fileName)

	if err != nil {
		panic(err)
	}

	stats, statsErr := file.Stat()
	if statsErr != nil {
		panic(statsErr)
	}

	var filesize int64 = stats.Size()
	bytes := make([]byte, filesize)

	buff := bufio.NewReader(file)
	_, err = buff.Read(bytes)

	instructionList := make([]uint32, len(bytes)/4)
	i := 0
	nextInstruction := 0
	for {
		if i >= len(bytes) {
			break
		}

		data := bytes[i : i+4]

		instructionList[nextInstruction] = binary.BigEndian.Uint32(data)

		nextInstruction++
		i += 4
	}
	file.Close()
	return instructionList
}

// Main function
func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: ./pal <file.palsm>|<file.bin>")
		os.Exit(1)
	}

	// Instantiate machine state
	var memStack MemStack
	memStack = InitMemStack(0, 1000000)

	deleteBin = false

	var data []uint32
	if filepath.Ext(os.Args[1]) == ".palsm" { // assemble it here, create a temp binary file
		deleteBin = true

		palsmData := palsm.ReadFile(os.Args[1])

		lexer := palexer.Lexer{Current_State: palexer.START, Index: 0}
		instructions := lexer.Lex(palsmData)

		if len(instructions) == 0 {
			os.Exit(0)
		}

		palsm.WriteBinaryFile(os.Args[1], instructions)

		data = ReadBinaryFile(os.Args[1][0:len(os.Args[1])-5] + "bin")
	} else {
		// Read in the .bin file
		data = ReadBinaryFile(os.Args[1])
	}

	VMExecute(data, &memStack)

	os.Exit(0)
}
