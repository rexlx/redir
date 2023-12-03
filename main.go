package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"log/syslog"
	"net"
	"os"
	"time"

	"github.com/quic-go/quic-go"
)

var (
	size         = flag.Int("size", 1024, "Size of the chunk to read from stdin")
	url          = flag.String("url", "127.0.0.1:514", "URL of the syslog server")
	proto        = flag.String("proto", "udp", "Protocol to use to send the data")
	id           = flag.String("id", "redir", "ID to use in the syslog message")
	verbose      = flag.Bool("verbose", false, "Verbose mode")
	local        = flag.Bool("local", false, "Use local syslog")
	experimental = flag.Bool("x", false, "Use experimental QUIC")
)

type SecretManager struct {
	QC          *quic.Config
	TC          *tls.Config
	Destination net.Addr
}

func main() {
	flag.Parse()

	logfile := "rider.log"
	f, err := os.OpenFile(logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln(err)
	}
	myLogger := log.New(f, "rider -", log.LstdFlags)

	if *experimental {
		addr, err := net.ResolveUDPAddr("udp", *url)
		if err != nil {
			log.Fatalln(err)
		}

		sm := &SecretManager{
			QC: newQUICConfig(),
			TC: loadTLSConfig(),
			Destination: &net.UDPAddr{
				IP:   addr.IP,
				Port: addr.Port,
			},
		}

		err = scanAndWriteToQUICStream(myLogger, dialQUIC(*url, sm))
		if err != nil {
			log.Fatalln(err)
		}

		os.Exit(0)
	} else {

		if *local {
			p := "local"
			proto = &p
		}

		syslog, err := chooseWriter(*proto, *url, *id)
		if err != nil {
			log.Fatalln(err)
		}

		err = scanAndWriteToSyslog(*size, os.Stdin, syslog)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func loadTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"rider-protocol"},
	}
}

func newQUICConfig() *quic.Config {
	return &quic.Config{}
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

func dialQUIC(url string, sm *SecretManager) quic.Stream {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second) // 3s handshake timeout
	defer cancel()

	conn, err := quic.DialAddr(ctx, url, sm.TC, sm.QC)
	if err != nil {
		log.Fatalln(err)
	}

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	return stream
}

func scanAndWriteToQUICStream(logger *log.Logger, stream quic.Stream) error {
	logger.Println("starting")
	bufReader := bufio.NewReader(os.Stdin)
	// stream.SetDeadline(time.Now().Add(30 * time.Second))
	keepAlive := time.NewTicker(10 * time.Second)
	logChan := make(chan string)
	killChan := make(chan bool)

	go func() {
		for {
			select {
			case <-keepAlive.C:
				logger.Println("sending heartbeat")
				_, err := stream.Write([]byte("|beat|"))
				if err != nil {
					logger.Fatalln(err)
				}
			case val := <-logChan:
				logger.Println("sending log")
				_, err := stream.Write([]byte(val))
				if err != nil {
					logger.Fatalln(err)
				}
			case <-killChan:
				logger.Println("killing")
				return
			}
		}
	}()
	for {
		logger.Println("reading")
		chunk := make([]byte, *size)

		n, err := bufReader.Read(chunk)
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}
		logChan <- string(chunk[:n])
	}
	keepAlive.Stop()
	killChan <- true
	log.Println("done")
	return nil
}

func scanAndWriteToSyslog(size int, r io.Reader, w *syslog.Writer) error {
	bufReader := bufio.NewReader(r)

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
