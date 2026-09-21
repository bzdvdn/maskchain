package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/compliance"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

type catalogBody struct {
	Detectors []string `json:"detectors"`
	Reactions []string `json:"reactions"`
	Packs     []struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"packs"`
}

func getCatalog(t *testing.T, h *CatalogHandler) (int, catalogBody) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/shield/catalog", h.Handle)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/shield/catalog", nil)
	router.ServeHTTP(w, req)

	var body catalogBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode catalog: %v (%s)", err, w.Body.String())
	}
	return w.Code, body
}

func packsRegistry(t *testing.T, packs ...*compliance.Pack) *compliance.Registry {
	t.Helper()
	reg := compliance.NewRegistry()
	for _, p := range packs {
		reg.Add(p)
	}
	return reg
}

// @sk-test shield-detector-catalog#T4.1: catalog reports detectors and reactions (AC-001)
func TestCatalogDetectorsAndReactions(t *testing.T) {
	h := NewCatalogHandler([]entity.DetectorType{
		entity.DetectorTypeRegex,
		entity.DetectorTypeDictionary,
		entity.DetectorTypePromptInjection,
	}, nil)

	code, body := getCatalog(t, h)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	want := []string{"dictionary", "prompt_injection", "regex"}
	if len(body.Detectors) != len(want) {
		t.Fatalf("detectors = %v, want %v", body.Detectors, want)
	}
	for i := range want {
		if body.Detectors[i] != want[i] {
			t.Errorf("detectors[%d] = %q, want %q", i, body.Detectors[i], want[i])
		}
	}
	wantReactions := []string{"allow", "block", "log", "review"}
	if len(body.Reactions) != len(wantReactions) {
		t.Fatalf("reactions = %v, want %v", body.Reactions, wantReactions)
	}
	for i := range wantReactions {
		if body.Reactions[i] != wantReactions[i] {
			t.Errorf("reactions[%d] = %q, want %q", i, body.Reactions[i], wantReactions[i])
		}
	}
}

// @sk-test shield-detector-catalog#T4.1: catalog reports loaded packs sorted (AC-002)
func TestCatalogPacks(t *testing.T) {
	reg := packsRegistry(t,
		&compliance.Pack{Key: "PCI DSS", Name: "Payment Card Industry"},
		&compliance.Pack{Key: "HIPAA", Name: "Health Insurance"},
	)
	h := NewCatalogHandler(nil, reg)

	_, body := getCatalog(t, h)
	if len(body.Packs) != 2 {
		t.Fatalf("packs = %v, want 2", body.Packs)
	}
	if body.Packs[0].Key != "HIPAA" || body.Packs[0].Name != "Health Insurance" {
		t.Errorf("packs[0] = %+v", body.Packs[0])
	}
	if body.Packs[1].Key != "PCI DSS" || body.Packs[1].Name != "Payment Card Industry" {
		t.Errorf("packs[1] = %+v", body.Packs[1])
	}
}

// @sk-test shield-detector-catalog#T4.1: declared-but-unregistered types are not advertised (AC-004)
func TestCatalogOmitsUnregisteredType(t *testing.T) {
	// entity.DetectorTypePresidio is declared but no detector is registered for it.
	h := NewCatalogHandler([]entity.DetectorType{entity.DetectorTypeRegex}, nil)

	_, body := getCatalog(t, h)
	for _, d := range body.Detectors {
		if d == string(entity.DetectorTypePresidio) {
			t.Fatalf("catalog advertised unregistered type %q", d)
		}
	}
	if len(body.Detectors) != 1 || body.Detectors[0] != string(entity.DetectorTypeRegex) {
		t.Errorf("detectors = %v, want [regex]", body.Detectors)
	}
}

// @sk-test shield-detector-catalog#T4.1: empty pack set still returns detectors/reactions (AC-005)
func TestCatalogEmptyPacks(t *testing.T) {
	h := NewCatalogHandler([]entity.DetectorType{entity.DetectorTypeRegex}, nil)

	code, body := getCatalog(t, h)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if body.Packs == nil || len(body.Packs) != 0 {
		t.Errorf("packs = %v, want empty list", body.Packs)
	}
	if len(body.Detectors) == 0 || len(body.Reactions) == 0 {
		t.Errorf("expected detectors and reactions with empty packs, got %v / %v", body.Detectors, body.Reactions)
	}
}
