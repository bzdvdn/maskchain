package conversation

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/dto"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/domain/conversation"
	"github.com/bzdvdn/maskchain/src/internal/infra/crypto"
)

const (
	defaultPerPage = 20
	maxPerPage     = 100
)

// @sk-task conversation-logging#T3.1: ConversationHandler serves admin list/detail read API (AC-005, AC-006)
//
// ConversationHandler represents a domain entity or configuration.
type ConversationHandler struct {
	store conversation.ConversationStore
	enc   *crypto.Encryptor
}

// @sk-task conversation-logging#T3.1: NewConversationHandler creates the handler (AC-005, AC-006)
func NewConversationHandler(store conversation.ConversationStore, enc *crypto.Encryptor) *ConversationHandler {
	return &ConversationHandler{store: store, enc: enc}
}

// @sk-task conversation-logging#T3.1: HandleList returns metadata-only records with pagination (AC-006)
func (h *ConversationHandler) HandleList(c *gin.Context) {
	page := parsePositiveInt(c.Query("page"), 1)
	perPage := parsePositiveInt(c.Query("per_page"), defaultPerPage)
	if perPage > maxPerPage {
		perPage = maxPerPage
	}

	pageResult, err := h.store.List(c.Request.Context(), conversation.ConversationFilter{
		TenantID: c.Query("tenant_id"),
		Status:   conversation.ConversationStatus(c.Query("status")),
		Model:    c.Query("model"),
		Page:     page,
		PerPage:  perPage,
	})
	if err != nil {
		_ = c.Error(err)
		return
	}

	items := make([]dto.ConversationListItem, 0, len(pageResult.Items))
	for _, l := range pageResult.Items {
		items = append(items, dto.ConversationListItem{
			ID:        l.ID,
			TenantID:  l.TenantID,
			Model:     l.Model,
			Status:    string(l.Status),
			Masked:    l.Masked,
			Streamed:  l.Streamed,
			MaskID:    l.MaskID,
			CreatedAt: l.CreatedAt,
		})
	}

	// @sk-task conversation-logging#T3.1: Response is already enveloped, skip middleware double-wrap (AC-006)
	c.Set(middleware.EnvelopedKey, true)
	c.JSON(http.StatusOK, dto.NewSuccessPaginated(dto.ConversationListResponse{Items: items}, page, perPage, pageResult.Total))
}

// @sk-task conversation-logging#T3.1: HandleGet decrypts and base64-wraps a single record (AC-005)
func (h *ConversationHandler) HandleGet(c *gin.Context) {
	log, err := h.store.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, conversation.ErrNotFound) {
			middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "conversation not found")
			return
		}
		_ = c.Error(err)
		return
	}

	req, err := h.encryptedToB64(log.Request)
	if err != nil {
		_ = c.Error(fmt.Errorf("decrypt conversation request: %w", err))
		return
	}
	var respB64 *string
	if len(log.Response) > 0 {
		dec, err := h.encryptedToB64(log.Response)
		if err != nil {
			_ = c.Error(fmt.Errorf("decrypt conversation response: %w", err))
			return
		}
		respB64 = &dec
	}
	var maskingB64 *string
	if log.Masked && len(log.Masking) > 0 {
		dec, err := h.encryptedToB64(log.Masking)
		if err != nil {
			_ = c.Error(fmt.Errorf("decrypt conversation masking: %w", err))
			return
		}
		maskingB64 = &dec
	}

	// @sk-task conversation-logging#T3.1: Response is already enveloped, skip middleware double-wrap (AC-005)
	c.Set(middleware.EnvelopedKey, true)
	c.JSON(http.StatusOK, dto.NewSuccessResponse(dto.ConversationDetail{
		ID:        log.ID,
		TenantID:  log.TenantID,
		Model:     log.Model,
		Status:    string(log.Status),
		Masked:    log.Masked,
		Streamed:  log.Streamed,
		MaskID:    log.MaskID,
		CreatedAt: log.CreatedAt,
		Payload: dto.ConversationPayload{
			Request:  req,
			Response: respB64,
			Masking:  maskingB64,
		},
	}))
}

// encryptedToB64 decrypts ciphertext and base64-std-encodes the plaintext for
// transport; the client decodes it with atob/TextDecoder (AC-005).
func (h *ConversationHandler) encryptedToB64(ciphertext []byte) (string, error) {
	plain, err := h.enc.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(plain), nil
}

func parsePositiveInt(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
