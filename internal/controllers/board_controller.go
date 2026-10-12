package controllers

import (
	"errors"
	"math"
	"net/http"
	"strconv"

	"retrotide/internal/board"
	"retrotide/internal/board/actionitems"
	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"

	"github.com/gin-gonic/gin"
)

const (
	boardIDParam      = "boardID"
	columnIDParam     = "columnID"
	cardIDParam       = "cardID"
	knownVersionQuery = "known_version"
	retryAfterHeader  = "Retry-After"
)

var (
	boardErrorAnswers = []errorAnswer{
		{err: board.ErrInvalidID, status: http.StatusBadRequest, code: utils.InvalidClientIDCode},
		{err: board.ErrOpRequired, status: http.StatusBadRequest, code: "op_required"},
		{err: board.ErrTitleRequired, status: http.StatusBadRequest, code: "board_title_required"},
		{err: board.ErrTitleTooLong, status: http.StatusBadRequest, code: "board_title_too_long"},
		{err: board.ErrUnknownTemplate, status: http.StatusBadRequest, code: "unknown_template"},
		{err: board.ErrInvalidAnonymity, status: http.StatusBadRequest, code: "invalid_anonymity"},
		{err: board.ErrContextTooLong, status: http.StatusBadRequest, code: "board_context_too_long"},
		{err: board.ErrColumnTitleRequired, status: http.StatusBadRequest, code: "column_title_required"},
		{err: board.ErrColumnTitleTooLong, status: http.StatusBadRequest, code: "column_title_too_long"},
		{err: board.ErrColumnDescriptionTooLong, status: http.StatusBadRequest, code: "column_description_too_long"},
		{err: board.ErrInvalidColour, status: http.StatusBadRequest, code: "invalid_colour"},
		{err: board.ErrCardBodyRequired, status: http.StatusBadRequest, code: "card_body_required"},
		{err: board.ErrCardBodyTooLong, status: http.StatusBadRequest, code: "card_body_too_long"},
		{err: board.ErrInvalidLinkAccess, status: http.StatusBadRequest, code: "invalid_link_access"},
		{err: board.ErrPasswordBlank, status: http.StatusBadRequest, code: "board_password_blank"},
		{err: board.ErrPasswordTooLong, status: http.StatusBadRequest, code: "board_password_too_long"},
		{err: board.ErrCommentBodyRequired, status: http.StatusBadRequest, code: "comment_body_required"},
		{err: board.ErrCommentBodyTooLong, status: http.StatusBadRequest, code: "comment_body_too_long"},
		{err: board.ErrInvalidMaxVotes, status: http.StatusBadRequest, code: "invalid_max_votes"},
		{err: board.ErrInvalidSortMode, status: http.StatusBadRequest, code: "invalid_sort_mode"},
		{err: board.ErrInvalidTimerAction, status: http.StatusBadRequest, code: "invalid_timer_action"},
		{err: board.ErrInvalidTimerDuration, status: http.StatusBadRequest, code: "invalid_timer_duration"},
		{err: board.ErrInvalidRole, status: http.StatusBadRequest, code: "invalid_role"},
		{err: board.ErrInvalidStage, status: http.StatusBadRequest, code: "invalid_stage"},
		{err: board.ErrStageNotNext, status: http.StatusConflict, code: "stage_not_next"},
		{err: actionitems.ErrTitleRequired, status: http.StatusBadRequest, code: "action_item_title_required"},
		{err: actionitems.ErrTitleTooLong, status: http.StatusBadRequest, code: "action_item_title_too_long"},
		{err: actionitems.ErrInvalidDueDate, status: http.StatusBadRequest, code: "invalid_due_date"},
		{err: actionitems.ErrCardRequired, status: http.StatusBadRequest, code: "card_required"},
		{err: board.ErrTooManyMentions, status: http.StatusBadRequest, code: "too_many_mentions"},
		{err: actionitems.ErrInvalidStatus, status: http.StatusBadRequest, code: "invalid_action_item_status"},
		{err: actionitems.ErrInvalidReviewOutcome, status: http.StatusBadRequest, code: "invalid_review_outcome"},
		{err: board.ErrNotBoardMember, status: http.StatusBadRequest, code: "not_board_member"},
		{err: policy.ErrActionItemsOutOfStage, status: http.StatusConflict, code: "wrong_stage"},
		{err: policy.ErrActionItemNotFound, status: http.StatusNotFound},
		{err: policy.ErrMembersOnly, status: http.StatusForbidden, code: "members_only"},
		{err: policy.ErrCardsOutOfStage, status: http.StatusConflict, code: "wrong_stage"},
		{err: policy.ErrVotesOutOfStage, status: http.StatusConflict, code: "wrong_stage"},
		{err: policy.ErrCommentsOutOfStage, status: http.StatusConflict, code: "wrong_stage"},
		{err: policy.ErrMergesOutOfStage, status: http.StatusConflict, code: "wrong_stage"},
		{err: policy.ErrHideOutOfStage, status: http.StatusConflict, code: "wrong_stage"},
		{err: board.ErrParticipantNotFound, status: http.StatusNotFound},
		{err: board.ErrTimerTooLong, status: http.StatusConflict, code: "timer_too_long"},
		{err: board.ErrTimerNotRunning, status: http.StatusConflict, code: "timer_not_running"},
		{err: board.ErrTimerNotPaused, status: http.StatusConflict, code: "timer_not_paused"},
		{err: board.ErrCannotPromote, status: http.StatusConflict, code: "cannot_promote"},
		{err: board.ErrLeadFacilitates, status: http.StatusConflict, code: "lead_facilitates"},
		{err: board.ErrRemoveSelf, status: http.StatusConflict, code: "remove_self"},
		{err: policy.ErrMergeIntoSelf, status: http.StatusBadRequest, code: "merge_into_self"},
		{err: live.ErrWrongPassword, status: http.StatusForbidden, code: "wrong_password"},
		{err: live.ErrGuestsNotAllowed, status: http.StatusForbidden, code: "guests_not_allowed"},
		{err: live.ErrBoardFull, status: http.StatusTooManyRequests, code: "board_full"},
		{err: live.ErrNoAccess, status: http.StatusNotFound},
		{err: board.ErrTeamNotFound, status: http.StatusNotFound},
		{err: board.ErrOrganisationNotFound, status: http.StatusNotFound},
		{err: board.ErrColumnNotFound, status: http.StatusNotFound},
		{err: board.ErrTemplateNotFound, status: http.StatusNotFound},
		{err: policy.ErrCardNotFound, status: http.StatusNotFound},
		{err: policy.ErrCommentNotFound, status: http.StatusNotFound},
		{err: policy.ErrNotAllowed, status: http.StatusForbidden},
		{err: board.ErrCreateNotAllowed, status: http.StatusForbidden, code: "create_not_allowed"},
		{err: live.ErrRemoved, status: http.StatusForbidden, code: "removed"},
		{err: live.ErrLinkDisabled, status: http.StatusForbidden, code: "link_disabled"},
		{err: live.ErrPasswordRequired, status: http.StatusForbidden, code: "password_required"},
		{err: live.ErrAccountRequired, status: http.StatusForbidden, code: "account_required"},
		{err: board.ErrIDConflict, status: http.StatusConflict, code: "id_conflict"},
		{err: board.ErrTeamArchived, status: http.StatusConflict, code: "team_archived"},
		{err: live.ErrArchived, status: http.StatusConflict, code: "archived"},
		{err: policy.ErrReadOnly, status: http.StatusConflict, code: "read_only"},
		{err: policy.ErrCardsLocked, status: http.StatusConflict, code: "cards_locked"},
		{err: policy.ErrMovesLocked, status: http.StatusConflict, code: "moves_locked"},
		{err: policy.ErrNotParticipant, status: http.StatusConflict, code: "not_participant"},
		{err: policy.ErrVotingClosed, status: http.StatusConflict, code: "voting_closed"},
		{err: policy.ErrCommentsDisabled, status: http.StatusConflict, code: "comments_disabled"},
		{err: policy.ErrMergingDisabled, status: http.StatusConflict, code: "merging_disabled"},
		{err: policy.ErrCardsHidden, status: http.StatusConflict, code: "cards_hidden"},
		{err: policy.ErrNotMerged, status: http.StatusConflict, code: "not_merged"},
		{err: board.ErrVoteLimitReached, status: http.StatusConflict, code: "vote_limit_reached"},
		{err: board.ErrCardVoteLimitReached, status: http.StatusConflict, code: "card_vote_limit_reached"},
		{err: board.ErrNoVote, status: http.StatusConflict, code: "no_vote"},
		{err: policy.ErrColumnHidden, status: http.StatusConflict, code: "column_hidden"},
		{err: policy.ErrMoveAcrossReveal, status: http.StatusConflict, code: "column_revealed"},
		{err: policy.ErrCardNotShown, status: http.StatusConflict, code: "card_not_shown"},
		{err: board.ErrColumnVoteLimitReached, status: http.StatusConflict, code: "column_vote_limit_reached"},
		{err: board.ErrInvalidColumnVoteLimit, status: http.StatusBadRequest, code: "invalid_column_vote_limit"},
		{err: live.ErrBusy, status: http.StatusServiceUnavailable, code: "board_busy"},
	}
)

