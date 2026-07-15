package postgres

import (
	"encoding/json"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/preprocessor"
)

func marshalPreprocessors(pps []preprocessor.PreprocessorDef) ([]byte, error) {
	if pps == nil {
		return []byte("null"), nil
	}
	return json.Marshal(pps)
}

func unmarshalPreprocessors(data []byte) ([]preprocessor.PreprocessorDef, error) {
	if data == nil {
		return nil, nil
	}
	var pps []preprocessor.PreprocessorDef
	if err := json.Unmarshal(data, &pps); err != nil {
		return nil, err
	}
	return pps, nil
}
