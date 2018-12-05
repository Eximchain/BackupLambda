package main

import (
	"fmt"
	"os"
	"strings"
)

// UNUSED marks anything as unused
func UNUSED(intf interface{}) {}

// DumpEnvironment dumps the environment
func DumpEnvironment() {
	for _, e := range os.Environ() {
		pair := strings.Split(e, "=")
		k := pair[0]
		v := pair[1]
		fmt.Printf("Key: %s, Value: %s\n", k, v)
	}
}
