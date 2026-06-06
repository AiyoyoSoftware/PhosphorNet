package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/spf13/cobra"

	"phosphornet/internal/app"
	"phosphornet/internal/identity"
	"phosphornet/internal/knownnodes"
	"phosphornet/internal/protocol"
)

func Execute() error {
	return NewRootCommand().Execute()
}

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "phosphor",
		Short: "Connect to a PhosphorNet station",
	}
	root.AddCommand(newPassportCommand(), newConnectCommand())
	return root
}

func newPassportCommand() *cobra.Command {
	var passportPath string
	command := &cobra.Command{
		Use:   "passport",
		Short: "Manage local passports",
	}

	create := &cobra.Command{
		Use:   "create",
		Short: "Create a passport",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.EnsureParentDir(passportPath); err != nil {
				return err
			}
			passport, err := identity.Generate("traveler")
			if err != nil {
				return err
			}
			if err := identity.Save(passportPath, passport); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote passport to %s\n", passportPath)
			fmt.Fprintf(cmd.OutOrStdout(), "fingerprint: %s\n", passport.Fingerprint())
			return nil
		},
	}

	show := &cobra.Command{
		Use:   "show",
		Short: "Show passport fingerprint",
		RunE: func(cmd *cobra.Command, args []string) error {
			passport, err := identity.Load(passportPath)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "public key: %s\n", passport.PublicKey)
			fmt.Fprintf(cmd.OutOrStdout(), "fingerprint: %s\n", passport.Fingerprint())
			return nil
		},
	}

	create.Flags().StringVar(&passportPath, "passport", app.DefaultPassportPath(), "passport path")
	show.Flags().StringVar(&passportPath, "passport", app.DefaultPassportPath(), "passport path")
	command.AddCommand(create, show)
	return command
}

func newConnectCommand() *cobra.Command {
	var (
		passportPath   string
		knownNodesPath string
		rawAddress     string
		quick          bool
		replaceKnown   bool
	)

	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect to a node and render its JSON UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			if rawAddress == "" {
				rawAddress = "wss://127.0.0.1:7707/ws"
			}
			if quick {
				passportPath = app.QuickTestPassportPath()
				knownNodesPath = app.QuickTestKnownNodesPath()
				fmt.Fprintf(cmd.OutOrStdout(), "quick mode using %s and %s\n", passportPath, knownNodesPath)
			}
			address, err := normalizeWebSocketURL(rawAddress)
			if err != nil {
				return err
			}

			passport, err := ensurePassport(passportPath)
			if err != nil {
				return err
			}
			store, err := knownnodes.Load(knownNodesPath)
			if err != nil {
				return err
			}

			handshakeCtx, cancelHandshake := context.WithTimeout(context.Background(), 10*time.Second)

			conn, resp, err := websocket.Dial(handshakeCtx, address, &websocket.DialOptions{
				HTTPClient: websocketHTTPClient(),
			})
			if err != nil {
				cancelHandshake()
				return err
			}
			defer conn.CloseNow()
			conn.SetReadLimit(protocol.MaxWebSocketMessageBytes)

			if err := wsjson.Write(handshakeCtx, conn, protocol.DefaultHello(passport.PublicKey)); err != nil {
				cancelHandshake()
				return err
			}

			challenge, err := readChallengeOrError(handshakeCtx, conn)
			if err != nil {
				cancelHandshake()
				return err
			}
			cancelHandshake()
			if err := verifyServerChallenge(challenge, passport.PublicKey); err != nil {
				return fmt.Errorf("verify server challenge: %w", err)
			}

			trustSummary, err := buildTrustSummary(address, resp, challenge.Payload.NodeID, challenge.Payload.NodeName, store, replaceKnown)
			if err != nil {
				return err
			}
			if trustSummary.RequiresPrompt {
				if err := runFirstConnectTrustTUI(cmd.InOrStdin(), cmd.OutOrStdout(), trustSummary); err != nil {
					return err
				}
			} else if trustSummary.ShowNotice {
				printTrustSummary(cmd.OutOrStdout(), trustSummary, false)
			}

			if err := pinNode(address, challenge.Payload.NodeID, store, knownNodesPath, replaceKnown); err != nil {
				return err
			}

			authCtx, cancelAuth := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancelAuth()

			payload := identity.LoginPayload{
				Purpose:         "phosphornet.login.v1",
				NodeID:          challenge.Payload.NodeID,
				ClientPublicKey: passport.PublicKey,
				Nonce:           challenge.Payload.Nonce,
				Timestamp:       time.Now().UTC().Format(time.RFC3339),
			}
			signature, err := identity.SignLogin(passport, payload)
			if err != nil {
				return err
			}

			if err := wsjson.Write(authCtx, conn, protocol.AuthMessage{
				Type:      protocol.TypeAuth,
				Payload:   payload,
				Signature: signature,
			}); err != nil {
				return err
			}

			authOK, err := readAuthOKOrDenied(authCtx, conn)
			if err != nil {
				return err
			}
			var doorList protocol.DoorListMessage
			if err := wsjson.Read(authCtx, conn, &doorList); err != nil {
				return err
			}

			var render protocol.RenderMessage
			if err := wsjson.Read(authCtx, conn, &render); err != nil {
				return err
			}

			model := newTUIModel(conn, authOK, doorList.Doors, render, trustSummary.Status)
			program := tea.NewProgram(model, tea.WithAltScreen())
			go readLoop(conn, program)

			if _, err := program.Run(); err != nil {
				return err
			}

			return conn.Close(websocket.StatusNormalClosure, "done")
		},
	}

	cmd.Flags().StringVar(&passportPath, "passport", app.DefaultPassportPath(), "passport path")
	cmd.Flags().StringVar(&knownNodesPath, "known-nodes", app.DefaultKnownNodesPath(), "known nodes path")
	cmd.Flags().StringVar(&rawAddress, "addr", "wss://127.0.0.1:7707/ws", "node websocket address")
	cmd.Flags().BoolVar(&quick, "quick", false, "use auto-managed temp passport and known-node files for local testing")
	cmd.Flags().BoolVar(&replaceKnown, "replace-known-node", false, "replace a changed known-node key for local testing")
	return cmd
}

func websocketHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: true,
			},
		},
	}
}

func readChallengeOrError(ctx context.Context, conn *websocket.Conn) (protocol.ChallengeMessage, error) {
	var raw json.RawMessage
	if err := wsjson.Read(ctx, conn, &raw); err != nil {
		return protocol.ChallengeMessage{}, err
	}
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return protocol.ChallengeMessage{}, fmt.Errorf("decode handshake envelope: %w", err)
	}
	switch envelope.Type {
	case protocol.TypeChallenge:
		var challenge protocol.ChallengeMessage
		if err := json.Unmarshal(raw, &challenge); err != nil {
			return protocol.ChallengeMessage{}, fmt.Errorf("decode challenge message: %w", err)
		}
		return challenge, nil
	case protocol.TypeError:
		var message protocol.ErrorMessage
		if err := json.Unmarshal(raw, &message); err != nil {
			return protocol.ChallengeMessage{}, fmt.Errorf("decode error message: %w", err)
		}
		if message.Code != "" {
			return protocol.ChallengeMessage{}, fmt.Errorf("%s: %s", message.Code, message.Message)
		}
		return protocol.ChallengeMessage{}, fmt.Errorf("%s", message.Message)
	default:
		return protocol.ChallengeMessage{}, fmt.Errorf("expected challenge, got %q", envelope.Type)
	}
}

func readAuthOKOrDenied(ctx context.Context, conn *websocket.Conn) (protocol.AuthOKMessage, error) {
	var raw json.RawMessage
	if err := wsjson.Read(ctx, conn, &raw); err != nil {
		return protocol.AuthOKMessage{}, err
	}
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return protocol.AuthOKMessage{}, fmt.Errorf("decode auth envelope: %w", err)
	}
	switch envelope.Type {
	case protocol.TypeAuthOK:
		var authOK protocol.AuthOKMessage
		if err := json.Unmarshal(raw, &authOK); err != nil {
			return protocol.AuthOKMessage{}, fmt.Errorf("decode auth_ok message: %w", err)
		}
		return authOK, nil
	case protocol.TypeAuthDenied:
		var denied protocol.AuthDeniedMessage
		if err := json.Unmarshal(raw, &denied); err != nil {
			return protocol.AuthOKMessage{}, fmt.Errorf("decode auth_denied message: %w", err)
		}
		return protocol.AuthOKMessage{}, fmt.Errorf("authentication failed: %s", denied.Reason)
	case protocol.TypeError:
		var message protocol.ErrorMessage
		if err := json.Unmarshal(raw, &message); err != nil {
			return protocol.AuthOKMessage{}, fmt.Errorf("decode error message: %w", err)
		}
		if message.Code != "" {
			return protocol.AuthOKMessage{}, fmt.Errorf("%s: %s", message.Code, message.Message)
		}
		return protocol.AuthOKMessage{}, fmt.Errorf("%s", message.Message)
	default:
		return protocol.AuthOKMessage{}, fmt.Errorf("authentication failed: expected auth_ok, got %q", envelope.Type)
	}
}