func (s *Service) getAllBoardRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodGet, Path: "/orgs/:orgID/boards", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgView)}, Handler: s.listOrganisationBoards},
		{Method: http.MethodPost, Path: "/orgs/:orgID/boards", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgCreateBoard)}, Handler: s.createOrganisationBoard},
		{Method: http.MethodGet, Path: "/teams/:teamID/boards", Before: gin.HandlersChain{s.Guards.TeamPermission(orgauthz.PermTeamView)}, Handler: s.listBoards},
		{Method: http.MethodPost, Path: "/teams/:teamID/boards", Before: gin.HandlersChain{s.Guards.TeamPermission(orgauthz.PermTeamCreateBoard)}, Handler: s.createBoard},
		{Method: http.MethodGet, Path: "/boards/:boardID", Before: gin.HandlersChain{s.Guards.Principal}, Handler: s.getBoard},
		{Method: http.MethodPost, Path: "/boards/:boardID/participants/me", Before: gin.HandlersChain{s.Guards.Principal}, Handler: s.joinBoard},
		{Method: http.MethodPut, Path: "/boards/:boardID/context", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.describeBoard},
		{Method: http.MethodGet, Path: "/boards/:boardID/sharing", Before: gin.HandlersChain{s.Guards.Principal}, Handler: s.getSharing},
		{Method: http.MethodPut, Path: "/boards/:boardID/sharing/link-access", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.setLinkAccess},
		{Method: http.MethodPut, Path: "/boards/:boardID/sharing/password", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.setBoardPassword},
		{Method: http.MethodPost, Path: "/boards/:boardID/sharing/rotate", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.rotateLink},
	}
}

