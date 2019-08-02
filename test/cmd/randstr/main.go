package main

import (
	"flag"
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyz"
	defaultLength = 8
)

var lengthStr string

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
	flag.StringVar(&lengthStr, "length", "", "Length of random string.")
}

func main() {
	flag.Parse()
	var n int
	if lengthStr != "" {
		var err error
		if n, err = strconv.Atoi(lengthStr); err != nil {
			n = 0
		}
	}
	if n <= 0 {
		n = defaultLength
	}
	fmt.Print(RandomString(n))
}

func RandomString(n int) string {
	r := make([]byte, n)
	for i := range r {
		r[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(r)
}
