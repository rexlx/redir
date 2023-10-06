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
	local   = flag.Bool("local", false, "Use local syslog")
)

func main() {
	flag.Parse()

	if *local {
		p := "local"
		proto = &p
	}
	syslog, err := chooseWriter(*proto, *url, *id)
	err = scanAndWriteToSyslog(*size, os.Stdin, syslog)
	if err != nil {
		log.Fatalln(err)
	}
}

func chooseWriter(proto, url, id string) (*syslog.Writer, error) {
	switch proto {
	case "udp":
		return syslog.Dial("udp", url, syslog.LOG_INFO, id)
	case "tcp":
		return syslog.Dial("tcp", url, syslog.LOG_INFO, id)
	default:
		return syslog.New(syslog.LOG_INFO, id)
	}
}

func writeToSyslog(w *syslog.Writer, data []byte, verbose bool) error {

	n, err := w.Write(data)
	if err != nil {
		return err
	}
	if verbose {
		fmt.Println("written:", n)
	}
	return nil
}

func scanAndWriteToSyslog(size int, r io.Reader, w *syslog.Writer) error {
	// Create a new bufio reader to read from stdin
	bufReader := bufio.NewReader(r)

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

		err = writeToSyslog(w, chunk[:n], *verbose)
		if err != nil {
			return err
		}
	}

	return nil
}
