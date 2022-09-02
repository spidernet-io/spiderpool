// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	time.Sleep(time.Minute)
	fmt.Printf(" I am init-config")
	time.Sleep(time.Minute)
	fmt.Fprintf(os.Stdout, "bye bye ")
}
