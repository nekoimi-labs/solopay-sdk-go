package signing

import (
	"encoding/json"
	"os"
	"testing"
)

type goldenVector struct {
	Name               string `json:"name"`
	Method             string `json:"method"`
	Path               string `json:"path"`
	Timestamp          int64  `json:"timestamp"`
	Nonce              string `json:"nonce"`
	Body               string `json:"body"`
	Secret             string `json:"secret"`
	ExpectedBodySHA256 string `json:"expected_body_sha256"`
	ExpectedSignature  string `json:"expected_signature"`
}

func TestGoldenVectors(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/golden_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []goldenVector
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, vector := range vectors {
		t.Run(vector.Name, func(t *testing.T) {
			body := []byte(vector.Body)
			if got := BodySHA256Hex(body); got != vector.ExpectedBodySHA256 {
				t.Fatalf("body hash = %s, want %s", got, vector.ExpectedBodySHA256)
			}
			canonical := Canonical(vector.Method, vector.Path, vector.Timestamp, vector.Nonce, body)
			if got := SignHex([]byte(vector.Secret), canonical); got != vector.ExpectedSignature {
				t.Fatalf("signature = %s, want %s", got, vector.ExpectedSignature)
			}
		})
	}
}
