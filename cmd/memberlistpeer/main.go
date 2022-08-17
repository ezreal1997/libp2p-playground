package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/hashicorp/memberlist"
	uuid "github.com/satori/go.uuid"
)

var (
	members        = flag.String("members", "", "comma seperated list of members")
	broadcastQueue *memberlist.TransmitLimitedQueue
)

func init() {
	flag.Parse()
}

type broadcastData struct {
	msg    []byte
	notify chan<- struct{}
}

func (b *broadcastData) Invalidates(data memberlist.Broadcast) bool {
	return false
}

func (b *broadcastData) Message() []byte {
	return b.msg
}

func (b *broadcastData) Finished() {
	if b.notify != nil {
		close(b.notify)
	}
}

type delegate struct{}

func (d *delegate) NodeMeta(limit int) []byte {
	return []byte{}
}

func (d *delegate) NotifyMsg(buf []byte) {
	if len(buf) == 0 {
		return
	}
	fmt.Printf("receive msg: %s\n", string(buf))
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	return broadcastQueue.GetBroadcasts(overhead, limit)
}

func (d *delegate) LocalState(join bool) []byte {
	return nil
}

func (d *delegate) MergeRemoteState(buf []byte, join bool) {
	if len(buf) == 0 {
		return
	}
	if !join {
		return
	}
	// Do nothing
}

type eventDelegate struct{}

func (ed *eventDelegate) NotifyJoin(node *memberlist.Node) {
	fmt.Printf("A node has joined: %s\n", node.String())
}

func (ed *eventDelegate) NotifyLeave(node *memberlist.Node) {
	fmt.Printf("A node has left: %s\n", node.String())
}

func (ed *eventDelegate) NotifyUpdate(node *memberlist.Node) {
	fmt.Printf("A node was updated: %s\n", node.String())
}

func main() {
	conf := memberlist.DefaultLocalConfig()
	conf.Events = &eventDelegate{}
	conf.Delegate = &delegate{}
	conf.BindPort = 0 // 0 implies get a random port
	conf.Name = uuid.Must(uuid.NewV4(), nil).String()

	m, err := memberlist.Create(conf)
	if err != nil {
		panic(err)
	}
	defer m.Shutdown()
	if len(*members) > 0 {
		memberList := strings.Split(*members, ",")
		if _, err := m.Join(memberList); err != nil {
			panic(err)
		}
	}
	fmt.Printf("find %d nodes\n", m.NumMembers())

	broadcastQueue = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			return m.NumMembers()
		},
		RetransmitMult: 3,
	}

	node := m.LocalNode()
	fmt.Printf("listen addr %s:%d\n", node.Addr, node.Port)

	// write loop
	go func() {
		input := bufio.NewScanner(os.Stdin)
		for input.Scan() {
			line := input.Text()
			broadcastQueue.QueueBroadcast(&broadcastData{
				msg:    []byte(line),
				notify: nil,
			})
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	sig := <-ch
	fmt.Printf("received signal %v, quiting gracefully\n", sig)
}
