package palsm_h

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
)

func ReadFile(filePath string) string {
	file, err := os.Open(filePath)

	if err != nil {
		panic(err)
	}
	defer file.Close()

	stats, statsErr := file.Stat()
	if statsErr != nil {
		panic(statsErr)
	}

	if filepath.Ext(filePath) != ".palsm" {
		fmt.Println("ERROR: File does not have .palsm extension")
		os.Exit(1)
		return ""
	}

	var filesize int64 = stats.Size()
	bytes := make([]byte, filesize)

	buff := bufio.NewReader(file)
	_, err = buff.Read(bytes)

	data := ""
	for i := range bytes {
		data += string(bytes[i])
	}

	return data
}

func WriteBinaryFile(filePath string, instructions []uint32) {
	fileName := filePath[0 : len(filePath)-len(filepath.Ext(filePath))]

	file, err := os.Create(fileName + ".bin")

	if err != nil {
		panic(err)
	}

	for _, inst := range instructions {
		buff := make([]byte, 4)
		for i := 3; i >= 0; i-- {
			buff[3-i] = byte((inst >> (8 * i)) & 0xFF)
		}
		numOfBytes, err := file.Write(buff)
		if numOfBytes != 4 || err != nil {
			fmt.Println("ERROR: Something went wrong when writing to", fileName+".bin")
			os.Exit(1)
		}
	}

	file.Close()
}
