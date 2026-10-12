package controllers

import (
	"net/http"
	"slices"
	"strings"

	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/media"
	"retrotide/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	mediaKindQuery  = "kind"
	uploadFileParam = "file"
	uploadExtension = ".webp"
)

var (
	uploadContentTypes = []string{"image/png", "image/jpeg", "image/webp"}

	mediaErrorAnswers = slices.Concat([]errorAnswer{
		{err: media.ErrUnsupportedImage, status: http.StatusBadRequest, code: "unsupported_image"},
		{err: media.ErrInvalidKind, status: http.StatusBadRequest, code: "invalid_media_kind"},
		{err: media.ErrImageDimensions, status: http.StatusBadRequest, code: "image_dimensions"},
		{err: media.ErrMediaNotFound, status: http.StatusNotFound},
		{err: media.ErrEncoderUnavailable, status: http.StatusServiceUnavailable, code: "images_unavailable", recordCause: true},
	}, boardErrorAnswers)
)

func (s *Service) getAllMediaRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodPost, Path: "/boards/:boardID/media", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.uploadMedia},
	}
}

func (s *Service) getAllUploadRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodGet, Path: "/uploads/:boardID/:file", Before: gin.HandlersChain{s.Guards.Principal}, Handler: s.serveUpload},
	}
}

func (s *Service) uploadMedia(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	if !slices.Contains(uploadContentTypes, c.ContentType()) {
		answerError(c, mediaErrorAnswers, media.ErrUnsupportedImage, "failed to upload the image")
		return
	}

	upload := media.UploadCommand{
		Principal:     command.Principal,
		BoardID:       command.BoardID,
		Kind:          model.MediaKind(c.Query(mediaKindQuery)),
		ContentLength: c.Request.ContentLength,
		Body:          c.Request.Body,
	}

	view, err := s.MediaService.Upload(c.Request.Context(), upload)
	if mediaFailed(c, err, "failed to upload the image") {
		return
	}

	c.JSON(http.StatusCreated, view)
}

func (s *Service) serveUpload(c *gin.Context) {
	principal, ok := utils.Principal(c)
	if !ok {
		return
	}

	boardID, boardErr := uuid.Parse(c.Param(boardIDParam))
	mediaID, mediaOK := uploadMediaID(c.Param(uploadFileParam))
	if boardErr != nil || !mediaOK {
		c.AbortWithStatusJSON(http.StatusNotFound, dto.ErrorResponse{Error: media.ErrMediaNotFound.Error()})
		return
	}

	file, err := s.MediaService.Open(c.Request.Context(), media.OpenQuery{Principal: principal, BoardID: boardID, MediaID: mediaID})
	if mediaFailed(c, err, "failed to read the image") {
		return
	}

	c.Data(http.StatusOK, model.MediaContentType, file.Data)
}

func uploadMediaID(file string) (uuid.UUID, bool) {
	name, found := strings.CutSuffix(file, uploadExtension)
	if !found {
		return uuid.Nil, false
	}

	id, err := uuid.Parse(name)
	if err != nil {
		return uuid.Nil, false
	}

	return id, true
}

func mediaFailed(c *gin.Context, err error, msg string) bool {
	if err == nil {
		return false
	}

	if utils.MapEntitlementError(c, err) {
		return true
	}

	answerError(c, mediaErrorAnswers, err, msg)

	return true
}
