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

	if *experimental {
		// parse addr
		addr, err := net.ResolveUDPAddr("udp", *url)
		if err != nil {
			log.Fatalln(err)
		}
		// create udp conn for client
		// conn, err := net.ListenUDP("udp", nil)
		// if err != nil {
		// 	log.Fatalln(err)
		// }
		// h := &net.UDPAddr{
		// 	IP:   addr.IP,
		// 	Port: addr.Port,
		// }
		// create config
		sm := &SecretManager{
			QC: NewQUICConfig(),
			TC: loadTLSConfig(),
			Destination: &net.UDPAddr{
				IP:   addr.IP,
				Port: addr.Port,
			},
		}
		err = scanAndWriteToQUICStream(dialQUIC(*url, sm))
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

func NewQUICConfig() *quic.Config {
	return &quic.Config{}
}

func chooseWriter(proto, url, id string) (*syslog.Writer, error) {
	switch proto {
	case "udp":
		return syslog.Dial("udp", url, syslog.LOG_INFO, id)
	case "tcp":
		return syslog.Dial("tcp", url, syslog.LOG_INFO, id)
	case "quic":
		return syslog.Dial("quic", url, syslog.LOG_INFO, id) // place holder for real code
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
	// create quic conn
	conn, err := quic.DialAddr(ctx, url, sm.TC, sm.QC)
	if err != nil {
		log.Fatalln(err)
	}
	// create stream
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	return stream
	// tr := quic.Transport{}
	// conn, err := tr.Dial(ctx, sm.Destination, sm.TC, sm.QC)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// sync, err := conn.OpenStreamSync(ctx)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// return sync
}

func scanAndWriteToQUICStream(stream quic.Stream) error {
	// Create a new bufio reader to read from stdin
	bufReader := bufio.NewReader(os.Stdin)

	// Read data from stdin in 4k chunks and write it to syslog
	for {
		chunk := make([]byte, 4096)
		n, err := bufReader.Read(chunk)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		_, err = stream.Write(chunk[:n])
		if err != nil {
			return err
		}
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
