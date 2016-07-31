package main

import (
    "bufio"
    "encoding/binary"
    "fmt"
    "os"
)

func main() {
    execute("/Users/brandon/fuck-chrome/chunk.mp4")
}

func check(e error) {
    if e != nil {
        panic(e)
    }
}

func copyBytes (sourceFile *os.File, destinationFile *os.File, position int64, sizeOfAtom int64) {
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

    parseAtoms(0, fileSize, sourceFile, destinationFile)

    destinationFile.Close()
}

var atoms = map[string]bool {"moov": true}

func parseAtoms(position int64, size int64, sourceFile *os.File, destinationFile *os.File) {
    startPosition := position
    for position < startPosition + size {
        position = parseAtom(position, sourceFile, destinationFile)
        pause()
    }
}

func parseAtom(position int64, sourceFile *os.File, destinationFile *os.File) int64 {
	buffer := make([]byte, 4)
    sourceFile.ReadAt(buffer, position)

    sizeOfAtom := int64(binary.BigEndian.Uint32(buffer))

    sourceFile.ReadAt(buffer, position + 4)
    typeOfAtom := string(buffer)

    fmt.Printf("Atom %s @ %d size %d\n", typeOfAtom, position, sizeOfAtom)
    if atoms[typeOfAtom] {
        parseAtoms(position + 8, sizeOfAtom, sourceFile, destinationFile)
    } else {
        copyBytes(sourceFile, destinationFile, position, sizeOfAtom)
    }

    return position + int64(sizeOfAtom)
}

func pause() {
    bufio.NewReader(os.Stdin).ReadBytes('\n')
}
