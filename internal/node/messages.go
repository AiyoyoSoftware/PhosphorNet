package node

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"phosphornet/internal/protocol"
)

func (s *Server) readClientMessage(ctx context.Context, conn *websocket.Conn) (any, error) {
	var raw json.RawMessage
	if err := wsjson.Read(ctx, conn, &raw); err != nil {
		return nil, err
	}
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode client message envelope: %w", err)
	}
	switch envelope.Type {
	case protocol.TypeOpenDoor:
		var message protocol.OpenDoorMessage
		if err := json.Unmarshal(raw, &message); err != nil {
			return nil, fmt.Errorf("decode open_door message: %w", err)
		}
		return message, nil
	case protocol.TypeEvent:
		var message protocol.EventMessage
		if err := json.Unmarshal(raw, &message); err != nil {
			return nil, fmt.Errorf("decode event message: %w", err)
		}
		return message, nil
	default:
		return nil, fmt.Errorf("unknown client message type %q", envelope.Type)
	}
}

func (s *Server) routeClientMessage(ctx context.Context, session *sessionState, raw any) error {
	switch message := raw.(type) {
	case protocol.OpenDoorMessage:
		return s.openDoor(ctx, session, message.DoorID)
	case protocol.EventMessage:
		if err := session.validateEventEnvelope(message, time.Now()); err != nil {
			return err
		}
		return s.handleDoorEvent(ctx, session, message.Event)
	default:
		return fmt.Errorf("unhandled client message type")
	}
}
