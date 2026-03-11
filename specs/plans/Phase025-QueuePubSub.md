# ✅ Phase 025: Queue Pub/Sub — `pkg/queue` + fullend.yaml 파싱 + gluegen + crosscheck

## 목표

`@publish` 시퀀스와 `@subscribe` 트리거를 fullend가 처리할 수 있도록 한다.

- `pkg/queue` — PostgreSQL + Memory 백엔드 구현
- `fullend.yaml` — `queue` 설정 파싱
- `gluegen` — subscribe 등록 + queue 초기화 + service 변환
- `crosscheck` — publish ↔ subscribe 교차 검증

설계 상세: `files/queue.md`

## 의존성

- SSaC 수정지시서 019 ✅ (`@publish` 파서 + `@subscribe` 파서 + 검증 + 코드젠)
- SSaC 수정지시서 020 ✅ (`@subscribe` 메시지 타입 struct + 코드젠 분리, `@subscribe "topic"` 문법 — 타입은 함수 파라미터에서 추출)
- Phase 022 ✅ (내장 모델 패턴 — session/cache와 동일 구조)

## SSaC 코드젠 현황 (020 완료 기준)

SSaC가 생성하는 코드:

**@publish (HTTP 함수 내부):**
```go
err := queue.Publish(c.Request.Context(), "order.completed", map[string]any{
    "Email":   order.Email,
    "OrderID": order.ID,
})
```

**@publish (subscribe 함수 내부):**
```go
err := queue.Publish(ctx, "order.completed", map[string]any{
    "Email":   order.Email,
    "OrderID": order.ID,
})
```

**@subscribe 함수:**
```go
func OnOrderCompleted(ctx context.Context, message OnOrderCompletedMessage) error {
    // message.OrderID 직접 접근
    // 에러: return fmt.Errorf(...)
    // 성공: return nil
}
```

- gin 의존성 없음, `ctx` + `message T` + `error` 시그니처
- 메시지 struct는 .ssac 파일에 Go struct로 선언됨 (SSaC가 파싱)
- `queue` 패키지를 직접 import하여 `queue.Publish()` 호출

## 1. `pkg/queue` 구현

SSaC가 `queue.Publish()` 패키지 함수로 호출하므로 **싱글턴 패턴** 사용.

### API

```go
package queue

// Init initializes the global queue with the given backend.
func Init(ctx context.Context, backend string, db *sql.DB) error

// Publish sends a message to the given topic.
func Publish(ctx context.Context, topic string, payload any, opts ...PublishOption) error

// Subscribe registers a handler for the given topic.
func Subscribe(topic string, handler func(ctx context.Context, msg []byte) error)

// Start begins processing subscribed messages (blocking, run in goroutine).
func Start(ctx context.Context) error

// Close stops the queue processor.
func Close() error

type PublishOption func(*publishConfig)

func WithDelay(seconds int) PublishOption
func WithPriority(p string) PublishOption
```

### PostgreSQL 백엔드

`fullend_queue` 테이블 자동 생성:

```sql
CREATE TABLE IF NOT EXISTS fullend_queue (
    id           BIGSERIAL PRIMARY KEY,
    topic        TEXT NOT NULL,
    payload      JSONB NOT NULL,
    priority     TEXT NOT NULL DEFAULT 'normal',
    status       TEXT NOT NULL DEFAULT 'pending',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deliver_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_fullend_queue_pending
    ON fullend_queue (topic, status, deliver_at)
    WHERE status = 'pending';
```

- `Publish` — INSERT into fullend_queue (delay → deliver_at 설정)
- `Subscribe` — 핸들러 맵에 등록
- `Start` — polling 루프 (SELECT ... FOR UPDATE SKIP LOCKED → 처리 → status 갱신)
- `Close` — polling 중지

### Memory 백엔드

테스트용. 동기 전달.

- `Publish` — 등록된 핸들러에 직접 전달
- `Subscribe` — 핸들러 맵 등록
- `Start` / `Close` — no-op

### 파일

| 파일 | 내용 |
|---|---|
| `pkg/queue/queue.go` | 싱글턴 API + PublishOption + PostgreSQL + Memory 구현 |
| `pkg/queue/queue_test.go` | Memory 백엔드 단위 테스트 |

## 2. `fullend.yaml` 파싱

`ProjectConfig`에 `Queue` 필드 추가:

```go
type ProjectConfig struct {
    // ... 기존 필드
    Queue *QueueBackend `yaml:"queue"`
}

type QueueBackend struct {
    Backend string `yaml:"backend"` // "postgres" or "memory"
}
```

fullend.yaml 예시:

```yaml
queue:
  backend: postgres
```

### 파일

| 파일 | 변경 |
|---|---|
| `internal/projectconfig/projectconfig.go` | `QueueBackend` 타입 + `Queue` 필드 추가 |

## 3. gluegen 변경

### 3-1. subscribe 함수 감지

`ServiceFunc.Subscribe != nil`인 함수를 수집.

```go
func collectSubscribers(funcs []ssacparser.ServiceFunc) []ssacparser.ServiceFunc
```

### 3-2. service 변환 — subscribe 함수 처리

SSaC 코드젠이 생성한 subscribe 함수:

```go
func OnOrderCompleted(ctx context.Context, message OnOrderCompletedMessage) error {
    order, err := orderModel.FindByID(...)
    ...
    return nil
}
```

gluegen이 변환:

```go
func (s *Server) OnOrderCompleted(ctx context.Context, message OnOrderCompletedMessage) error {
    order, err := s.OrderModel.FindByID(...)
    ...
    return nil
}
```

