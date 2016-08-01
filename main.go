package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

func main() {
	execute("chunk.mp4")
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func copyBytes(sourceFile *os.File, destinationFile *os.File, position int64, sizeOfAtom int64) {
	bufferLength := sizeOfAtom
	buffer := make([]byte, bufferLength)
	sourceFile.ReadAt(buffer, position)
	destinationFile.Write(buffer)
}

func execute(path string) {
	stat, err := os.Stat(path)
	check(err)
	fileSize := stat.Size()

	sourceFile, err := os.Open(path)
	check(err)

	destinationFile, err := os.Create("fixed.mp4")
	check(err)

	parseAtoms(0, fileSize, 0, sourceFile, destinationFile)

	destinationFile.Close()
}

var atoms = map[string]bool{"moov": true, "trak": true}

func parseAtoms(position int64, size int64, level int, sourceFile *os.File, destinationFile *os.File) {
	startPosition := position
	for position < startPosition+size {
		position = parseAtom(position, level, sourceFile, destinationFile)
	}
}

func parseAtom(position int64, level int, sourceFile *os.File, destinationFile *os.File) (int64) {
	buffer := make([]byte, 4)
	sourceFile.ReadAt(buffer, position)

	atomSize := int64(binary.BigEndian.Uint32(buffer))

	sourceFile.ReadAt(buffer, position+4)
	typeOfAtom := string(buffer)

	fmt.Printf("%sAtom %s @ %d size %d\n", strings.Repeat("\t", level), typeOfAtom, position, atomSize)
	if atoms[typeOfAtom] {
		headerSize := int64(8)
		copyBytes(sourceFile, destinationFile, position, headerSize)
		parseAtoms(position+headerSize, atomSize -headerSize, level+1, sourceFile, destinationFile)
	} else if typeOfAtom == "tkhd" {
		atomBytes := make([]byte, atomSize)
		sourceFile.ReadAt(atomBytes, position)

		width := decodeDimensionFromBinary(atomBytes[84:87])
		height := decodeDimensionFromBinary(atomBytes[88:91])

		destinationFile.Write(atomBytes[0:84])
		destinationFile.Write(encodeDimensionToBinary(height))
		destinationFile.Write(encodeDimensionToBinary(width))
		destinationFile.Write(atomBytes[92:])
	} else {
		copyBytes(sourceFile, destinationFile, position, atomSize)
	}

	return position + int64(atomSize)
}

func decodeDimensionFromBinary(bytes []byte) uint16 {
	return binary.BigEndian.Uint16(bytes[:2])
}

func encodeDimensionToBinary(dimension uint16) []byte {
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint16(bytes, dimension)
	return bytes
}


