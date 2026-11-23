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

// acquire checks the session key is in RAM and keeps the vault open until the
// returned release is called, so a Lock waits for this request to finish.
func (s *PhotoServer) acquire() (func(), error) {
	if !s.keyAgent.IsUnlocked() {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrVaultLocked)
	}
	release, err := s.v.Acquire()
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrVaultLocked)
	}
	return release, nil
}

func (s *PhotoServer) GetTimeline(ctx context.Context, req *connect.Request[pb.GetTimelineRequest]) (*connect.Response[pb.GetTimelineResponse], error) {
	// Comprobar que el Vault esté desbloqueado antes de consultar
	release, lockErr := s.acquire()
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()

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
	release, lockErr := s.acquire()
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()

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
	release, lockErr := s.acquire()
	if lockErr != nil {
		return lockErr
	}
	defer release()

	// Streaming de la miniatura en fragmentos
	return s.v.StreamThumbnail(ctx, req.Msg.GetPhotoId(), stream)
}

func (s *PhotoServer) StreamFullImage(ctx context.Context, req *connect.Request[pb.GetFullImageRequest], stream *connect.ServerStream[pb.Chunk]) error {
	release, lockErr := s.acquire()
	if lockErr != nil {
		return lockErr
	}
	defer release()

	// Streaming de la foto completa desencriptada en RAM
	return s.v.StreamFullImage(ctx, req.Msg.GetPhotoId(), stream)
}

func (s *PhotoServer) GetPhotoDetails(ctx context.Context, req *connect.Request[pb.GetPhotoDetailsRequest]) (*connect.Response[pb.GetPhotoDetailsResponse], error) {
	release, lockErr := s.acquire()
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()

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
	release, lockErr := s.acquire()
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()

	isFavorite, err := s.v.ToggleFavorite(ctx, req.Msg.GetPhotoId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.ToggleFavoriteResponse{
		IsFavorite: isFavorite,
	}), nil
}

func (s *PhotoServer) DeletePhoto(ctx context.Context, req *connect.Request[pb.DeletePhotoRequest]) (*connect.Response[pb.DeletePhotoResponse], error) {
	release, lockErr := s.acquire()
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()

	if err := s.v.DeletePhoto(ctx, req.Msg.GetPhotoId()); err != nil {
		if errors.Is(err, vault.ErrPhotoNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pb.DeletePhotoResponse{Success: true}), nil
}

func (s *PhotoServer) AddPhotoToAlbum(ctx context.Context, req *connect.Request[pb.AddPhotoToAlbumRequest]) (*connect.Response[pb.AddPhotoToAlbumResponse], error) {
	release, lockErr := s.acquire()
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()

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
	release, lockErr := s.acquire()
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()

	people, err := s.v.ListPeople(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pb.ListPeopleResponse{People: people}), nil
}

func (s *PhotoServer) LabelPerson(ctx context.Context, req *connect.Request[pb.LabelPersonRequest]) (*connect.Response[pb.LabelPersonResponse], error) {
	release, lockErr := s.acquire()
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()

	person, err := s.v.LabelPerson(ctx, req.Msg.GetPersonId(), req.Msg.GetName())
	if errors.Is(err, vault.ErrPersonNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if errors.Is(err, vault.ErrInvalidPersonName) {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pb.LabelPersonResponse{Person: person}), nil
}

func (s *PhotoServer) GetPersonDetails(ctx context.Context, req *connect.Request[pb.GetPersonDetailsRequest]) (*connect.Response[pb.GetPersonDetailsResponse], error) {
	release, lockErr := s.acquire()
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()

	details, err := s.v.GetPersonDetails(ctx, req.Msg.GetPersonId())
	if errors.Is(err, vault.ErrPersonNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(details), nil
}

func (s *PhotoServer) ListAlbums(ctx context.Context, req *connect.Request[pb.ListAlbumsRequest]) (*connect.Response[pb.ListAlbumsResponse], error) {
	release, lockErr := s.acquire()
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()

	albums, err := s.v.ListAlbums(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pb.ListAlbumsResponse{Albums: albums}), nil
}

func (s *PhotoServer) CreateAlbum(ctx context.Context, req *connect.Request[pb.CreateAlbumRequest]) (*connect.Response[pb.CreateAlbumResponse], error) {
	release, lockErr := s.acquire()
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()

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

func (s *PhotoServer) SetPersonCover(ctx context.Context, req *connect.Request[pb.SetPersonCoverRequest]) (*connect.Response[pb.SetPersonCoverResponse], error) {
	release, lockErr := s.acquire()
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()

	person, err := s.v.SetPersonCover(ctx, req.Msg.GetPersonId(), req.Msg.GetPhotoId())
	if errors.Is(err, vault.ErrPersonNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if errors.Is(err, vault.ErrPhotoNotOfPerson) {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pb.SetPersonCoverResponse{Person: person}), nil
}

func (s *PhotoServer) SetAlbumCover(ctx context.Context, req *connect.Request[pb.SetAlbumCoverRequest]) (*connect.Response[pb.SetAlbumCoverResponse], error) {
	release, lockErr := s.acquire()
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()

	album, err := s.v.SetAlbumCover(ctx, req.Msg.GetAlbumId(), req.Msg.GetPhotoId())
	if errors.Is(err, vault.ErrAlbumNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if errors.Is(err, vault.ErrPhotoNotInAlbum) {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pb.SetAlbumCoverResponse{Album: album}), nil
}

func (s *PhotoServer) FilterNewPhotos(ctx context.Context, req *connect.Request[pb.FilterNewPhotosRequest]) (*connect.Response[pb.FilterNewPhotosResponse], error) {
	release, lockErr := s.acquire()
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()

	missing, err := s.v.MissingHashes(ctx, req.Msg.GetHashes())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pb.FilterNewPhotosResponse{Missing: missing}), nil
}

func (s *PhotoServer) UploadPhoto(ctx context.Context, req *connect.Request[pb.UploadPhotoRequest]) (*connect.Response[pb.UploadPhotoResponse], error) {
	release, lockErr := s.acquire()
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()

	id, dup, err := s.v.UploadPhoto(ctx, req.Msg.GetData(), req.Msg.GetFilename(), req.Msg.GetCapturedAtUnix())
	if errors.Is(err, vault.ErrUnsupportedFile) {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pb.UploadPhotoResponse{PhotoId: id, Duplicate: dup}), nil
}
