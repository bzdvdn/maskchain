package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
)

// @sk-task 22-shield-mask-storage#T4.1: Implement MaskHandler (AC-002, AC-003, AC-006)
type MaskHandler struct {
	useCase *mask.MaskUseCase
}

func NewMaskHandler(useCase *mask.MaskUseCase) *MaskHandler {
	return &MaskHandler{useCase: useCase}
}

func (h *MaskHandler) HandleMask(c *gin.Context) {
	maskID := c.Query("mask_id")
	ownID := false
	if maskID == "" {
		maskID = mask.NewUUIDv7()
		ownID = true
	}

	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read request body"})
		return
	}

	maskedText, _, err := h.useCase.MaskText(c.Request.Context(), string(body), maskID)
	if err != nil {
		if errors.Is(err, mask.ErrMaskIDConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "mask_id already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if ownID {
		c.Header("X-Mask-ID", maskID)
	}
	c.String(http.StatusOK, maskedText)
}

func (h *MaskHandler) HandleUnmask(c *gin.Context) {
	maskIDsParam := c.Query("mask_ids")
	if maskIDsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mask_ids is required"})
		return
	}

	maskIDs := strings.Split(maskIDsParam, ",")

	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read request body"})
		return
	}

	restored, err := h.useCase.UnmaskText(c.Request.Context(), string(body), maskIDs)
	if err != nil {
		if errors.Is(err, mask.ErrMaskNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.String(http.StatusOK, restored)
}
