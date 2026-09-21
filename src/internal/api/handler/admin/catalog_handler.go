package admin

import (
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/compliance"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

// @sk-task shield-detector-catalog#T1.2: shield catalog handler (AC-001, AC-002, AC-004, AC-005)
//
// CatalogHandler serves the read-only shield catalog: the detector types the
// process actually serves, the allowed reactions, and the loaded compliance
// packs. It reads in-memory registries only, so it is cheap and deterministic.
type CatalogHandler struct {
	detectors []entity.DetectorType
	packs     *compliance.Registry
}

// NewCatalogHandler builds the handler. packs may be nil (compliance disabled);
// the catalog then reports an empty pack list.
func NewCatalogHandler(detectors []entity.DetectorType, packs *compliance.Registry) *CatalogHandler {
	return &CatalogHandler{detectors: detectors, packs: packs}
}

type catalogPack struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type catalogResponse struct {
	Detectors []string      `json:"detectors"`
	Reactions []string      `json:"reactions"`
	Packs     []catalogPack `json:"packs"`
}

// Handle returns the shield catalog.
func (h *CatalogHandler) Handle(c *gin.Context) {
	c.JSON(http.StatusOK, h.build())
}

func (h *CatalogHandler) build() catalogResponse {
	detectors := make([]string, 0, len(h.detectors))
	for _, d := range h.detectors {
		detectors = append(detectors, string(d))
	}
	sort.Strings(detectors)

	reactions := []string{
		string(entity.ReactionAllow),
		string(entity.ReactionBlock),
		string(entity.ReactionReview),
		string(entity.ReactionLog),
	}
	sort.Strings(reactions)

	packs := make([]catalogPack, 0)
	if h.packs != nil {
		for key, p := range h.packs.Packs() {
			name := key
			if p != nil && p.Name != "" {
				name = p.Name
			}
			packs = append(packs, catalogPack{Key: key, Name: name})
		}
	}
	sort.Slice(packs, func(i, j int) bool { return packs[i].Key < packs[j].Key })

	return catalogResponse{Detectors: detectors, Reactions: reactions, Packs: packs}
}
