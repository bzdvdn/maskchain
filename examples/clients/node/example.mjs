// Minimal MaskChain caller with the built-in fetch (Node 18+).
//
//   node example.mjs
//
// Env: MASKCHAIN_URL (default http://localhost:8080/api/v1)
//      MASKCHAIN_KEY (default sk-test-default)
//      MASKCHAIN_MODEL (default llama3.2)

const baseUrl = process.env.MASKCHAIN_URL ?? "http://localhost:8080/api/v1";
const apiKey = process.env.MASKCHAIN_KEY ?? "sk-test-default";
const model = process.env.MASKCHAIN_MODEL ?? "llama3.2";

const res = await fetch(`${baseUrl}/chat/completions`, {
  method: "POST",
  headers: {
    Authorization: `Bearer ${apiKey}`,
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    model,
    messages: [
      {
        role: "user",
        content: "Summarize this note: contact Alice at alice@example.com or +1-555-0100.",
      },
    ],
  }),
});

if (!res.ok) {
  console.error(`request failed: ${res.status} ${await res.text()}`);
  process.exit(1);
}

const body = await res.json();
console.log(body.choices[0].message.content);
