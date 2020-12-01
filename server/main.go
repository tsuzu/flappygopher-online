package main

import (
	"context"
	"log"
	"net/http"
	"reflect"
	"sort"

	"github.com/google/uuid"
	"github.com/grafov/bcast"

	"github.com/cs3238-tsuzu/flappygopher-online/internal/message"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type Hub struct {
	group *bcast.Group
}

func (h *Hub) standingWorker() {
	member := h.group.Join()
	defer member.Close()

	const maxLength = 5
	standing := make([]message.Result, 0, maxLength)

	for {
		select {
		case m, ok := <-member.Read:
			if !ok {
				return
			}

			msg := m.(*message.Message)
			log.Println(msg)

			if msg.Kind == message.KindUpdate {
				if msg.User.Running || msg.User.Score == 0 {
					continue
				}

				tmp := make([]message.Result, len(standing), len(standing)+1)
				copy(tmp, standing)

				tmp = append(tmp, message.Result{
					Name:  msg.User.Name,
					Score: msg.User.Score,
				})

				sort.SliceStable(tmp, func(i, j int) bool {
					return tmp[i].Score > tmp[j].Score
				})

				if len(tmp) > maxLength {
					tmp = tmp[:maxLength]
				}

				if !reflect.DeepEqual(standing, tmp) {
					if len(standing) < len(tmp) {
						standing = append(standing, make([]message.Result, len(tmp)-len(standing))...)
					}
					copy(standing, tmp)

					member.Send(&message.Message{
						Kind:     message.KindStanding,
						Standing: tmp,
					})
				}
			} else if msg.Kind == message.KindJoin {
				tmp := make([]message.Result, len(standing), len(standing)+1)
				copy(tmp, standing)

				member.Send(&message.Message{
					Kind:     message.KindStanding,
					Standing: tmp,
				})
			}
		}
	}

}

func (h *Hub) HandleGameConnection(ctx context.Context, conn *websocket.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	member := h.group.Join()
	defer member.Close()
	id := uuid.New().String()
	log.Println(id, "joined")
	defer log.Println(id, "left")

	member.Send(&message.Message{
		Kind: message.KindJoin,
	})

	go func() {
		defer cancel()
		for {
			select {
			case msg, ok := <-member.Read:
				if !ok {
					return
				}

				m := msg.(*message.Message)

				if m.Kind == message.KindJoin {
					continue
				}

				if err := wsjson.Write(ctx, conn, msg); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			break
		default:
		}

		var msg message.Message
		err := wsjson.Read(ctx, conn, &msg)
		if err != nil {
			break
		}

		if !msg.Validate() || msg.Kind != message.KindUpdate {
			continue
		}
		msg.User.ID = id

		member.Send(&msg)
	}

	member.Send(&message.Message{
		Kind: message.KindLeave,
		User: message.User{
			ID: id,
		},
	})
}

func (h *Hub) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close(websocket.StatusInternalError, "the sky is falling")

	h.HandleGameConnection(r.Context(), c)

	c.Close(websocket.StatusNormalClosure, "")
}

func main() {
	group := bcast.NewGroup()
	go group.Broadcast(0)

	hub := &Hub{
		group: group,
	}

	go hub.standingWorker()

	mux := http.NewServeMux()
	// mux.Handle("/", http.FileServer(http.Dir("./static")))
	mux.HandleFunc("/ws", hub.WebSocketHandler)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		mux.ServeHTTP(w, r)
	})

	http.ListenAndServe(":7777", handler)
}
