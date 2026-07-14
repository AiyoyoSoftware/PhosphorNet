package node

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"phosphornet/internal/action"
	"phosphornet/internal/storage"
)

const nodeIdentityStateKey = "node.identity"

type auditSink struct {
	mu   sync.Mutex
	file io.Writer
}

type serverOptions struct {
	AuditLogFile     io.Writer
	AuditLogMaxBytes int64
	DisconnectGrace  time.Duration
	ActionExecutor   action.Executor
}

type rotatingAuditFile struct {
	mu         sync.Mutex
	path       string
	file       *os.File
	maxBytes   int64
	maxBackups int
}

func openRotatingAuditFile(path string, maxBytes int64, maxBackups int) (*rotatingAuditFile, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	if maxBackups < 1 {
		maxBackups = 1
	}
	return &rotatingAuditFile{
		path:       path,
		file:       file,
		maxBytes:   maxBytes,
		maxBackups: maxBackups,
	}, nil
}

func (f *rotatingAuditFile) Write(data []byte) (int, error) {
	if f == nil || f.file == nil {
		return 0, fmt.Errorf("audit log file is not open")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.rotateIfNeeded(int64(len(data))); err != nil {
		return 0, err
	}
	return f.file.Write(data)
}

func (f *rotatingAuditFile) Close() error {
	if f == nil || f.file == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	err := f.file.Close()
	f.file = nil
	return err
}

func (f *rotatingAuditFile) rotateIfNeeded(incomingBytes int64) error {
	if f.maxBytes <= 0 {
		return nil
	}
	info, err := f.file.Stat()
	if err != nil {
		return err
	}
	if info.Size() == 0 || info.Size()+incomingBytes <= f.maxBytes {
		return nil
	}
	if err := f.file.Close(); err != nil {
		return err
	}
	for i := f.maxBackups - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", f.path, i), fmt.Sprintf("%s.%d", f.path, i+1))
	}
	_ = os.Remove(fmt.Sprintf("%s.%d", f.path, f.maxBackups+1))
	_ = os.Rename(f.path, f.path+".1")
	file, err := os.OpenFile(f.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	f.file = file
	return nil
}

func (s *Server) audit(ctx context.Context, event storage.AuditEvent) {
	if s == nil {
		return
	}
	if event.ActorPublicKey != "" && event.ActorFingerprint == "" {
		event.ActorFingerprint = fingerprintForPublicKey(event.ActorPublicKey)
	}
	if event.Metadata == nil {
		event.Metadata = map[string]any{}
	}

	recorded := event
	if s.store != nil {
		stored, err := s.store.AppendAuditEvent(ctx, event)
		if err != nil {
			log.Printf("audit event append failed action=%s target=%s: %v", event.Action, event.Target, err)
		} else {
			recorded = stored
		}
		if s.auditMaxBytes > 0 {
			if err := s.store.TrimAuditEventsToMaxBytes(ctx, s.auditMaxBytes); err != nil {
				log.Printf("audit event trim failed max_bytes=%d: %v", s.auditMaxBytes, err)
			}
		}
	}
	if s.auditLog == nil || s.auditLog.file == nil {
		return
	}
	s.auditLog.mu.Lock()
	defer s.auditLog.mu.Unlock()
	data, err := json.Marshal(recorded)
	if err != nil {
		log.Printf("audit event encode failed action=%s target=%s: %v", event.Action, event.Target, err)
		return
	}
	if _, err := fmt.Fprintln(s.auditLog.file, string(data)); err != nil {
		log.Printf("audit log file write failed action=%s target=%s: %v", event.Action, event.Target, err)
	}
}

func auditEvent(actorPublicKey, action, target, result string, metadata map[string]any) storage.AuditEvent {
	return storage.AuditEvent{
		ActorPublicKey: actorPublicKey,
		Action:         action,
		Target:         target,
		Result:         result,
		Metadata:       metadata,
	}
}

func (s *Server) rememberNodeIdentity(ctx context.Context) error {
	if s == nil || s.store == nil {
		return nil
	}
	state, err := s.store.LoadNodeState(ctx, nodeIdentityStateKey)
	if err != nil {
		return err
	}
	previous, _ := state["public_key"].(string)
	if previous != "" && previous != s.cfg.NodeID {
		s.audit(ctx, storage.AuditEvent{
			Action: "node.key_changed",
			Target: s.cfg.Name,
			Result: "success",
			Metadata: map[string]any{
				"previous_public_key":  previous,
				"previous_fingerprint": fingerprintForPublicKey(previous),
				"current_public_key":   s.cfg.NodeID,
				"current_fingerprint":  fingerprintForPublicKey(s.cfg.NodeID),
			},
		})
	}
	return s.store.SaveNodeState(ctx, nodeIdentityStateKey, map[string]any{
		"public_key":  s.cfg.NodeID,
		"fingerprint": fingerprintForPublicKey(s.cfg.NodeID),
	})
}