type trustSummary struct {
	Address              string
	Transport            string
	Certificate          string
	HostnameVerification string
	StationName          string
	StationIdentity      string
	StationFingerprint   string
	StationPublicKey     string
	Status               string
	RequiresPrompt       bool
	ShowNotice           bool
}

func buildTrustSummary(address string, resp *http.Response, nodeID, nodeName string, store *knownnodes.KnownNodes, replace bool) (trustSummary, error) {
	transport, certificate, hostname := transportTrustDetails(address, resp)
	fingerprint := identity.Fingerprint(nodeID)
	summary := trustSummary{
		Address:              address,
		Transport:            transport,
		Certificate:          certificate,
		HostnameVerification: hostname,
		StationName:          nodeName,
		StationFingerprint:   fingerprint,
		StationPublicKey:     nodeID,
	}

	record, found := store.Find(address)
	switch {
	case !found:
		summary.StationIdentity = fmt.Sprintf("new Ed25519 station identity (%s)", fingerprint)
		summary.Status = fmt.Sprintf("%s Station identity newly pinned: %s.", compactTransportStatus(resp), fingerprint)
		summary.RequiresPrompt = true
	case record.PublicKey == nodeID:
		summary.StationIdentity = fmt.Sprintf("pinned Ed25519 station identity matched (%s)", fingerprint)
		summary.Status = fmt.Sprintf("%s Station identity pinned: %s.", compactTransportStatus(resp), fingerprint)
	case replace:
		oldFingerprint := identity.Fingerprint(record.PublicKey)
		summary.StationIdentity = fmt.Sprintf("changed Ed25519 station identity; replacing %s with %s", oldFingerprint, fingerprint)
		summary.Status = fmt.Sprintf("%s Station identity pin replaced: %s.", compactTransportStatus(resp), fingerprint)
		summary.ShowNotice = true
	default:
		return trustSummary{}, fmt.Errorf("node identity changed for %s\nPinned station identity: %s\nPresented station identity: %s\nThis could be a station reinstall or an impersonation. Use --replace-known-node only when you intentionally replaced the station identity.", address, identity.Fingerprint(record.PublicKey), fingerprint)
	}
	return summary, nil
}

func printTrustSummary(output io.Writer, summary trustSummary, ask bool) {
	fmt.Fprintln(output)
	if summary.RequiresPrompt {
		fmt.Fprintln(output, "First Connection Trust Check")
	} else {
		fmt.Fprintln(output, "Station Trust Check")
	}
	fmt.Fprintf(output, "Address: %s\n", summary.Address)
	fmt.Fprintf(output, "Transport: %s\n", summary.Transport)
	fmt.Fprintf(output, "Certificate: %s\n", summary.Certificate)
	fmt.Fprintf(output, "Hostname verification: %s\n", summary.HostnameVerification)
	fmt.Fprintf(output, "Station name: %s\n", summary.StationName)
	fmt.Fprintf(output, "Station identity: %s\n", summary.StationIdentity)
	fmt.Fprintf(output, "Station public key: %s\n", summary.StationPublicKey)
	fmt.Fprintln(output)
	fmt.Fprintln(output, "PhosphorNet uses the Ed25519 station key as the identity anchor.")
	fmt.Fprintln(output, "Transport security, certificate status, and pinned station identity are separate facts.")
	if ask {
		fmt.Fprint(output, "Trust and pin this station identity for this address? [y/N] ")
	}
}

func transportTrustDetails(address string, resp *http.Response) (transport, certificate, hostname string) {
	parsed, err := url.Parse(address)
	if err != nil {
		return "unknown transport", "certificate status unavailable", "hostname verification unavailable"
	}
	if parsed.Scheme != "wss" {
		return "not encrypted (plain ws://)", "none (no TLS certificate)", "not available without TLS"
	}
	if resp == nil || resp.TLS == nil {
		return "encrypted status unavailable (wss://)", "certificate status unavailable", "hostname verification unavailable"
	}

	state := resp.TLS
	transport = fmt.Sprintf("encrypted (%s)", tlsVersionName(state.Version))
	if len(state.PeerCertificates) == 0 {
		return transport, "not presented", "not available because no certificate was presented"
	}

	host := parsed.Hostname()
	leaf := state.PeerCertificates[0]
	if verifyWebPKI(leaf, state.PeerCertificates[1:], host) == nil {
		return transport, "domain-authenticated WebPKI certificate", fmt.Sprintf("verified for %s through WebPKI", host)
	}

	hostnameErr := leaf.VerifyHostname(host)
	switch {
	case isSelfSignedCertificate(leaf) && hostnameErr == nil:
		return transport, "self-signed station certificate", fmt.Sprintf("name matches %s, but issuer is not WebPKI trusted", host)
	case isSelfSignedCertificate(leaf):
		return transport, "self-signed station certificate", fmt.Sprintf("not WebPKI verified; certificate name does not match %s", host)
	case hostnameErr == nil:
		return transport, "certificate is not WebPKI trusted", fmt.Sprintf("name matches %s, but chain is not WebPKI trusted", host)
	default:
		return transport, "certificate is not WebPKI trusted", fmt.Sprintf("not WebPKI verified for %s", host)
	}
}

