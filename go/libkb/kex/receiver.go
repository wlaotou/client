package kex

import (
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/keybase/client/go/libkb"
)

// GlobalTimeout is unused currently, but is intended to be the
// overall timeout for a receive operation.
var GlobalTimeout = 5 * time.Minute

// Receiver gets kex messages from the server and routes them to a
// kex Handler.
type Receiver struct {
	handler   Handler
	seqno     int
	pollDur   time.Duration
	direction Direction
}

// NewReceiver creates a Receiver that will route messages to the
// provided handler.  It will receive messages for the specified
// direction.
func NewReceiver(handler Handler, dir Direction) *Receiver {
	return &Receiver{handler: handler, pollDur: 20 * time.Second, direction: dir}
}

// Receive gets the next set of messages from the server and
// routes them to the handler.
func (r *Receiver) Receive(m *Meta) error {
	msgs, err := r.get(m)
	if err != nil {
		return err
	}
	for _, msg := range msgs {
		if msg.Seqno > r.seqno {
			r.seqno = msg.Seqno
		}

		// set context's sender and receiver
		m.Sender = msg.Sender
		m.Receiver = msg.Receiver

		switch msg.Name {
		case startkexMsg:
			return r.handler.StartKexSession(m, msg.Args.StrongID)
		case startrevkexMsg:
			return r.handler.StartReverseKexSession(m)
		case helloMsg:
			return r.handler.Hello(m, msg.Args.DeviceID, msg.Args.DevKeyID)
		case pleasesignMsg:
			return r.handler.PleaseSign(m, msg.Args.SigningKey, msg.Args.Sig, msg.Args.DevType, msg.Args.DevDesc)
		case doneMsg:
			return r.handler.Done(m, msg.Args.MerkleTriple)
		default:
			return fmt.Errorf("unhandled message name: %q", msg.Name)
		}
	}
	return nil
}

// get performs a Get request to long poll for a set of messages.
func (r *Receiver) get(m *Meta) (MsgList, error) {
	G.Log.Debug("get: w = %x, dir = %d, seqno = %d", m.WeakID, r.direction, r.seqno)
	res, err := G.API.Get(libkb.ApiArg{
		Endpoint:    "kex/receive",
		NeedSession: true,
		Args: libkb.HttpArgs{
			"w":    libkb.S{Val: hex.EncodeToString(m.WeakID[:])},
			"dir":  libkb.I{Val: int(r.direction)},
			"low":  libkb.I{Val: r.seqno + 1},
			"poll": libkb.I{Val: int(r.pollDur / time.Second)},
		},
	})
	if err != nil {
		return nil, err
	}

	msgs := res.Body.AtKey("msgs")
	n, err := msgs.Len()
	if err != nil {
		return nil, err
	}

	var messages MsgList
	for i := 0; i < n; i++ {
		m, err := MsgImport(msgs.AtIndex(i))
		if err != nil {
			if err != ErrMACMismatch {
				return nil, err
			}
			G.Log.Warning("Received message with bad HMAC.  Ignoring it.")
		} else {
			messages = append(messages, m)
		}
	}

	sort.Sort(messages)

	return messages, nil
}
