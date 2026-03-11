# ✅ Phase 024: `fullend gen-model` — 외부 OpenAPI → Go 모델 생성

## 목표

외부 서비스의 OpenAPI 문서로부터 Go interface + 타입 + HTTP client 구현체를 한 번에 생성한다. 생성된 파일은 SSaC에서 패키지 접두사 `@model`로 사용하며, 런타임에 실제 HTTP 호출을 수행한다.

```bash
fullend gen-model <openapi-source> <output-dir>
```

`<openapi-source>`는 로컬 파일 경로 또는 URL(`http://`, `https://`)을 모두 지원한다. URL인 경우 다운로드 후 파싱한다.

## 용법

```bash
# 로컬 파일
fullend gen-model specs/my-project/external/escrow.openapi.yaml specs/my-project/external/

# URL
fullend gen-model https://api.stripe.com/openapi.yaml specs/my-project/external/

# → specs/my-project/external/escrow.go (또는 stripe.go)
```

SSaC에서 사용:
```go
import "github.com/org/project/internal/external"

// @post external.EscrowHoldResponse result = external.Escrow.Hold({GigID: gig.ID, Amount: gig.Budget})
```

## 입력

```
specs/<project>/external/
├── escrow.openapi.yaml       ← 외부 결제 서비스 OpenAPI 3.x
└── notification.openapi.yaml ← 외부 알림 서비스 OpenAPI 3.x
```

## 출력

하나의 `.go` 파일에 interface + 타입 + HTTP client 구현체를 모두 포함한다.

```go
// escrow.go (자동 생성)
package external

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
)

// --- Interface ---

type EscrowModel interface {
    Hold(ctx context.Context, gigID int64, amount int64) (*EscrowHoldResponse, error)
    Release(ctx context.Context, gigID int64, freelancerID int64) (*EscrowReleaseResponse, error)
}

// --- Types ---

type EscrowHoldResponse struct {
    TransactionID int64  `json:"transactionId"`
    Status        string `json:"status"`
}

type EscrowReleaseResponse struct {
    TransactionID int64  `json:"transactionId"`
    Amount        int64  `json:"amount"`
}

// --- HTTP Client Implementation ---

type escrowClient struct {
    baseURL string
    client  *http.Client
}

func NewEscrowModel(baseURL string) EscrowModel {
    return &escrowClient{baseURL: baseURL, client: &http.Client{}}
}

func (c *escrowClient) Hold(ctx context.Context, gigID int64, amount int64) (*EscrowHoldResponse, error) {
    body := map[string]any{"gigId": gigID, "amount": amount}
    var resp EscrowHoldResponse
    if err := c.do(ctx, "POST", "/escrow/hold", body, &resp); err != nil {
        return nil, err
    }
    return &resp, nil
}

func (c *escrowClient) Release(ctx context.Context, gigID int64, freelancerID int64) (*EscrowReleaseResponse, error) {
    body := map[string]any{"gigId": gigID, "freelancerId": freelancerID}
    var resp EscrowReleaseResponse
    if err := c.do(ctx, "POST", "/escrow/release", body, &resp); err != nil {
        return nil, err
    }
    return &resp, nil
}

func (c *escrowClient) do(ctx context.Context, method, path string, body any, result any) error {
    var buf bytes.Buffer
    if body != nil {
        if err := json.NewEncoder(&buf).Encode(body); err != nil {
            return fmt.Errorf("encode request: %w", err)
        }
    }
    req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, &buf)
    if err != nil {
        return fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")
    resp, err := c.client.Do(req)
    if err != nil {
        return fmt.Errorf("http %s %s: %w", method, path, err)
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        return fmt.Errorf("http %s %s: status %d", method, path, resp.StatusCode)
    }
    if result != nil {
        if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
            return fmt.Errorf("decode response: %w", err)
        }
    }
    return nil
}
```

## OpenAPI → 생성 규칙

### 이름 결정

