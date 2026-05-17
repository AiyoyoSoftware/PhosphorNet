package node

import (
	"context"
	"fmt"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"phosphornet/internal/identity"
	"phosphornet/internal/protocol"
)

func (s *Server) authenticate(ctx context.Context, conn *websocket.Conn) (*sessionState, bool) {
	var hello protocol.HelloMessage
	if err := wsjson.Read(ctx, conn, &hello); err != nil {
		s.events.add("auth_failure", "", "", "expected hello")
		_ = conn.Close(websocket.StatusInvalidFramePayloadData, "expected hello")
		return nil, false
	}
	if err := protocol.ValidateClientHello(hello); err != nil {
		s.events.add("auth_failure", "", hello.ClientPublicKey, err.Error())
		_ = wsjson.Write(ctx, conn, protocol.ErrorMessageFor(err))
		_ = conn.Close(websocket.StatusPolicyViolation, "expected hello")
		return nil, false
	}

	challenge, err := randomNonce()
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "nonce generation failed")
		return nil, false
	}
	nodePassport := &identity.Passport{
		DisplayName: s.cfg.Name,
		PublicKey:   s.cfg.NodeID,
		PrivateKey:  s.cfg.PrivateKey,
	}
	challengePayload := identity.NodeChallengePayload{
		Purpose:         "phosphornet.node_challenge.v1",
		NodeID:          s.cfg.NodeID,
		NodeName:        s.cfg.Name,
		ClientPublicKey: hello.ClientPublicKey,
		Nonce:           challenge,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}
	challengeSignature, err := identity.SignNodeChallenge(nodePassport, challengePayload)
	if err != nil {
		s.events.add("auth_failure", "", hello.ClientPublicKey, "node challenge signing failed")
		_ = conn.Close(websocket.StatusInternalError, "node challenge signing failed")
		return nil, false
	}
	if err := wsjson.Write(ctx, conn, protocol.ChallengeMessage{
		Type:      protocol.TypeChallenge,
		Payload:   challengePayload,
		Signature: challengeSignature,
	}); err != nil {
		return nil, false
	}

	var auth protocol.AuthMessage
	if err := wsjson.Read(ctx, conn, &auth); err != nil {
		return nil, false
	}
	if auth.Type != protocol.TypeAuth {
		s.events.add("auth_failure", "", hello.ClientPublicKey, "expected auth")
		_ = wsjson.Write(ctx, conn, protocol.AuthDeniedMessage{
			Type:   protocol.TypeAuthDenied,
			Reason: "expected auth",
		})
		return nil, false
	}
	if auth.Payload.Purpose != "phosphornet.login.v1" {
		s.events.add("auth_failure", "", hello.ClientPublicKey, "invalid purpose")
		_ = wsjson.Write(ctx, conn, protocol.AuthDeniedMessage{
			Type:   protocol.TypeAuthDenied,
			Reason: "invalid purpose",
		})
		return nil, false
	}
	if auth.Payload.NodeID != challengePayload.NodeID || auth.Payload.Nonce != challengePayload.Nonce || auth.Payload.ClientPublicKey != hello.ClientPublicKey {
		s.events.add("auth_failure", "", hello.ClientPublicKey, "challenge mismatch")
		_ = wsjson.Write(ctx, conn, protocol.AuthDeniedMessage{
			Type:   protocol.TypeAuthDenied,
			Reason: "challenge mismatch",
		})
		return nil, false
	}
	if err := identity.VerifyLogin(auth.Payload, auth.Signature); err != nil {
		s.events.add("auth_failure", "", hello.ClientPublicKey, err.Error())
		_ = wsjson.Write(ctx, conn, protocol.AuthDeniedMessage{
			Type:   protocol.TypeAuthDenied,
			Reason: err.Error(),
		})
		return nil, false
	}
	if _, banned, _, _, _ := s.moderationForPublicKey(ctx, auth.Payload.ClientPublicKey); banned {
		s.events.add("access_denied", "", auth.Payload.ClientPublicKey, "station access revoked")
		_ = wsjson.Write(ctx, conn, protocol.AuthDeniedMessage{
			Type:   protocol.TypeAuthDenied,
			Reason: "station access revoked",
		})
		return nil, false
	}
	if !s.stationAllows(ctx, auth.Payload.ClientPublicKey) {
		s.events.add("access_denied", "", auth.Payload.ClientPublicKey, "station is invite-only")
		_ = wsjson.Write(ctx, conn, protocol.AuthDeniedMessage{
			Type:   protocol.TypeAuthDenied,
			Reason: "station is invite-only",
		})
		return nil, false
	}

	if err := s.store.RecordUser(ctx, auth.Payload.ClientPublicKey); err != nil {
		_ = wsjson.Write(ctx, conn, protocol.ErrorMessageFor(protocol.NewCodedError(protocol.ErrorStorage, err.Error(), err)))
		return nil, false
	}

	sessionID, err := randomSessionID()
	if err != nil {
		_ = wsjson.Write(ctx, conn, protocol.ErrorMessageFor(protocol.NewCodedError(protocol.ErrorProtocol, fmt.Sprintf("create session: %v", err), err)))
		return nil, false
	}
	role := s.roleForPublicKey(ctx, auth.Payload.ClientPublicKey)
	if err := wsjson.Write(ctx, conn, protocol.AuthOKMessage{
		Type:        protocol.TypeAuthOK,
		NodeID:      s.cfg.NodeID,
		NodeName:    s.cfg.Name,
		Role:        role,
		Fingerprint: identity.Fingerprint(auth.Payload.ClientPublicKey),
	}); err != nil {
		return nil, false
	}
	s.events.add("auth_success", "", auth.Payload.ClientPublicKey, "authenticated as "+role)

	return &sessionState{
		id:        sessionID,
		publicKey: auth.Payload.ClientPublicKey,
		role:      role,
	}, true
}