func verifyWebPKI(leaf *x509.Certificate, intermediates []*x509.Certificate, host string) error {
	opts := x509.VerifyOptions{
		DNSName:     host,
		CurrentTime: time.Now(),
	}
	if len(intermediates) > 0 {
		pool := x509.NewCertPool()
		for _, certificate := range intermediates {
			pool.AddCert(certificate)
		}
		opts.Intermediates = pool
	}
	_, err := leaf.Verify(opts)
	return err
}

func isSelfSignedCertificate(certificate *x509.Certificate) bool {
	return bytes.Equal(certificate.RawSubject, certificate.RawIssuer)
}

func tlsVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS13:
		return "TLS 1.3"
	case tls.VersionTLS12:
		return "TLS 1.2"
	default:
		return "TLS"
	}
}

func compactTransportStatus(resp *http.Response) string {
	if resp != nil && resp.TLS != nil {
		return "Transport encrypted."
	}
	return "Transport not encrypted."
}

func ensurePassport(path string) (*identity.Passport, error) {
	passport, err := identity.Load(path)
	if err == nil {
		return passport, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := app.EnsureParentDir(path); err != nil {
		return nil, err
	}
	passport, err = identity.Generate("traveler")
	if err != nil {
		return nil, err
	}
	if err := identity.Save(path, passport); err != nil {
		return nil, err
	}
	return passport, nil
}

func pinNode(address, publicKey string, store *knownnodes.KnownNodes, path string, replace bool) error {
	record, found := store.Find(address)
	if found && record.PublicKey != publicKey && !replace {
		return fmt.Errorf("node identity changed for %s", address)
	}
	if found && record.PublicKey == publicKey {
		return nil
	}
	if err := app.EnsureParentDir(path); err != nil {
		return err
	}
	store.Upsert(knownnodes.Record{
		Address:   address,
		PublicKey: publicKey,
		Name:      identity.Fingerprint(publicKey),
	})
	return knownnodes.Save(path, store)
}

func normalizeWebSocketURL(rawAddress string) (string, error) {
	parsed, err := url.Parse(rawAddress)
	if err != nil {
		return "", fmt.Errorf("parse address: %w", err)
	}
	if parsed.Scheme == "" {
		return "", fmt.Errorf("address must include ws:// or wss://")
	}
	return parsed.String(), nil
}

func verifyServerChallenge(challenge protocol.ChallengeMessage, clientPublicKey string) error {
	if challenge.Payload.Purpose != "phosphornet.node_challenge.v1" {
		return fmt.Errorf("unexpected challenge purpose %q", challenge.Payload.Purpose)
	}
	if challenge.Payload.ClientPublicKey != clientPublicKey {
		return fmt.Errorf("challenge client public key mismatch")
	}
	if challenge.Payload.NodeID == "" {
		return fmt.Errorf("missing challenge node id")
	}
	if challenge.Payload.NodeName == "" {
		return fmt.Errorf("missing challenge node name")
	}
	if challenge.Payload.Nonce == "" {
		return fmt.Errorf("missing challenge nonce")
	}
	if challenge.Signature == "" {
		return fmt.Errorf("missing challenge signature")
	}
	return identity.VerifyNodeChallenge(challenge.Payload, challenge.Signature)
}

func readLoop(conn *websocket.Conn, program *tea.Program) {
	for {
		ctx := context.Background()
		raw, err := readRawMessage(ctx, conn)
		if err != nil {
			status := websocket.CloseStatus(err)
			if status == websocket.StatusNormalClosure {
				program.Send(connectionClosedMsg{reason: "connection closed"})
				return
			}
			program.Send(errMsg{err: err})
			return
		}
		program.Send(raw)
	}
}
