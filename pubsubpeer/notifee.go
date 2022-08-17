package pubsubpeer

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
)

// DiscoveryNotifee gets notified when we find a new peer via mDNS discovery
type DiscoveryNotifee struct {
	ctx context.Context
	h   host.Host
}

func SetupDiscovery(ctx context.Context,
	h host.Host, discoveryTag string) (mdns.Service, error) {
	s := mdns.NewMdnsService(h, discoveryTag, &DiscoveryNotifee{ctx: ctx, h: h})
	if err := s.Start(); err != nil {
		return nil, err
	}
	return s, nil
}

// HandlePeerFound connects to peers discovered via mDNS. Once they're connected,
// the PubSub system will automatically start interacting with them if they also
// support PubSub.
func (dn *DiscoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Printf("discovered new peer %s\n", pi.ID.Pretty())
	err := dn.h.Connect(context.Background(), pi)
	if err != nil {
		fmt.Printf("error connecting to peer %s: %v\n", pi.ID.Pretty(), err)
	}
}
