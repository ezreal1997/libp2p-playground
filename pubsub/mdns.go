package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsubp2p "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

type MdnPubSub struct {
	ctx       context.Context
	cancel    context.CancelFunc
	wg        *sync.WaitGroup
	h         host.Host
	gossipSvr *pubsubp2p.PubSub
	topic     *pubsubp2p.Topic
	sub       *pubsubp2p.Subscription
}

func NewMdns(ctx context.Context, h host.Host, topicName, discoveryTag string) (*MdnPubSub, error) {
	mCtx, mCancel := context.WithCancel(ctx)
	// create a new PubSub service using the GossipSub router
	gs, err := pubsubp2p.NewGossipSub(mCtx, h)
	if err != nil {
		mCancel()
		return nil, err
	}
	// setup local mDNS discovery
	s := mdns.NewMdnsService(h, discoveryTag, &mdnsNotifee{ctx: mCtx, h: h})
	if err := s.Start(); err != nil {
		mCancel()
		return nil, err
	}
	// join the pubsub topic
	topic, err := gs.Join(topicName)
	if err != nil {
		mCancel()
		return nil, err
	}
	// subscribe to topic
	sub, err := topic.Subscribe()
	if err != nil {
		mCancel()
		return nil, err
	}

	return &MdnPubSub{
		ctx:       mCtx,
		cancel:    mCancel,
		wg:        &sync.WaitGroup{},
		h:         h,
		gossipSvr: gs,
		topic:     topic,
		sub:       sub,
	}, nil
}

func (m *MdnPubSub) Run(msgHandler func(msg []byte) error) {
	m.wg.Add(1)
	go m.readLoop(msgHandler)
}

func (m *MdnPubSub) Stop() {
	m.cancel()
	m.wg.Wait()
}

func (m *MdnPubSub) readLoop(msgHandler func(msg []byte) error) {
	defer m.wg.Done()
	t := time.NewTicker(50 * time.Millisecond)
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-t.C:
			ctxRead, cancelRead := context.WithTimeout(m.ctx, time.Second)
			msg, err := m.sub.Next(ctxRead)
			if errors.Is(err, context.DeadlineExceeded) {
				cancelRead()
				continue
			} else if err != nil {
				cancelRead()
				fmt.Printf("read from topic failed: %v\n", err)
				continue
			}
			cancelRead()
			// only forward messages delivered by others
			if msg.ReceivedFrom == m.h.ID() {
				continue
			}
			if err := msgHandler(msg.Data); err != nil {
				fmt.Printf("deal with topic data failed: %v\n", err)
				continue
			}
		}
	}
}

func (m *MdnPubSub) Publish(msg string) error {
	record := TopicRecord{
		Message:  msg,
		SenderID: m.h.ID().Pretty(),
	}
	msgBytes, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return m.topic.Publish(m.ctx, msgBytes)
}

func (m *MdnPubSub) ListPeers() []peer.ID {
	return m.gossipSvr.ListPeers(m.topic.String())
}

// mdnsNotifee gets notified when we find a new peer via mDNS discovery
type mdnsNotifee struct {
	ctx context.Context
	h   host.Host
}

// HandlePeerFound connects to peers discovered via mDNS. Once they're connected,
// the PubSub system will automatically start interacting with them if they also
// support PubSub.
func (n *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Printf("discovered new peer %s\n", pi.ID.Pretty())
	err := n.h.Connect(context.Background(), pi)
	if err != nil {
		fmt.Printf("error connecting to peer %s: %v\n", pi.ID.Pretty(), err)
	}
}
