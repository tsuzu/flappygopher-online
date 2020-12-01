package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"

	"github.com/cs3238-tsuzu/flappygopher-online/internal/message"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type Client struct {
	conn *websocket.Conn

	members     map[string]*Gopher
	membersLock sync.Mutex

	standing     []message.Result
	standingLock sync.Mutex

	gopherInitializer func() *Gopher
}

func NewClient(host string, gopherInitializer func() *Gopher) (*Client, error) {
	conn, _, err := websocket.Dial(context.Background(), host, nil)

	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	c := &Client{
		conn:              conn,
		members:           make(map[string]*Gopher),
		gopherInitializer: gopherInitializer,
	}

	go c.recvHandler()

	return c, nil
}

func (c *Client) sendMessage(ctx context.Context, msg *message.Message) error {
	return wsjson.Write(ctx, c.conn, msg)
}

func (c *Client) recvHandler() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var msg message.Message
	for {
		if err := wsjson.Read(ctx, c.conn, &msg); err != nil {
			return
		}

		log.Println("recv", msg)
		switch msg.Kind {
		case message.KindLeave:
			c.membersLock.Lock()

			user, ok := c.members[msg.User.ID]

			if ok {
				delete(c.members, msg.User.ID)
				go user.Close()
			}
			c.membersLock.Unlock()

		case message.KindUpdate:
			c.membersLock.Lock()

			user, ok := c.members[msg.User.ID]
			if !ok {
				user = c.gopherInitializer()

				c.members[msg.User.ID] = user
			}
			c.membersLock.Unlock()

			user.UpdateByMessage(&msg.User)

		case message.KindStanding:
			c.standingLock.Lock()

			if len(c.standing) != len(msg.Standing) {
				c.standing = make([]message.Result, len(msg.Standing))
			}
			copy(c.standing, msg.Standing)

			c.standingLock.Unlock()
		}
		log.Println("after", msg)
	}
}

func (c *Client) List() []*Gopher {
	c.membersLock.Lock()
	members := make([]*Gopher, len(c.members))
	idx := 0
	for key := range c.members {
		members[idx] = c.members[key]
		idx++
	}
	c.membersLock.Unlock()

	sort.Slice(members, func(i, j int) bool {
		return members[i].id < members[j].id
	})

	return members
}

func (c *Client) Standing() []message.Result {
	c.standingLock.Lock()
	defer c.standingLock.Unlock()

	res := make([]message.Result, len(c.standing))
	copy(res, c.standing)

	return res
}
