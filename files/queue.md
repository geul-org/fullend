# Queue Pub/Sub 설계

## 개요

fullend에서 비동기 이벤트 발행/구독을 선언적으로 처리한다.

- **@publish** — 시퀀스 타입 (함수 내부에서 이벤트 발행)
- **@subscribe** — 트리거 (이벤트 수신 시 함수 실행)

## fullend.yaml

```yaml
queue:
  backend: postgres   # postgres | redis | kafka
```

프로젝트당 하나의 큐 백엔드. PostgreSQL LISTEN/NOTIFY로 시작, 규모 커지면 backend만 교체.

## @publish — 시퀀스 타입 (11번째)

서비스 함수 내부에서 이벤트를 발행한다.

### 문법

```
@publish "topic" {payload} [{options}]
```

- `"topic"` — 이벤트 토픽명 (문자열 리터럴)
- `{payload}` — 메시지 페이로드 (필수, 기존 args 형식)
- `{options}` — 전송 옵션 (선택)

### 예시

```go
// 기본
// @publish "order.completed" {OrderID: order.ID, Email: user.Email}

// 지연 발행
// @publish "cart.abandoned" {CartID: cart.ID} {delay: 1800}

// 우선순위
// @publish "payment.failed" {PaymentID: payment.ID} {priority: "high"}
```

### 옵션

| 옵션 | 타입 | 설명 |
|---|---|---|
| `delay` | int (초) | 지연 발행 |
| `priority` | string | 우선순위 ("high", "normal", "low") |

## @subscribe — 트리거 (함수 레벨)

큐 이벤트를 구독하여 서비스 함수를 실행한다.

### 문법

```
@subscribe "topic"
```

함수 본문 앞에 선언. 내부 시퀀스는 기존과 동일.

### 예시

```go
package service

// @subscribe "order.completed"
// @get Order order = Order.FindByID({ID: message.OrderID})
// @call notification.SendEmail({To: order.Email, Subject: "주문 완료"})
// @put Order.UpdateNotified({ID: order.ID, Notified: "true"})
func OnOrderCompleted() {}
```

### 입력 변수

HTTP 트리거에서 `request`를 쓰듯, 구독 트리거에서는 `message`를 쓴다.

| 트리거 | 입력 변수 | 출처 |
|---|---|---|
| HTTP | `request` | HTTP body/params |
| Queue | `message` | 큐 메시지 페이로드 |

`message.OrderID`, `message.Email` 형태로 페이로드 필드에 접근.

### @response 없음

구독 함수는 비동기 처리이므로 `@response`가 없다. 있으면 ERROR.

## SSaC 변경 요약

| 구분 | 기존 | 추가 |
|---|---|---|
| 트리거 | HTTP (`func Name()`) | Queue (`@subscribe "topic"`) |
| 시퀀스 | 10종 | +1 `@publish` (11종) |
| 입력 변수 | `request`, `currentUser`, `config`, `query` | +1 `message` |

## fullend 코드젠

### 발행 (ssac-gen)

```go
// @publish "order.completed" {OrderID: order.ID, Email: user.Email}
// ↓ 생성
queue.Publish(ctx, "order.completed", map[string]any{
    "OrderID": order.ID,
    "Email":   user.Email,
})
```

### 구독 등록 (main.go)

```go
// @subscribe "order.completed" → OnOrderCompleted
// ↓ main.go에 생성
queue.Subscribe("order.completed", func(ctx context.Context, msg []byte) error {
    var message OnOrderCompletedMessage
    json.Unmarshal(msg, &message)
    return handler.OnOrderCompleted(ctx, message)
})
```

### 메시지 타입 자동 생성

`@subscribe` 함수의 내부 시퀀스에서 `message.X`로 접근하는 필드를 수집하여 메시지 struct를 자동 생성한다.

```go
// 자동 생성
type OnOrderCompletedMessage struct {
    OrderID int64  `json:"OrderID"`
    Email   string `json:"Email"`
}
```

필드 타입은 해당 필드를 사용하는 시퀀스에서 역추적 (DDL 컬럼 타입 등).

## 교차 검증

| Rule | Level |
|---|---|
| @publish topic → @subscribe 함수 존재 | WARNING |
| @subscribe topic → @publish 존재 | WARNING |
| @subscribe 함수에 @response 있음 | ERROR |
| @subscribe 함수의 message 필드 → @publish payload 필드 매칭 | WARNING |
| fullend.yaml queue.backend 미설정 + @publish/@subscribe 사용 | ERROR |

## pkg/queue 구현

```go
type QueueModel interface {
    Publish(ctx context.Context, topic string, payload any, opts ...PublishOption) error
    Subscribe(topic string, handler func(ctx context.Context, msg []byte) error) error
    Close() error
}

type PublishOption func(*publishConfig)

func WithDelay(seconds int) PublishOption { ... }
func WithPriority(p string) PublishOption { ... }
```

### 백엔드

| 백엔드 | 구현 |
|---|---|
| `postgres` | LISTEN/NOTIFY + fullend_queue 테이블 (지연/우선순위 지원) |
| `redis` | Redis Pub/Sub + Sorted Set (지연 지원) |
| `kafka` | Kafka Producer/Consumer |

PostgreSQL이 기본. 별도 인프라 없이 DB 하나로 시작.

## 실행 순서

| 순서 | 작업 | 위치 |
|---|---|---|
| 1 | `pkg/queue` 구현 (PostgreSQL + Memory) | fullend |
| 2 | fullend.yaml `queue` 설정 파싱 | fullend |
| 3 | SSaC `@publish` 파서 + 검증 | SSaC (수정지시서) |
| 4 | SSaC `@subscribe` 파서 + 검증 | SSaC (수정지시서) |
| 5 | SSaC 코드젠 — publish 호출 생성 | SSaC (수정지시서) |
| 6 | fullend gen — subscribe 등록 + 메시지 타입 생성 | fullend |
| 7 | fullend crosscheck — publish ↔ subscribe 교차 검증 | fullend |
