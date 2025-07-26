package utils

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"

	petname "github.com/dustinkirkland/golang-petname"
)

func NewUsername() string {
	petname.NonDeterministicMode()
	n, _ := rand.Int(rand.Reader, big.NewInt(1000))
	return fmt.Sprintf("%s-%03d", petname.Generate(2, "-"), n.Int64())
}

func BytesToUint16Arr(data []byte) ([]uint16, error) {

	if len(data)%2 != 0 {
		return nil, fmt.Errorf("in BytesToUint16Arr:\nbytes length must be even")
	}

	u16 := make([]uint16, len(data)/2)

	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(data[2*i : 2*i+2])
	}

	return u16, nil

}

func Uint16ArrToBytes(u16 []uint16) []byte {

	data := make([]byte, len(u16)*2)

	for i, val := range u16 {
		binary.LittleEndian.PutUint16(data[2*i:2*i+2], val)
	}

	return data

}
