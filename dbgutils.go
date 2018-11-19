package main

import (
	"fmt"
	"os"
	"strings"
)

func UNUSED(intf interface{}) {}

func DumpEnvironment() {
	for _, e := range os.Environ() {
		pair := strings.Split(e, "=")
		k := pair[0]
		v := pair[1]
		fmt.Printf("Key: %s, Value: %s\n", k, v)
	}
}
