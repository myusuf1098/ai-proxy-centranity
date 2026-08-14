# ProxyGateway Enterprise — End-to-End User & Operator Guide

## 1. Overview
ProxyGateway Enterprise acts as a unified AI reverse proxy sitting between your applications and upstream LLM providers (including containerized 9Router). It provides access control, model aliases, rate limiting, budget enforcement, outbound proxy routing, observability, and terminal administration.

---

## 2. Setting Up Client Applications

### 2.1 Python (OpenAI SDK)
```python
from openai import OpenAI

client = OpenAI(
    base_url="http://127.0.0.1:8088/v1",
    api_key="sk-pg-your-client-api-key"
)

response = client.chat.completions.create(
    model="coding",  # Automatically resolves to cc-sonnet or healthy fallback
    messages=[{"role": "user", "content": "How do I implement a LRU cache in Python?"}],
    stream=False
)
print(response.choices[0].message.content)
```

### 2.2 Node.js / TypeScript
```typescript
import OpenAI from "openai";

const openai = new OpenAI({
  baseURL: "http://127.0.0.1:8088/v1",
  apiKey: "sk-pg-your-client-api-key",
});

async function main() {
  const stream = await openai.chat.completions.create({
    model: "fast", // Resolves to cc-haiku or gemini-flash
    messages: [{ role: "user", content: "Count from 1 to 5." }],
    stream: true,
  });

  for await (const chunk of stream) {
    process.stdout.write(chunk.choices[0]?.delta?.content || "");
  }
}
main();
```

---

## 3. Policy & Model Governance

### 3.1 Hierarchical Precedence
When a client requests a model (e.g. `cc-opus`), ProxyGateway evaluates policies in strict order:
1. **Global Denylist**: If model matches global deny patterns, request is immediately rejected (HTTP 403 `MODEL_DENIED`).
2. **API Key Denylist**: If model matches key-specific `denied_models`, request is rejected (HTTP 403 `MODEL_NOT_ALLOWED`).
3. **API Key Allowlist**: If `allowed_models` is defined, model must be present; otherwise rejected.
4. **Permitted**: If no rules block the request, routing proceeds.

---

## 4. Model Aliases & Circuit Breakers

| Alias | Target Models (Priority Ordered) | Primary Use Case |
| :--- | :--- | :--- |
| `coding` | `1. cc-sonnet`, `2. cc-haiku` | Software engineering, code generation & refactoring |
| `fast` | `1. cc-haiku`, `2. gemini-flash` | Ultra-low latency chat & triage |
| `reasoning` | `1. cc-opus`, `2. cc-sonnet` | Complex multi-step reasoning |
| `cheap` / `free` | `1. cc-haiku` | High-volume cost-sensitive tasks |

If `cc-sonnet` experiences upstream 5xx errors or network timeouts, the Circuit Breaker marks it `OPEN` and automatically routes the request to `cc-haiku` without client intervention.

---

## 5. TUI Dashboard Operations
Launch the terminal dashboard anytime to monitor operational status:
```bash
./bin/proxygateway-tui
```
- Press `1` for Overview
- Press `3` to inspect discovered models
- Press `5` to view API key quotas and rate limits
- Press `7` to check active alias routes and circuit states
- Press `r` to refresh metrics
- Press `q` to quit
