package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/libp2p/go-libp2p"

	"github.com/ezreal1997/libp2p-playground/pubsubpeer"
)

var (
	topicName    = flag.String("topic", "default-topic", "topic name")
	discoveryTag = flag.String("discovery", "default-pubsub-service", "discovery tag")
)

func init() {
	flag.Parse()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create a new libp2p Host that listens on a random TCP port
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))
	if err != nil {
		panic(err)
	}
	defer h.Close()
	fmt.Printf("listen addr: %s\n", h.Addrs())
	fmt.Printf("host id: %s\n", h.ID().Pretty())

	mdnsSvr, err := pubsubpeer.NewMdns(ctx, h, *topicName, *discoveryTag)
	if err != nil {
		panic(err)
	}
	mdnsSvr.Run(func(msg []byte) error {
		var record pubsubpeer.TopicRecord
		if err := json.Unmarshal(msg, &record); err != nil {
			return err
		}
		fmt.Printf("receive msg: %+v\n", record)
		return nil
	})

	go func() {
		input := bufio.NewScanner(os.Stdin)
		for input.Scan() {
			line := input.Text()
			if err := mdnsSvr.Publish(line); err != nil {
				panic(err)
			}
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	sig := <-ch
	fmt.Printf("received signal %v, quiting gracefully\n", sig)
	mdnsSvr.Stop()
}
