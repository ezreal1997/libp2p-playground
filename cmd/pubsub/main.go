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
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

type TopicRecord struct {
	Message  string
	SenderID string
}

// DiscoveryServiceTag is used in our mDNS advertisements to discover other chat peers.
const DiscoveryServiceTag = "default-pubsub-service"

func main() {
	// parse topic flag
	topicFlag := flag.String("topic", "default-topic", "topic name")
	flag.Parse()

	ctx := context.Background()

	// create a new libp2p Host that listens on a random TCP port
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))
	if err != nil {
		panic(err)
	}
	defer h.Close()

	// create a new PubSub service using the GossipSub router
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		panic(err)
	}
	// setup local mDNS discovery
	if err := setupDiscovery(h); err != nil {
		panic(err)
	}
	// join the pubsub topic
	topic, err := ps.Join(*topicFlag)
	if err != nil {
		panic(err)
	}
	// subscribe to topic
	sub, err := topic.Subscribe()
	if err != nil {
		panic(err)
	}

	publish := func(msg string) error {
		m := TopicRecord{
			Message:  msg,
			SenderID: h.ID().Pretty(),
		}
		msgBytes, err := json.Marshal(m)
		if err != nil {
			return err
		}
		return topic.Publish(ctx, msgBytes)
	}

	listPeers := func() []peer.ID {
		return ps.ListPeers(*topicFlag)
	}

	go func() {
		for {
			msg, err := sub.Next(ctx)
			if err != nil {
				panic(err)
			}
			// only forward messages delivered by others
			if msg.ReceivedFrom == h.ID() {
				continue
			}
			var record TopicRecord
			err = json.Unmarshal(msg.Data, &record)
			if err != nil {
				panic(err)
			}
			fmt.Printf("receive msg: %+v\n", record)

			peers := listPeers()
			fmt.Printf("now have %d peers\n", len(peers))
			for i, p := range peers {
				fmt.Printf("peer %d: %s\n", i, p.Pretty())
			}
		}
	}()

	go func() {
		input := bufio.NewScanner(os.Stdin)
		for input.Scan() {
			line := input.Text()
			if err := publish(line); err != nil {
				panic(err)
			}
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	sig := <-ch
	fmt.Printf("received signal %v, quiting gracefully\n", sig)
}

// discoveryNotifee gets notified when we find a new peer via mDNS discovery
type discoveryNotifee struct {
	h host.Host
}

// HandlePeerFound connects to peers discovered via mDNS. Once they're connected,
// the PubSub system will automatically start interacting with them if they also
// support PubSub.
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Printf("discovered new peer %s\n", pi.ID.Pretty())
	err := n.h.Connect(context.Background(), pi)
	if err != nil {
		fmt.Printf("error connecting to peer %s: %s\n", pi.ID.Pretty(), err)
	}
}

// setupDiscovery creates an mDNS discovery service and attaches it to the libp2p Host.
// This lets us automatically discover peers on the same LAN and connect to them.
func setupDiscovery(h host.Host) error {
	// setup mDNS discovery to find local peers
	s := mdns.NewMdnsService(h, DiscoveryServiceTag, &discoveryNotifee{h: h})
	return s.Start()
}
