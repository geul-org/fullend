# Phase 023: External 모델 (OpenAPI 기반 코드젠)

## 배경

Phase 022에서 Session, Cache, File 내장 모델은 고정 interface + 런타임 패키지.
External은 성격이 다름 — 외부 서비스 OpenAPI 문서로부터 서비스별 interface + HTTP client를 생성하는 **코드젠 도구**.

## 설계

### 1. OpenAPI 배치

```
specs/<project>/
├── api/openapi.yaml              ← 우리 API (기존)
├── external/
│   ├── escrow.openapi.yaml       ← 외부 결제 서비스 OpenAPI
│   └── notification.openapi.yaml ← 외부 알림 서비스 OpenAPI
```

### 2. OpenAPI에서 추출하는 정보

- **operationId** → 모델 메서드명 매핑 (e.g. `createEscrowHold` → `Escrow.CreateHold`)
- **request schema** → 메서드 파라미터 타입
- **response schema** → 메서드 리턴 타입
- **서버 URL** → base URL (환경변수로 오버라이드 가능)
- **security scheme** → API key, Bearer token 등 인증 방식

### 3. 코드젠 결과 (서비스별로 다른 interface)

```go
// artifacts/<project>/backend/internal/external/escrow_model.go (자동 생성)
type EscrowModel interface {
    Hold(req EscrowHoldRequest) (*EscrowHoldResponse, error)
    Release(req EscrowReleaseRequest) (*EscrowReleaseResponse, error)
}
type escrowModelImpl struct {
    baseURL string
    client  *http.Client
}
```

### 4. SSaC에서의 사용

```go
// @post HoldResponse result = escrow.Escrow.Hold({GigID: gig.ID, Amount: gig.Budget})
```

### 5. 공식 외부 서비스 패키지

- 메이저 서비스(Stripe, Google, Slack, Twilio, SendGrid, Telegram 등)는 공개 OpenAPI를 활용하여 fullend가 공식 패키지로 제공
- 별도 레지스트리(`fullend-ext/`)로 분리하여 코어와 독립 버전 관리
- `fullend ext-gen <openapi.yaml>` 도구로 사용자도 직접 패키지 생성 가능

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `internal/gluegen/external_model.go` | 신규 — 외부 OpenAPI → interface + HTTP client 코드젠 |
| `internal/orchestrator/validate.go` | 외부 스펙 디렉토리 스캔 |

## 의존성

- Phase 022 완료 (내장 모델 패턴 확립)

## 미결 사항

1. **인증 정보 주입** — API key, OAuth 등 인증 정보 어디서 주입?
2. **타임아웃, 재시도 정책** — 정의 위치 (fullend.yaml? 코드젠 옵션?)
3. **ext 레지스트리** — `fullend-ext/` 별도 조직으로 분리 시점

## 검증 방법

```bash
go test ./internal/gluegen/...
fullend validate specs/dummy-gigbridge  # 외부 스펙 파싱 + 교차 검증
fullend gen specs/dummy-gigbridge artifacts/dummy-gigbridge
cd artifacts/dummy-gigbridge/backend && go build ./cmd/
```
