package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	provider "github.com/bzdvdn/maskchain/src/internal/adapters/provider"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <MISTRAL_API_KEY>\n", os.Args[0])
		os.Exit(1)
	}
	apiKey := os.Args[1]

	body := map[string]interface{}{
		"model": "mistral-large-latest",
		"messages": []map[string]string{
			{
				"role": "user",
				"content": `I have an employee database dump in CSV format below. Please analyze it and give me a clear answer with a table.

` + "```" + `
ID,FullName,Email,Phone,SSN,Position,Department,Project,Salary
EMP-001,James LastName1,James.lastname1@example.com,+1-555-123-4567,987-65-4321,Software Engineer,Engineering #1,Project-42,125000
EMP-002,John LastName2,John.lastname2@example.com,+1-555-234-5678,987-65-4322,Senior Software Engineer,Engineering #1,Project-42,135000
EMP-003,Robert LastName3,Robert.lastname3@example.com,+1-555-345-6789,987-65-4323,Lead Engineer,Engineering #1,Project-Omega,150000
EMP-004,Michelle LastName4,Michelle.lastname4@example.com,+1-555-456-7890,987-65-4324,Senior Software Engineer,Engineering #2,Project-42,130000
EMP-005,William LastName5,William.lastname5@example.com,+1-555-567-8901,987-65-4325,Software Engineer,Engineering #2,Project-42,120000
EMP-006,Emily LastName6,Emily.lastname6@example.com,+1-555-678-9012,987-65-4326,DevOps Engineer,Infrastructure,Project-Omega,140000
EMP-007,Michael LastName7,Michael.lastname7@example.com,+1-555-789-0123,987-65-4327,Senior DevOps Engineer,Infrastructure,Project-Omega,155000
EMP-008,Sarah LastName8,Sarah.lastname8@example.com,+1-555-890-1234,987-65-4328,Product Manager,Product,Project-42,145000
EMP-009,David LastName9,David.lastname9@example.com,+1-555-901-2345,987-65-4329,Senior Product Manager,Product,Project-Omega,160000
EMP-010,Laura LastName10,Laura.lastname10@example.com,+1-555-012-3456,987-65-4330,Director of Engineering,Engineering #1,Project-Omega,175000
EMP-011,James LastName11,James.lastname11@example.com,+1-555-123-4567,987-65-4331,Software Engineer,Engineering #1,Project-42,125000
EMP-012,John LastName12,John.lastname12@example.com,+1-555-234-5678,987-65-4332,Senior Software Engineer,Engineering #1,Project-42,135000
EMP-013,Robert LastName13,Robert.lastname13@example.com,+1-555-345-6789,987-65-4333,Lead Engineer,Engineering #1,Project-Omega,150000
EMP-014,Michelle LastName14,Michelle.lastname14@example.com,+1-555-456-7890,987-65-4334,Senior Software Engineer,Engineering #2,Project-42,130000
EMP-015,William LastName15,William.lastname15@example.com,+1-555-567-8901,987-65-4335,Software Engineer,Engineering #2,Project-42,120000
EMP-016,Emily LastName16,Emily.lastname16@example.com,+1-555-678-9012,987-65-4336,DevOps Engineer,Infrastructure,Project-Omega,140000
EMP-017,Michael LastName17,Michael.lastname17@example.com,+1-555-789-0123,987-65-4337,Senior DevOps Engineer,Infrastructure,Project-Omega,155000
EMP-018,Sarah LastName18,Sarah.lastname18@example.com,+1-555-890-1234,987-65-4338,Product Manager,Product,Project-42,145000
EMP-019,David LastName19,David.lastname19@example.com,+1-555-901-2345,987-65-4339,Senior Product Manager,Product,Project-Omega,160000
EMP-020,Laura LastName20,Laura.lastname20@example.com,+1-555-012-3456,987-65-4330,Director of Engineering,Engineering #1,Project-Omega,175000
` + "```" + `

Please answer:
1. Who is the highest paid employee?`,
			},
		},
	}
	bodyJSON, _ := json.Marshal(body)

	pcfg := &config.ProviderConfig{
		Name:    "mistral",
		APIType: "openai",
		BaseURL: "https://api.mistral.ai",
		APIKeys: []string{apiKey},
		Timeout: "60s",
	}

	egressCfg := &config.EgressConfig{
		MaxIdleConns:        10,
		IdleTimeout:         30 * time.Second,
		MaxIdleConnsPerHost: 2,
	}

	client, err := provider.NewProviderClient(pcfg, egressCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating client: %v\n", err)
		os.Exit(1)
	}

	req := &ports.ProviderRequest{
		Method:  "POST",
		URL:     "/v1/chat/completions",
		Body:    bodyJSON,
		Headers: map[string]string{},
		Path:    "/api/v1/chat/completions",
	}

	fmt.Fprintf(os.Stderr, "\n=== SENDING TO MISTRAL ===\nBody:\n%s\n=== END BODY ===\n\n", string(bodyJSON))

	start := time.Now()
	resp, err := client.Call(context.Background(), req)
	elapsed := time.Since(start)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "=== RESPONSE (took %v) ===\nStatus: %d\nHeaders: %v\nBody: %s\n", elapsed, resp.StatusCode, resp.Headers, string(resp.Body))
}
