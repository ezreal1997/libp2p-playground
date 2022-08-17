package pubsubpeer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/pkg/errors"
)

type MdnsPubSub struct {
	ctx         context.Context
	cancel      context.CancelFunc
	wg          *sync.WaitGroup
	h           host.Host
	mdnsService mdns.Service
	gossipSvr   *pubsub.PubSub
	topic       *pubsub.Topic
	sub         *pubsub.Subscription
}

func NewMdns(ctx context.Context, h host.Host, topicName, discoveryTag string) (*MdnsPubSub, error) {
	mCtx, mCancel := context.WithCancel(ctx)
	// create a new PubSub service using the GossipSub router
	gs, err := pubsub.NewGossipSub(mCtx, h)
	if err != nil {
		mCancel()
		return nil, err
	}
	// setup local mDNS discovery
	mdnsService, err := SetupDiscovery(mCtx, h, discoveryTag)
	if err != nil {
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

	return &MdnsPubSub{
		ctx:         mCtx,
		cancel:      mCancel,
		wg:          &sync.WaitGroup{},
		h:           h,
		mdnsService: mdnsService,
		gossipSvr:   gs,
		topic:       topic,
		sub:         sub,
	}, nil
}

func (m *MdnsPubSub) Run(msgHandler func(msg []byte) error) {
	m.wg.Add(1)
	go m.readLoop(msgHandler)
}

func (m *MdnsPubSub) Stop() {
	m.cancel()
	
	m.mdnsService.Close()
	m.topic.Close()
	m.sub.Cancel()

	m.wg.Wait()
}

func (m *MdnsPubSub) readLoop(msgHandler func(msg []byte) error) {
	defer m.wg.Done()
	t := time.NewTicker(50 * time.Millisecond)
	defer t.Stop()
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

func (m *MdnsPubSub) Publish(msg string) error {
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

func (m *MdnsPubSub) ListPeers() []peer.ID {
	return m.gossipSvr.ListPeers(m.topic.String())
}
