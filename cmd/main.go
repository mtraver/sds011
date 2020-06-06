package main

import (
	"fmt"
	"os"

	"github.com/mtraver/sds011"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s serial_port\n", os.Args[0])
}

func main() {
	if len(os.Args) != 2 {
		usage()
		os.Exit(2)
	}

	d, err := sds011.New(os.Args[1])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	m, err := d.Sense()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Printf("PM2.5: %v μg/m^3\nPM10:  %v μg/m^3\n", m.PM25, m.PM10)
}
