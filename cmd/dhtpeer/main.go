package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/google/martian/log"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/multiformats/go-multiaddr"

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
	ctx := context.Background()

	// create a new libp2p Host that listens on a random TCP port
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))
	if err != nil {
		panic(err)
	}
	defer h.Close()

	// view host details and addresses
	fmt.Printf("host ID %s\n", h.ID().Pretty())
	fmt.Printf("following are the assigned addresses\n")
	for _, addr := range h.Addrs() {
		fmt.Printf("%s\n", addr.String())
	}

	// create a new PubSub service using the GossipSub router
	gossipSub, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		panic(err)
	}

	// setup DHT with discovery peers:
	// 1. if empty peers, so this will be a discovery peer for others
	//    this peer should run on cloud(with public ip address).
	// 2. else, setup DHT with discovery server
	//	  this peer could run behind the nat(with private ip address)
	//multiAddr, err := multiaddr.NewMultiaddr("/ip4/30.138.358.234/tcp/9898/p2p/QmSrSELBGEsDjGQP2PkCwQWNayNBecvWmbrgz53Jv68xg5")
	//if err != nil {
	//	 panic(err)
	//}
	//discoveryPeers := []multiaddr.Multiaddr{multiAddr}
	var discoveryPeers []multiaddr.Multiaddr
	dht, err := pubsubpeer.NewDHT(ctx, h, discoveryPeers)
	if err != nil {
		panic(err)
	}

	// setup peer discovery
	go pubsubpeer.Discover(ctx, h, dht, "default-rendezvous")

	// setup local mDNS discovery
	mdnsService, err := pubsubpeer.SetupDiscovery(ctx, h, *discoveryTag)
	if err != nil {
		panic(err)
	}
	defer mdnsService.Close()

	// join the pubsub topic
	topic, err := gossipSub.Join(*topicName)
	if err != nil {
		panic(err)
	}

	// create publisher
	go publish(ctx, topic)

	// subscribe to topic
	subscriber, err := topic.Subscribe()
	if err != nil {
		panic(err)
	}
	go subscribe(subscriber, ctx, h.ID())

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	sig := <-ch
	fmt.Printf("received signal %v, quiting gracefully\n", sig)
}

// start publisher to topic
func publish(ctx context.Context, topic *pubsub.Topic) {
	for {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			msg := scanner.Text()
			if len(msg) != 0 {
				// publish message to topic
				bytes := []byte(msg)
				if err := topic.Publish(ctx, bytes); err != nil {
					log.Errorf("publish to topic %s failed: %v\n", topic.String(), err)
				}
			}
		}
	}
}

// start subscriber to topic
func subscribe(subscriber *pubsub.Subscription, ctx context.Context, hostID peer.ID) {
	for {
		msg, err := subscriber.Next(ctx)
		if err != nil {
			log.Errorf("fetch msg from topic failed: %v\n", err)
		}

		// only consider messages delivered by other peers
		if msg.ReceivedFrom == hostID {
			continue
		}

		fmt.Printf("got message: %s, from: %s\n", string(msg.Data), msg.ReceivedFrom.Pretty())
	}
}
