package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/dto"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/preprocessor"
)

// @sk-task 22-shield-mask-storage#T4.1: Implement MaskHandler (AC-002, AC-003, AC-006)
// @sk-task 23-shield-reactions#T1.2: Migrate handler from MaskText to MaskFromResults
// @sk-task 25-shield-preprocessors#T3.2: Integrate preprocessors into MaskHandler (AC-008)
type MaskHandler struct {
	useCase       *mask.MaskUseCase
	registry      *detector.DetectorRegistry
	preprocessors []preprocessor.Processor
}

func NewMaskHandler(useCase *mask.MaskUseCase, registry *detector.DetectorRegistry) *MaskHandler {
	return &MaskHandler{useCase: useCase, registry: registry}
}

// @sk-task 25-shield-preprocessors#T3.2: Add WithPreprocessors setter (AC-008)
func (h *MaskHandler) WithPreprocessors(pps []preprocessor.Processor) {
	h.preprocessors = pps
}

// @sk-task 118-api-consistency#T2.1: Migrate HandleMask to JSON ApiResponse envelope (AC-006, RQ-005)
func (h *MaskHandler) HandleMask(c *gin.Context) {
	maskID := c.Query("mask_id")
	ownID := false
	var docMaskID string
	if maskID == "" {
		maskID = mask.NewShortID()
		docMaskID = maskID
		ownID = true
	} else {
		if !validMaskID(maskID) {
			c.JSON(http.StatusBadRequest, dto.NewErrorResponse("VALIDATION_ERROR", "invalid mask_id: only [a-zA-Z0-9-] allowed, max 64 chars"))
			return
		}
		if len(maskID) > 12 {
			docMaskID = mask.NewShortID()
		} else {
			docMaskID = maskID
		}
	}

	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.NewErrorResponse("VALIDATION_ERROR", "cannot read request body"))
		return
	}

	// Run preprocessors before detectors (DEC-004: fail-open)
	processText := string(body)
	if len(h.preprocessors) > 0 {
		for _, pp := range h.preprocessors {
			r := pp.Process(processText, maskID)
			processText = r.ModifiedText
		}
	}

	var allResults []detector.DetectorResult
	for _, typ := range h.registry.Types() {
		d := h.registry.Get(typ)
		if d == nil {
			continue
		}
		results, scanErr := d.Scan(c.Request.Context(), processText)
		if scanErr != nil {
			c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("DETECTOR_ERROR", fmt.Sprintf("detector %s: %s", typ, scanErr)))
			return
		}
		allResults = append(allResults, results...)
	}

	if tenant, tenantOk := middleware.TenantFromContext(c); tenantOk {
		for _, dict := range tenant.Dictionaries() {
			dd := detector.NewDictionaryDetector(dict)
			results, scanErr := dd.Scan(c.Request.Context(), processText)
			if scanErr != nil {
				continue
			}
			allResults = append(allResults, results...)
		}
	}

	maskedText, _, err := h.useCase.MaskFromResults(c.Request.Context(), processText, maskID, docMaskID, allResults)
	if err != nil {
		if errors.Is(err, mask.ErrMaskIDConflict) {
			c.JSON(http.StatusConflict, dto.NewErrorResponse("MASK_ID_CONFLICT", "mask_id already exists"))
			return
		}
		c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("INTERNAL_ERROR", err.Error()))
		return
	}

	if ownID {
		c.Header("X-Mask-ID", maskID)
	}
	c.Header("X-Document-Mask-ID", docMaskID)
	c.JSON(http.StatusOK, dto.NewSuccessResponse(gin.H{
		"masked_text":      maskedText,
		"mask_id":          maskID,
		"document_mask_id": docMaskID,
	}))
}

func validMaskID(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	for i := 0; i < len(id); i++ {
		b := id[i]
		if (b < 'a' || b > 'z') && (b < 'A' || b > 'Z') && (b < '0' || b > '9') && b != '-' {
			return false
		}
	}
	return true
}

// @sk-task 118-api-consistency#T2.1: Migrate HandleUnmask to JSON ApiResponse envelope (AC-006, RQ-005)
func (h *MaskHandler) HandleUnmask(c *gin.Context) {
	maskIDsParam := c.Query("mask_ids")
	if maskIDsParam == "" {
		c.JSON(http.StatusBadRequest, dto.NewErrorResponse("VALIDATION_ERROR", "mask_ids is required"))
		return
	}

	maskIDs := strings.Split(maskIDsParam, ",")

	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.NewErrorResponse("VALIDATION_ERROR", "cannot read request body"))
		return
	}

	restored, err := h.useCase.UnmaskText(c.Request.Context(), string(body), maskIDs)
	if err != nil {
		if errors.Is(err, mask.ErrMaskNotFound) {
			c.JSON(http.StatusNotFound, dto.NewErrorResponse("NOT_FOUND", err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("INTERNAL_ERROR", err.Error()))
		return
	}

	c.JSON(http.StatusOK, dto.NewSuccessResponse(gin.H{
		"restored_text": restored,
	}))
}
