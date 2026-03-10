# Phase 018: smoke test 상태 전이 순서 정렬

## 배경

hurl-gen이 생성하는 smoke.hurl은 endpoint를 OpenAPI 정의 순서대로 배치한다. 상태 전이(`@state`)가 필요한 endpoint는 선행 전이가 완료되어야 200을 받을 수 있는데, 현재는 순서가 보장되지 않아 409(상태 전이 불가) 에러가 발생한다.

## 목표

smoke test endpoint 순서를 stateDiagram 전이 흐름에 맞게 자동 정렬한다.

## 정렬 규칙

1. **Auth**: Register → Login (토큰 획득)
2. **Create**: `@state` 없는 POST endpoint (리소스 생성, ID 캡처)
3. **State transitions**: stateDiagram 전이 순서대로 `@state` 있는 endpoint 배치
   - 각 stateDiagram의 `[*] → s1 → s2 → ...` 경로를 추출
   - 해당 전이를 트리거하는 endpoint를 순서대로 배치
4. **Read**: GET endpoint (ListXxx, GetXxx) 마지막

## 필요 데이터

모두 기존 SSOT에서 획득 가능:

- **stateDiagram**: 전이 순서 (`[*] → draft → open → in_progress → ...`)
- **SSaC `@state`**: endpoint별 전이 이벤트명 (예: `ApproveWork`)
- **OpenAPI `operationId`**: endpoint 식별

## 변경 파일

### `internal/gluegen/hurl.go`

- `orderStepsForSmoke()` 함수 추가: stateDiagram 전이 순서 기반으로 step 정렬
- `writeStep()` 호출 전에 정렬 적용

### 입력

- `[]scenarioStep` — 현재 생성된 step 목록
- `[]*statemachine.Diagram` — stateDiagram 파서 결과
- SSaC 파싱 결과에서 각 함수의 `@state` 이벤트명

## 의존성

- `internal/statemachine` — stateDiagram 파서 (기존)
- SSaC 파싱 결과 — `@state` 이벤트명 (기존)

## 검증 방법

```bash
fullend gen specs/dummy-gigbridge artifacts/dummy-gigbridge
```

생성된 smoke.hurl의 endpoint 순서가 비즈니스 흐름과 일치하는지 확인:
1. Register → Login
2. CreateGig
3. PublishGig (draft → open)
4. SubmitProposal (open gig 필요)
5. AcceptProposal (pending → accepted, open → in_progress)
6. SubmitWork (in_progress → under_review)
7. ApproveWork (under_review → completed)
8. ListGigs, GetGig

`DISABLE_AUTHZ=1`로 서버 시작 후 `hurl --test smoke.hurl` 전체 통과.

## 상태: 미착수