| OpenAPI 요소 | 생성 대상 |
|---|---|
| 파일명 (`escrow.openapi.yaml`) | 출력 파일명 (`escrow.go`), 패키지 접두사 (`external`) |
| `info.title` 또는 파일명 stem | interface 이름 (`EscrowModel`) |
| `operationId` | 메서드명 |
| `servers[0].url` | `NewXxxModel(baseURL)` 기본값 (생성 코드의 주석에 기록) |

### 메서드 시그니처 결정

| OpenAPI 요소 | Go 메서드 구성 |
|---|---|
| request body schema properties | 파라미터 (이름 + 타입) |
| path parameters | 파라미터에 추가 |
| response 200 schema | 리턴 타입 (`*ResponseType, error`) |
| response 200 없음 | `error` only |

`context.Context`는 항상 첫 번째 파라미터로 추가.

### 타입 매핑

| OpenAPI type/format | Go type |
|---|---|
| `integer` / `int64` | `int64` |
| `integer` / `int32` | `int32` |
| `integer` / (none) | `int` |
| `number` / `float` | `float32` |
| `number` / `double` or (none) | `float64` |
| `string` | `string` |
| `string` / `date-time` | `time.Time` |
| `boolean` | `bool` |
| `array` | `[]ElementType` |
| `object` (named `$ref`) | `*RefType` |

### HTTP client 메서드 결정

| OpenAPI method + path | client.do() 호출 |
|---|---|
| `POST /escrow/hold` | `c.do(ctx, "POST", "/escrow/hold", body, &resp)` |
| `GET /escrow/{id}` | `c.do(ctx, "GET", fmt.Sprintf("/escrow/%d", id), nil, &resp)` |
| `DELETE /escrow/{id}` | `c.do(ctx, "DELETE", fmt.Sprintf("/escrow/%d", id), nil, nil)` |

## SSaC 연동

생성된 `*_model.go`는 `specs/<project>/external/`에 위치한다.

- SSaC validate가 Go interface를 파싱하여 파라미터 매칭 검증 (기존 패키지 접두사 모델과 동일)
- `fullend gen` 시 `artifacts/<project>/backend/internal/external/`로 복사
- `main.go`에서 `NewXxxModel(os.Getenv("ESCROW_BASE_URL"))` 식으로 DI

## fullend gen 연동

`fullend gen`이 `specs/<project>/external/*.go` 파일을 감지하면:

1. `artifacts/<project>/backend/internal/external/`로 복사
2. `main.go`에 모델 초기화 코드 생성
3. 서비스 핸들러에 모델 주입

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `cmd/fullend/main.go` | `gen-model` 서브커맨드 추가 |
| `internal/genmodel/genmodel.go` | 신규 — OpenAPI → interface + types + HTTP client 코드젠 |
| `internal/genmodel/genmodel_test.go` | 신규 — 단위 테스트 |
| `internal/genmodel/testdata/escrow.openapi.yaml` | 신규 — 테스트용 OpenAPI 문서 |
| `internal/gluegen/gluegen.go` | `external/*.go` 복사 단계 추가 |
| `internal/gluegen/main_go.go` | external 모델 초기화 코드 생성 |

## 의존성

- Phase 022 ✅ (내장 모델 패턴 확립)
- Phase 023 ✅ (.ssac 확장자)
- SSaC 수정지시서 017 ✅ (파라미터 매칭 검증)

## 검증 방법

```bash
# 1. 단위 테스트
go test ./internal/genmodel/...

# 2. gen-model → gofmt 통과
fullend gen-model internal/genmodel/testdata/escrow.openapi.yaml /tmp/out/
gofmt -e /tmp/out/escrow.go

# 3. dummy 프로젝트 end-to-end
fullend gen-model specs/dummy-gigbridge/external/escrow.openapi.yaml specs/dummy-gigbridge/external/
fullend validate specs/dummy-gigbridge
fullend gen specs/dummy-gigbridge artifacts/dummy-gigbridge
cd artifacts/dummy-gigbridge/backend && go build ./cmd/
```
