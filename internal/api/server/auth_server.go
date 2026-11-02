package server

import (
	"bytes"
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	pb "umbela-server/internal/api/pb"
	"umbela-server/internal/auth"
	"umbela-server/internal/tor"
	"umbela-server/internal/vault"
)

type AuthServer struct {
	v        *vault.Vault
	sessions *auth.SessionStore
	keyAgent *auth.KeyAgent
	torCtrl  *tor.Controller
}

func NewAuthServer(v *vault.Vault, sessions *auth.SessionStore, keyAgent *auth.KeyAgent, torCtrl *tor.Controller) *AuthServer {
	return &AuthServer{
		v:        v,
		sessions: sessions,
		keyAgent: keyAgent,
		torCtrl:  torCtrl,
	}
}

func (s *AuthServer) Identify(ctx context.Context, req *connect.Request[pb.IdentifyRequest]) (*connect.Response[pb.IdentifyResponse], error) {
	nonce := req.Msg.GetNonce()
	if len(nonce) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("nonce vacío"))
	}

	sig, err := s.torCtrl.SignNonce(nonce)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.IdentifyResponse{Signature: sig}), nil
}

func (s *AuthServer) Unlock(ctx context.Context, req *connect.Request[pb.UnlockRequest]) (*connect.Response[pb.UnlockResponse], error) {
	kek := req.Msg.GetKek()
	if len(kek) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("KEK vacía"))
	}

	// Verificar si la clave enviada por la app es correcta.
	// Si el servidor ya está desbloqueado (por CLI), comparamos en RAM.
	if s.keyAgent.IsUnlocked() {
		currentKey, _ := s.keyAgent.GetKey()
		if !bytes.Equal(currentKey, kek) {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("KEK incorrecta"))
		}
	} else {
		// Si el servidor estaba bloqueado, intenta desbloquear el Vault (necesitarás este método en vault.go)
		// err := s.v.ValidateAndLoadKEK(kek)
		// if err != nil { return err }
		s.keyAgent.SetKey(kek)
	}

	// Emitir token de sesión para la app móvil
	token, expiresAt, err := s.sessions.Create(24 * time.Hour)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.UnlockResponse{
		SessionToken:  token,
		ExpiresAtUnix: expiresAt.Unix(),
	}), nil
}

func (s *AuthServer) GetStatus(ctx context.Context, req *connect.Request[pb.GetStatusRequest]) (*connect.Response[pb.GetStatusResponse], error) {
	unlocked := s.keyAgent.IsUnlocked()
	return connect.NewResponse(&pb.GetStatusResponse{IsUnlocked: unlocked}), nil
}

func (s *AuthServer) Lock(ctx context.Context, req *connect.Request[pb.LockRequest]) (*connect.Response[pb.LockResponse], error) {
	s.keyAgent.ClearKey()
	s.sessions.DestroyAll()
	return connect.NewResponse(&pb.LockResponse{Success: true}), nil
}
