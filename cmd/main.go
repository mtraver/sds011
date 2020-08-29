package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/mtraver/sds011"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s -listen|-query serial_port\n", os.Args[0])

		fmt.Fprintf(flag.CommandLine.Output(), "Positional Arguments:\n")
		fmt.Fprintf(flag.CommandLine.Output(), `  serial_port
    	Name of the sensor's serial port. Required.
`)

		fmt.Fprintf(flag.CommandLine.Output(), "\nFlags:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  Exactly one of the following flags must be given.\n\n")
		flag.PrintDefaults()
	}
}

func main() {
	listenFlag := flag.Bool("listen", false, "listen mode")
	queryFlag := flag.Bool("query", false, "query mode")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 || (*listenFlag == *queryFlag) {
		flag.Usage()
		os.Exit(2)
	}

	d, err := sds011.New(args[0])
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	if err := d.Wake(); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	if fw, err := d.GetFirmwareVersion(); err != nil {
		log.Println(err)
		os.Exit(1)
	} else {
		log.Printf("Firmware version: %v\n", fw)
	}

	if *queryFlag {
		m, err := active(d)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
		log.Printf("%v\n", m)
	} else if *listenFlag {
		listen(d)
	} else {
		log.Println("Error: No known flags given")
		os.Exit(2)
	}
}

func active(d sds011.Dev) (sds011.Measurement, error) {
	if err := d.SetMode(sds011.ModeQuery); err != nil {
		return sds011.Measurement{}, err
	}

	log.Println("Warming up")
	time.Sleep(10 * time.Second)
	log.Println("Querying")

	m, err := d.Sense()
	if err != nil {
		return sds011.Measurement{}, err
	}

	time.Sleep(1 * time.Second)

	if err := d.Sleep(); err != nil {
		return sds011.Measurement{}, err
	}

	return m, err
}

func listen(d sds011.Dev) error {
	if err := d.SetMode(sds011.ModeActive); err != nil {
		return err
	}

	if err := d.SetPeriod(0); err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		log.Println("Listening")
		defer wg.Done()
		if err := d.Listen(handler); err != nil {
			log.Printf("Listen failed: %v\n", err)
		}
	}()

	time.Sleep(2 * time.Second)

	// This will fail because we're already listening.
	err := d.Listen(handler)
	fmt.Printf("Second listen: %v\n", err)

	time.Sleep(7 * time.Second)
	d.Stop()
	wg.Wait()

	return nil
}

func handler(m sds011.Measurement) {
	log.Printf("%v\n", m)
}