기존 `transformSource()`와 동일한 패턴 — receiver 추가 + 모델 참조 변환. 단, subscribe 함수는 `(c *gin.Context)` 대신 `(ctx context.Context, message T) error` 시그니처이므로 receiver 삽입 위치만 다름.

### 3-3. subscribe 등록 코드젠 (main.go)

subscribe 함수를 queue에 등록하는 코드 생성. 토픽은 `ServiceFunc.Subscribe.Topic`, 메시지 타입은 `ServiceFunc.Param.TypeName`에서 추출:

```go
// main.go에 생성
queue.Subscribe("order.completed", func(ctx context.Context, msg []byte) error {
    var message service.OnOrderCompletedMessage
    if err := json.Unmarshal(msg, &message); err != nil {
        return fmt.Errorf("unmarshal: %w", err)
    }
    return server.OnOrderCompleted(ctx, message)
})
```

- 메시지 struct는 SSaC가 .ssac 파일에서 파싱 → 코드젠으로 service 패키지에 포함
- `json.Unmarshal` → 타입 변환 → 서버 메서드 호출

### 3-4. main.go 확장 — queue 초기화

```go
import "github.com/.../pkg/queue"

// queue 초기화
if err := queue.Init(ctx, "postgres", conn); err != nil {
    log.Fatalf("queue init: %v", err)
}
defer queue.Close()

// subscribe 등록 (3-3에서 생성)
queue.Subscribe("order.completed", func(ctx context.Context, msg []byte) error { ... })

// queue 시작
go queue.Start(ctx)
```

### 3-5. queue import 경로 변환

SSaC가 `"queue"` import를 생성하므로, gluegen이 fullend `pkg/queue` 경로로 변환:

```go
// SSaC 생성: import "queue"
// gluegen 변환: import "github.com/geul-org/fullend/pkg/queue"
```

기존 `transformSource()`의 import 경로 변환 패턴과 동일.

### 3-6. GlueInput 확장

```go
type GlueInput struct {
    // ... 기존 필드
    QueueBackend string // "postgres", "memory", "" (없으면 queue 미사용)
}
```

### 파일

| 파일 | 변경 |
|---|---|
| `internal/gluegen/gluegen.go` | `GlueInput.QueueBackend` 필드 + subscribe 감지 + import 변환 |
| `internal/gluegen/main_go.go` | queue 초기화 + subscribe 등록 + Start 호출 |

## 4. crosscheck 변경

### publish ↔ subscribe 교차 검증

| Rule | Level | 설명 |
|---|---|---|
| `@publish` topic → `@subscribe` 함수 존재 | WARNING | 발행만 있고 구독 없음 |
| `@subscribe` topic → `@publish` 존재 | WARNING | 구독만 있고 발행 없음 |
| `@subscribe` message struct 필드 → `@publish` payload 필드 매칭 | WARNING | 필드명 불일치 |
| fullend.yaml `queue` 미설정 + `@publish`/`@subscribe` 사용 | ERROR | 설정 누락 |

### 구현

```go
func CheckQueue(funcs []ssacparser.ServiceFunc, queueBackend string) []CrossError {
    publishTopics := map[string]map[string]bool{} // topic → payload field set
    subscribeTopics := map[string]ssacparser.ServiceFunc{} // topic → func

    for _, fn := range funcs {
        if fn.Subscribe != nil {
            subscribeTopics[fn.Subscribe.Topic] = fn
        }
        for _, seq := range fn.Sequences {
            if seq.Type == "publish" {
                fields := map[string]bool{}
                for k := range seq.Inputs {
                    fields[k] = true
                }
                publishTopics[seq.Topic] = fields
            }
        }
    }
    // 검증 로직
}
```

message struct 필드 매칭: `ServiceFunc.Structs`에서 `Param.TypeName`에 해당하는 struct의 필드를 `@publish` payload 필드와 비교.

### 파일

| 파일 | 변경 |
|---|---|
| `internal/crosscheck/queue.go` | 신규 — publish ↔ subscribe 교차 검증 |
| `internal/crosscheck/queue_test.go` | 신규 — 단위 테스트 |
| `internal/crosscheck/crosscheck.go` | `QueueBackend` 필드 + `CheckQueue` 호출 |

## 5. orchestrator 변경

`CrossValidateInput`과 `GlueInput`에 `QueueBackend` 전달.

| 파일 | 변경 |
|---|---|
| `internal/orchestrator/orchestrator.go` | `QueueBackend` 필드 전달 |

## 전체 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `pkg/queue/queue.go` | 신규 — 싱글턴 API + PostgreSQL + Memory |
| `pkg/queue/queue_test.go` | 신규 — Memory 단위 테스트 |
| `internal/projectconfig/projectconfig.go` | `QueueBackend` 타입 + 필드 추가 |
| `internal/crosscheck/crosscheck.go` | `QueueBackend` 필드 + `CheckQueue` 호출 |
| `internal/crosscheck/queue.go` | 신규 — 교차 검증 |
| `internal/crosscheck/queue_test.go` | 신규 — 테스트 |
| `internal/gluegen/gluegen.go` | `QueueBackend` 필드 + subscribe 감지 + import 변환 |
| `internal/gluegen/main_go.go` | queue 초기화 + subscribe 등록 코드 생성 |
| `internal/orchestrator/orchestrator.go` | `QueueBackend` 전달 |

## 검증 방법

```bash
# 1. pkg/queue 단위 테스트
go test ./pkg/queue/...

# 2. crosscheck 단위 테스트
go test ./internal/crosscheck/...

# 3. 전체 빌드
go build ./cmd/fullend/

# 4. dummy 프로젝트 end-to-end (별도)
fullend validate specs/dummy-gigbridge
fullend gen specs/dummy-gigbridge artifacts/dummy-gigbridge
```
