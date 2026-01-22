package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	"github.com/adamdecaf/pool_checkr/pkg/mining_notify"

	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

var (
	flagAddresses = flag.String("addresses", "", "Comma separated list of IP:port addresses")
	flagExpected  = flag.String("expected", "", "Comma separated list of address that expect payouts")

	flagCount = flag.Int("count", 0, "Number of mining.notify logs to inspect")
)

func main() {
	flag.Parse()

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Received shutdown signal...")
		cancel()
	}()

	// Connect to each address
	g, ctx := errgroup.WithContext(ctx)

	var addresses []string
	if *flagAddresses != "" {
		addresses = strings.Split(*flagAddresses, ",")
	}
	if len(addresses) == 0 {
		log.Fatal("ERROR: no miner addresses specified, use -addresses")
		return
	}

	var expectedAddresses []string
	if exp := *flagExpected; exp != "" {
		expectedAddresses = strings.Split(exp, ",")
	}

	for _, addr := range addresses {
		g.Go(func() error {
			err := parseLogsForMiningNotify(ctx, *flagCount, strings.TrimSpace(addr), expectedAddresses)
			if err != nil {
				return fmt.Errorf("parsing logs from %s failed: %w", addr, err)
			}
			return nil
		})
	}

	err := g.Wait()
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}

func parseLogsForMiningNotify(ctx context.Context, expectedInspections int, addr string, expectedAddresses []string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "missing port in address"):
			host = addr
			port = "80"

		default:
			return fmt.Errorf("parsing %s failed: %w", addr, err)
		}
	}
	if port == "" {
		port = "80"
	}

	address := fmt.Sprintf("ws://%s:%s/api/ws", host, port)
	log.Printf("INFO: connecting to %s", address)

	c, _, err := websocket.DefaultDialer.Dial(address, nil)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}

	// Shutdown handler
	go func() {
		<-ctx.Done()
		log.Printf("Closing connection to %s due to shutdown", address)
		err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "shutdown"))
		if err != nil {
			log.Printf("write close error: %v", err)
		}
		err = c.Close()
		if err != nil {
			log.Printf("close error: %v", err)
		}
	}()

	reader := bufio.NewReader(&wsReader{conn: c})

	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)

	remainingInspections := max(expectedInspections, 1)
	for scanner.Scan() {
		line := scanner.Text()

		// skip empty lines or do basic cleaning
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse mining.notify lines
		if strings.Contains(line, "mining.notify") {
			remainingInspections--
			parseMiningNotifyLine(line, expectedAddresses)
		}

		// After the expected number of inspections quit
		if remainingInspections == 0 {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) || errors.Is(err, context.Canceled) {
			log.Println("Connection closed")
		} else {
			return fmt.Errorf("scanner error: %w", err)
		}
	}

	return nil
}

func parseMiningNotifyLine(line string, expectedAddresses []string) {
	start := strings.Index(line, "{")
	end := strings.LastIndex(line, "}")

	if start >= 0 && end > start {
		blob := line[start : end+1]

		notify, err := mining_notify.Parse(blob)
		if err != nil {
			fmt.Printf("%v\n", blob)
			log.Printf("ERROR: parsing mining.notify line failed: %v", err)
		}

		if notify != nil {
			log.Printf("INFO: height=%v has %d coinbase outputs", notify.Height, len(notify.CoinbaseOuts))

			missingAddresses := make(map[string]bool)
			for _, cb := range notify.CoinbaseOuts {
				// Skip OP_RETURN outputs
				if cb.Type == "OP_RETURN" {
					continue
				}

				if len(expectedAddresses) > 0 {
					if !slices.Contains(expectedAddresses, cb.Address) {
						missingAddresses[cb.Address] = true
					}
				}

				if cb.ValueSatoshis > 0 {
					log.Printf("INFO: %v (%v) receives %0.8f BTC", cb.Address, cb.Type, cb.ValueBTC)
				}
			}

			if len(expectedAddresses) > 0 {
				switch {
				case len(missingAddresses) == 0:
					log.Printf("INFO: all %d addresses expected are in coinbase output", len(expectedAddresses))

				case len(missingAddresses) > 0:
					log.Printf("ERROR: missing %d addresses from coinbase output", len(missingAddresses))
					for missing := range missingAddresses {
						log.Printf("ERROR: %s was not found in coinbase", missing)
					}
					os.Exit(1)
				}
			}
		}
	}
}

// wsReader lets bufio work directly on websocket messages
type wsReader struct {
	conn *websocket.Conn
	buf  []byte // leftover from previous Read()
}

func (r *wsReader) Read(p []byte) (n int, err error) {
	// If we still have data from previous message → use it first
	if len(r.buf) > 0 {
		n = copy(p, r.buf)
		r.buf = r.buf[n:]
		return n, nil
	}

	messageType, msg, err := r.conn.ReadMessage()
	if err != nil {
		return 0, err
	}

	if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
		return 0, fmt.Errorf("unexpected message type: %d", messageType)
	}

	r.buf = msg
	n = copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}