func (s *Service) listBoards(c *gin.Context) {
	teamID, ok := utils.ParseIDParam(c, teamIDParam)
	if !ok {
		return
	}

	listing, err := s.BoardService.List(c.Request.Context(), teamID)
	if boardFailed(c, err, "failed to list the boards") {
		return
	}

	c.JSON(http.StatusOK, listing)
}

func (s *Service) listOrganisationBoards(c *gin.Context) {
	viewer, ok := orgViewer(c)
	if !ok {
		return
	}

	listing, err := s.BoardService.ListForOrganisation(c.Request.Context(), viewer)
	if boardFailed(c, err, "failed to list the boards") {
		return
	}

	c.JSON(http.StatusOK, listing)
}

func (s *Service) createBoard(c *gin.Context) {
	principal, ok := utils.Principal(c)
	if !ok {
		return
	}

	teamID, ok := utils.ParseIDParam(c, teamIDParam)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.CreateBoardRequest](c)
	if !ok {
		return
	}

	command := board.CreateCommand{
		Principal:       principal,
		TeamID:          new(teamID),
		Title:           req.Title,
		TemplateKey:     req.TemplateKey,
		SavedTemplateID: req.SavedTemplateID,
		Context:         req.Context,
		Anonymity:       req.Anonymity,
		CardsShown:      req.CardsShown,
	}

	created, err := s.BoardService.Create(c.Request.Context(), command)
	if boardFailed(c, err, "failed to create the board") {
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (s *Service) createOrganisationBoard(c *gin.Context) {
	principal, ok := utils.Principal(c)
	if !ok {
		return
	}

	orgID, ok := utils.ParseIDParam(c, orgIDParam)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.CreateOrgBoardRequest](c)
	if !ok {
		return
	}

	command := board.CreateCommand{
		Principal:       principal,
		OrgID:           orgID,
		TeamID:          req.TeamID,
		Title:           req.Title,
		TemplateKey:     req.TemplateKey,
		SavedTemplateID: req.SavedTemplateID,
		Context:         req.Context,
		Anonymity:       req.Anonymity,
		CardsShown:      req.CardsShown,
	}

	created, err := s.BoardService.Create(c.Request.Context(), command)
	if boardFailed(c, err, "failed to create the board") {
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (s *Service) getBoard(c *gin.Context) {
	principal, ok := utils.Principal(c)
	if !ok {
		return
	}

	boardID, ok := utils.ParseIDParam(c, boardIDParam)
	if !ok {
		return
	}

	query := spec.BoardSnapshotQuery{Principal: principal, BoardID: boardID}

	raw, present := c.GetQuery(knownVersionQuery)
	if present {
		known, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid " + knownVersionQuery})
			return
		}

		query.KnownVersion = &known
	}

	snapshot, err := s.BoardService.GetSnapshot(c.Request.Context(), query)
	if errors.Is(err, board.ErrNotModified) {
		noContent(c)
		return
	}
	if boardFailed(c, err, "failed to load the board") {
		return
	}

	c.JSON(http.StatusOK, snapshot)
}

func (s *Service) joinBoard(c *gin.Context) {
	principal, ok := utils.Principal(c)
	if !ok {
		return
	}

	boardID, ok := utils.ParseIDParam(c, boardIDParam)
	if !ok {
		return
	}

	_, err := s.BoardLive.JoinAsMember(c.Request.Context(), spec.BoardAccessQuery{Principal: principal, BoardID: boardID})
	if boardFailed(c, err, "failed to join the board") {
		return
	}

	noContent(c)
}

func (s *Service) describeBoard(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.DescribeBoardRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	result, err := s.BoardService.Describe(c.Request.Context(), board.DescribeCommand{Command: command, Context: req.Context})
	if boardFailed(c, err, "failed to describe the board") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func boardCommand(c *gin.Context) (board.Command, bool) {
	principal, ok := utils.Principal(c)
	if !ok {
		return board.Command{}, false
	}

	boardID, ok := utils.ParseIDParam(c, boardIDParam)
	if !ok {
		return board.Command{}, false
	}

	return board.Command{Principal: principal, BoardID: boardID}, true
}

func (s *Service) getSharing(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	sharing, err := s.BoardService.Sharing(c.Request.Context(), spec.BoardAccessQuery{Principal: command.Principal, BoardID: command.BoardID})
	if boardFailed(c, err, "failed to load the board's sharing") {
		return
	}

	c.JSON(http.StatusOK, sharing)
}

func (s *Service) setLinkAccess(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.SetLinkAccessRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	result, err := s.BoardService.SetLinkAccess(c.Request.Context(), board.SetLinkAccessCommand{Command: command, LinkAccess: req.LinkAccess})
	answerShared(c, result, err, "failed to change the board's link access")
}

func (s *Service) setBoardPassword(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.SetBoardPasswordRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	result, err := s.BoardService.SetPassword(c.Request.Context(), board.SetPasswordCommand{Command: command, Password: req.Password})
	answerShared(c, result, err, "failed to change the board's password")
}

func (s *Service) rotateLink(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.BoardCommandRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	result, err := s.BoardService.RotateLink(c.Request.Context(), command)
	answerShared(c, result, err, "failed to rotate the board's link")
}

func answerShared(c *gin.Context, result *dto.BoardCommandResponse, err error, msg string) {
	if errors.Is(err, board.ErrNotAudited) && result != nil {
		c.Error(err)
		c.JSON(http.StatusOK, result)
		return
	}

	if boardFailed(c, err, msg) {
		return
	}

	c.JSON(http.StatusOK, result)
}

func boardFailed(c *gin.Context, err error, msg string) bool {
	if err == nil {
		return false
	}

	if retry, ok := errors.AsType[*live.RetryError](err); ok {
		c.Header(retryAfterHeader, strconv.Itoa(max(1, int(math.Ceil(retry.RetryAfter.Seconds())))))
		c.AbortWithStatusJSON(http.StatusTooManyRequests, dto.ErrorResponse{Error: live.ErrPasswordAttempts.Error(), Code: "password_attempts"})
		return true
	}

	answerError(c, boardErrorAnswers, err, msg)

	return true
}
