package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"log/syslog"
	"os"
)

var (
	size    = flag.Int("size", 1024, "Size of the chunk to read from stdin")
	url     = flag.String("url", "127.0.0.1:514", "URL of the syslog server")
	proto   = flag.String("proto", "udp", "Protocol to use to send the data")
	id      = flag.String("id", "redir", "ID to use in the syslog message")
	verbose = flag.Bool("verbose", false, "Verbose mode")
)

func main() {
	flag.Parse()
	err := scanAndWriteToSyslog(*size, os.Stdin)
	if err != nil {
		log.Fatalln(err)
	}
}

func writeToSyslog(data []byte, verbose bool) error {
	syslog, err := syslog.Dial(*proto, *url, syslog.LOG_INFO, *id)
	if err != nil {
		return err
	}
	defer syslog.Close()
	n, err := syslog.Write(data)
	if err != nil {
		return err
	}
	if verbose {
		fmt.Println("written:", n)
	}
	return nil
}

func scanAndWriteToSyslog(size int, reader io.Reader) error {
	// Create a new bufio reader to read from stdin
	bufReader := bufio.NewReader(reader)

	// Read data from stdin in 4k chunks and write it to syslog
	for {
		chunk := make([]byte, size)
		n, err := bufReader.Read(chunk)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		err = writeToSyslog(chunk[:n], *verbose)
		if err != nil {
			return err
		}
	}

	return nil
}
