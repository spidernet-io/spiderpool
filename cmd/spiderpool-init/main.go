package main

import (
	"fmt"
	"time"
	"os"
)

func main() {
	time.Sleep(time.Minute)
	fmt.Printf(" I am init-config")
	time.Sleep(time.Minute)
	fmt.Fprintf(os.Stdout, "bye bye ")
}
