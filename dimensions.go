package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"unsafe"
)

type Dimensions struct {
	HeightPosition     int64
	Height             [4]byte
	WidthPosition      int64
	Width              [4]byte
	SwapHeightAndWidth uint8 //only using 0 and 1 but we gotta do this for binary serialization
}

type HttpFileReader struct {
	URL    url.URL
	client http.Client
}

func (r *HttpFileReader) NewHttpFileReader() {
	r.client = http.Client{
		Timeout: time.Second * 10,
	}
}

func (r *HttpFileReader) Size() (s int64, err error) {
	res, err := http.Head(r.URL.String())
	defer res.Body.Close()
	if err != nil {
		return
	}

	contentLength := res.Header.Get("Content-Length")
	if contentLength == "" {
		return -1, errors.New("Content-Length Header not found")
	}

	s, err = strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return
	}

	return s, nil
}

func (r *HttpFileReader) ReadAt(b []byte, off int64) (n int, err error) {
	req, err := http.NewRequest("GET", r.URL.String(), nil)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, (off+int64(len(b)))))

	res, err := r.client.Do(req)
	defer res.Body.Close()
	if err != nil {
		return
	}

	n, err = res.Body.Read(b)
	return
}

func calculateDimensions(url *url.URL) *Dimensions {
	reader := &HttpFileReader{
		URL: *(url),
	}

	fileSize, err := reader.Size()
	fmt.Printf("FileSize: %d\n", fileSize)
	check(err)

	return parseAtoms(0, fileSize, 0, reader)
}

var atoms = map[string]bool{"moov": true, "trak": true}

type Atom struct {
	Length uint32
	Type   [4]byte
}

type FullBox struct {
	Atom
	Version uint8
	Flags   [3]byte
}

type FixedPoint32 struct {
	IntegerPart    uint16
	FractionalPart uint16
}

type TrackHeaderBoxCommon struct {
	Reserved2      [2]uint32
	Layer          int16
	AlternateGroup int16
	Volume         int16
	Reserved3      uint16
	Matrix         [3][3]FixedPoint32
	Width          [4]byte
	Height         [4]byte
}

type TrackHeaderBox interface {
	Read(r io.Reader) error
	Dimensions(atomPosition int64) *Dimensions
}

type TrackHeaderBoxVersion0 struct {
	FullBox
	CreationTime     uint32
	ModificationTime uint32
	TrackID          uint32
	Reserved         uint32
	Duration         uint32
	TrackHeaderBoxCommon
}

func (b *TrackHeaderBoxVersion0) Read(r io.Reader) error {
	err := binary.Read(r, binary.BigEndian, b)
	if err != nil {
		return err
	}

	return nil
}

func (b *TrackHeaderBoxVersion0) Dimensions(atomPosition int64) *Dimensions {
	return &Dimensions{
		Width:              b.Width,
		WidthPosition:      atomPosition + int64(unsafe.Offsetof(TrackHeaderBoxVersion0{}.Width)),
		Height:             b.Height,
		HeightPosition:     int64(unsafe.Offsetof(TrackHeaderBoxVersion0{}.Height)) + atomPosition,
		SwapHeightAndWidth: calculateSwapHeightAndWidth(b.Matrix),
	}
}

type TrackHeaderBoxVersion1 struct {
	FullBox
	CreationTime     uint64
	ModificationTime uint64
	TrackID          uint32
	Reserved         uint32
	Duration         uint64
	TrackHeaderBoxCommon
}

func (b *TrackHeaderBoxVersion1) Read(r io.Reader) error {
	err := binary.Read(r, binary.BigEndian, b)
	if err != nil {
		return err
	}

	return nil
}

func (b *TrackHeaderBoxVersion1) Dimensions(atomPosition int64) *Dimensions {
	return &Dimensions{
		Width:              b.Width,
		WidthPosition:      atomPosition + int64(unsafe.Offsetof(TrackHeaderBoxVersion1{}.Width)),
		Height:             b.Height,
		HeightPosition:     int64(unsafe.Offsetof(TrackHeaderBoxVersion1{}.Height)) + atomPosition,
		SwapHeightAndWidth: calculateSwapHeightAndWidth(b.Matrix),
	}
}

type ReaderAt interface {
	ReadAt(b []byte, off int64) (n int, err error)
}

func parseAtoms(position int64, size int64, level int, r ReaderAt) *Dimensions {
	startPosition := position
	for position < startPosition+size {
		nextPosition, dimensions := parseAtom(position, level, r)
		if dimensions != nil {
			return dimensions
		}
		position = nextPosition
	}

	return nil
}

func parseAtom(position int64, level int, r ReaderAt) (nextPosition int64, d *Dimensions) {
	headerSize := int64(unsafe.Sizeof(Atom{}))
	buffer := make([]byte, headerSize)
	r.ReadAt(buffer, position)

	atom := Atom{}
	err := binary.Read(bytes.NewReader(buffer), binary.BigEndian, &atom)
	check(err)

	atomLength := int64(atom.Length)
	atomType := string(atom.Type[:])

	//fmt.Printf("%sAtom %s @ %d size %d\n", strings.Repeat("\t", level), atomType, position, atomLength)
	if atoms[atomType] {
		dimensions := parseAtoms(position+headerSize, atomLength-headerSize, level+1, r)
		if dimensions != nil {
			return -1, dimensions
		}
	} else if atomType == "tkhd" {
		atomBytes := make([]byte, atomLength)
		r.ReadAt(atomBytes, position)

		fullBox := FullBox{}
		err := binary.Read(bytes.NewReader(atomBytes), binary.BigEndian, &fullBox)
		check(err)

		var box TrackHeaderBox
		if fullBox.Version == 0 {
			box = new(TrackHeaderBoxVersion0)
		} else if fullBox.Version == 1 {
			box = new(TrackHeaderBoxVersion1)
		} else {
			panic("Unable to parse tkhd, unknown Version")
		}

		err = box.Read(bytes.NewReader(atomBytes))
		check(err)

		d = box.Dimensions(position)

		return -1, d
	}

	return position + int64(atomLength), nil
}

func calculateSwapHeightAndWidth(matrix [3][3]FixedPoint32) uint8 {
	if matrix[0][1].IntegerPart != 0 && matrix[1][0].IntegerPart != 0 {
		return 1
	}

	return 0
}
