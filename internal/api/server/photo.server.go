package server

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	pb "umbela-server/internal/api/pb"
	"umbela-server/internal/auth"
	"umbela-server/internal/vault"
)

type PhotoServer struct {
	v        *vault.Vault
	keyAgent *auth.KeyAgent
}

func NewPhotoServer(v *vault.Vault, keyAgent *auth.KeyAgent) *PhotoServer {
	return &PhotoServer{
		v:        v,
		keyAgent: keyAgent,
	}
}

func (s *PhotoServer) GetTimeline(ctx context.Context, req *connect.Request[pb.GetTimelineRequest]) (*connect.Response[pb.GetTimelineResponse], error) {
	// Comprobar que el Vault esté desbloqueado antes de consultar
	if !s.keyAgent.IsUnlocked() {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrVaultLocked)
	}

	var (
		photos []*pb.PhotoSummary
		next   string
		err    error
	)
	if albumID := req.Msg.GetAlbumId(); albumID != "" {
		photos, next, err = s.v.GetAlbumTimelinePage(
			ctx,
			albumID,
			req.Msg.GetFromUnix(),
			req.Msg.GetToUnix(),
			int(req.Msg.GetPageSize()),
			req.Msg.GetCursor(),
		)
	} else {
		photos, next, err = s.v.GetTimelinePage(
			ctx,
			req.Msg.GetFromUnix(),
			req.Msg.GetToUnix(),
			int(req.Msg.GetPageSize()),
			req.Msg.GetCursor(),
		)
	}
	if errors.Is(err, vault.ErrInvalidCursor) {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.GetTimelineResponse{
		Photos:     photos,
		NextCursor: next,
	}), nil
}

// maxTZOffsetSeconds is the largest real UTC offset (+14:00) with margin.
const maxTZOffsetSeconds = 15 * 3600

func (s *PhotoServer) GetTimelineBuckets(ctx context.Context, req *connect.Request[pb.GetTimelineBucketsRequest]) (*connect.Response[pb.GetTimelineBucketsResponse], error) {
	if !s.keyAgent.IsUnlocked() {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrVaultLocked)
	}

	tz := int(req.Msg.GetTzOffsetSeconds())
	if tz < -maxTZOffsetSeconds || tz > maxTZOffsetSeconds {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("tz_offset_seconds out of range"))
	}

	buckets, err := s.v.GetTimelineBuckets(ctx, tz)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pb.GetTimelineBucketsResponse{Buckets: buckets}), nil
}

func (s *PhotoServer) StreamThumbnail(ctx context.Context, req *connect.Request[pb.GetThumbnailRequest], stream *connect.ServerStream[pb.Chunk]) error {
	if !s.keyAgent.IsUnlocked() {
		return connect.NewError(connect.CodeUnauthenticated, auth.ErrVaultLocked)
	}

	// Streaming de la miniatura en fragmentos
	return s.v.StreamThumbnail(ctx, req.Msg.GetPhotoId(), stream)
}

func (s *PhotoServer) StreamFullImage(ctx context.Context, req *connect.Request[pb.GetFullImageRequest], stream *connect.ServerStream[pb.Chunk]) error {
	if !s.keyAgent.IsUnlocked() {
		return connect.NewError(connect.CodeUnauthenticated, auth.ErrVaultLocked)
	}

	// Streaming de la foto completa desencriptada en RAM
	return s.v.StreamFullImage(ctx, req.Msg.GetPhotoId(), stream)
}

func (s *PhotoServer) GetPhotoDetails(ctx context.Context, req *connect.Request[pb.GetPhotoDetailsRequest]) (*connect.Response[pb.GetPhotoDetailsResponse], error) {
	if !s.keyAgent.IsUnlocked() {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrVaultLocked)
	}

	details, err := s.v.GetPhotoDetails(ctx, req.Msg.GetPhotoId())
	if errors.Is(err, vault.ErrPhotoNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pb.GetPhotoDetailsResponse{Photo: details}), nil
}

func (s *PhotoServer) ToggleFavorite(ctx context.Context, req *connect.Request[pb.ToggleFavoriteRequest]) (*connect.Response[pb.ToggleFavoriteResponse], error) {
	if !s.keyAgent.IsUnlocked() {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrVaultLocked)
	}

	isFavorite, err := s.v.ToggleFavorite(ctx, req.Msg.GetPhotoId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.ToggleFavoriteResponse{
		IsFavorite: isFavorite,
	}), nil
}

func (s *PhotoServer) DeletePhoto(ctx context.Context, req *connect.Request[pb.DeletePhotoRequest]) (*connect.Response[pb.DeletePhotoResponse], error) {
	if !s.keyAgent.IsUnlocked() {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrVaultLocked)
	}

	if err := s.v.DeletePhoto(ctx, req.Msg.GetPhotoId()); err != nil {
		if errors.Is(err, vault.ErrPhotoNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pb.DeletePhotoResponse{Success: true}), nil
}

func (s *PhotoServer) AddPhotoToAlbum(ctx context.Context, req *connect.Request[pb.AddPhotoToAlbumRequest]) (*connect.Response[pb.AddPhotoToAlbumResponse], error) {
	if !s.keyAgent.IsUnlocked() {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrVaultLocked)
	}

	err := s.v.AddPhotoToAlbum(ctx, req.Msg.GetPhotoId(), req.Msg.GetAlbumId())
	if errors.Is(err, vault.ErrPhotoNotFound) || errors.Is(err, vault.ErrAlbumNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pb.AddPhotoToAlbumResponse{Success: true}), nil
}

func (s *PhotoServer) ListPeople(ctx context.Context, req *connect.Request[pb.ListPeopleRequest]) (*connect.Response[pb.ListPeopleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("no implementado aún"))
}

func (s *PhotoServer) LabelPerson(ctx context.Context, req *connect.Request[pb.LabelPersonRequest]) (*connect.Response[pb.LabelPersonResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("no implementado aún"))
}

func (s *PhotoServer) GetPersonDetails(ctx context.Context, req *connect.Request[pb.GetPersonDetailsRequest]) (*connect.Response[pb.GetPersonDetailsResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("no implementado aún"))
}

func (s *PhotoServer) ListAlbums(ctx context.Context, req *connect.Request[pb.ListAlbumsRequest]) (*connect.Response[pb.ListAlbumsResponse], error) {
	if !s.keyAgent.IsUnlocked() {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrVaultLocked)
	}

	albums, err := s.v.ListAlbums(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pb.ListAlbumsResponse{Albums: albums}), nil
}

func (s *PhotoServer) CreateAlbum(ctx context.Context, req *connect.Request[pb.CreateAlbumRequest]) (*connect.Response[pb.CreateAlbumResponse], error) {
	if !s.keyAgent.IsUnlocked() {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrVaultLocked)
	}

	album, err := s.v.CreateAlbum(ctx, req.Msg.GetName())
	if errors.Is(err, vault.ErrInvalidAlbumName) {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pb.CreateAlbumResponse{Album: album}), nil
}

func (s *PhotoServer) ListPlaces(ctx context.Context, req *connect.Request[pb.ListPlacesRequest]) (*connect.Response[pb.ListPlacesResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("no implementado aún"))
}
