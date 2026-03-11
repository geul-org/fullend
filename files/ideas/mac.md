# MaC (Model as Code) 검토

## 배경

fullend의 모델 계층이 6종 I/O를 커버해야 하므로 규모가 커짐.
SSaC(서비스), STML(UI)처럼 모델도 독립 도구로 분리하는 것이 자연스러움.

## 현재 구조

```
SSaC   = Service as Code   → 서비스 흐름 파서 + 핸들러 코드젠
STML   = UI 마크업          → UI 파서 + React 코드젠
fullend                     → 모델 코드젠 + 교차 검증 + 오케스트레이션 + 인프라
```

## MaC 분리 시 구조

```
SSaC   = Service as Code   → 서비스 흐름
STML   = UI 마크업          → 프론트엔드
MaC    = Model as Code     → 모델 (6종 I/O)
fullend                     → 교차 검증 + 오케스트레이션 + 인프라
```

## 모델 I/O 분류 (6종)

| # | 분류 | 목적 | 예시 | SSOT |
|---|---|---|---|---|
| 1 | **DB** | 영속 데이터 | PostgreSQL, MySQL, MongoDB | DDL SQL |
| 2 | **Session** | 사용자 귀속 상태 (로그인, 장바구니 등) | Redis, DB | 미정 |
| 3 | **Cache** | 빈번한 요청 효율화 (TTL 기반 임시 저장) | Redis, Memcached | 미정 |
| 4 | **File** | 파일/오브젝트 스토리지 | S3, GCS, 로컬 파일시스템 | 미정 |
| 5 | **External** | 외부 서비스 API | Stripe, Twilio, SendGrid | 외부 OpenAPI |
| 6 | **Queue** | 비동기 메시지 | Kafka, RabbitMQ, SQS | 미정 |

- Session과 Cache는 기술적으로 같은 인프라(Redis 등)를 쓸 수 있지만, 목적이 다르므로 모델 계층에서 분리.
- External은 외부 서비스가 공개하는 OpenAPI 문서를 그대로 SSOT로 사용.

## @model vs @call 역할 구분 (확정)

- **@model** = 모든 I/O 추상화. I/O가 필요하면 반드시 @model을 통해 접근.
- **@call func** = 순수 비즈니스 로직. I/O 금지. 계산, 판단, 변환만 허용.
- 서비스 시퀀스가 @call(계산) → @model(I/O) 순서를 조율.

## SSaC에서의 사용 (전부 @model)

```go
// DB
// @get User user = User.FindByID({ID: request.ID})

// External API
// @post EscrowResult result = Escrow.Hold({Amount: gig.Budget})

// File
// @post FileResult file = FileStore.Upload({Key: key, Body: request.File})

// Session
// @get Session session = Session.Get({Token: request.Token})

// Cache
// @get CachedGig gig = GigCache.Get({ID: request.ID})

// Queue
// @post QueueResult result = NotificationQueue.Publish({UserID: user.ID, Message: msg})
```

SSaC는 모델이 어떤 I/O 종류인지 모름 — MaC(또는 fullend)가 판단.

## MaC의 역할

1. **모델 DSL 파싱** — 각 I/O 종류별 스펙 파싱 (DDL, 외부 OpenAPI 등)
2. **인터페이스 생성** — Go interface 자동 생성
3. **구현체 코드젠** — I/O 종류별 구현체 자동 생성 (`*sql.DB`, `*http.Client`, `*redis.Client` 등)
4. **문법 검증** — 스펙 내부 정합성 검증

fullend는 MaC를 Go 모듈로 import하여 사용 (SSaC, STML과 동일 패턴).

## 미결 사항

- MaC 독립 프로젝트로 분리할 시점 (지금 vs 모델 종류가 늘어난 후)
- Session, Cache, File, Queue 각각의 SSOT 형식
- DB 모델 코드젠 (현재 fullend model_impl.go)을 MaC로 이관하는 마이그레이션 전략
