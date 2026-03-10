# Fullend — Full-stack SSOT Orchestrator

> Frontend부터 Backend까지, 5개 SSOT의 정합성을 한 번에 검증하고 코드를 산출한다.

## 왜 필요한가

AI 에이전트에게 "예약 시스템 만들어"라고 하면 코드를 만든다. 열 번째 기능을 추가할 때 에이전트는 앞의 아홉 번을 기억하지 못한다. 구현 코드 10만 줄을 컨텍스트에 넣을 수 없기 때문이다.

코드에는 사용자의 결정(what)과 프레임워크 배선(how)이 섞여 있다. 결정만 분리하면 10만 줄이 12,500줄로 줄어든다. AI가 한 번에 읽을 수 있는 크기다.

Fullend는 이 12,500줄을 구성하는 5개 SSOT를 하나의 CLI로 검증하고 코드를 산출한다.

## 5개 SSOT

| SSOT | 포맷 | 선언 내용 | 도구 |
|---|---|---|---|
| 화면 | STML (HTML5 + data-*) | 뭘 보여주고 뭘 하는가 | stml |
| API 계약 | OpenAPI 3.x | 어떤 요청을 받고 어떤 응답을 주는가 | openapi-generator |
| 서비스 흐름 | SSaC (Go comment DSL) | 어떤 순서로 처리하는가 | ssac |
| 데이터 구조 | SQL DDL | 뭘 저장하는가 | sqlc |
| 인프라 | Terraform HCL | 어디서 돌리는가 | terraform |

Fullend가 만드는 건 마지막 교차 검증과 오케스트레이션뿐이다. 각 도구를 재발명하지 않는다.

## 설치

```bash
go install github.com/geul-org/fullend@latest
```

## CLI

### fullend validate

5개 SSOT의 개별 검증 + 교차 정합성 검증을 한 번에 수행한다.

```bash
fullend validate specs/
```

내부 실행 순서:

```
1. sqlc compile                    ← DDL + 쿼리 검증
2. openapi-generator validate      ← OpenAPI 스키마 검증
3. ssac validate                   ← 서비스 흐름 내부 검증 + OpenAPI/DDL 교차
4. stml validate                   ← UI 선언 내부 검증 + OpenAPI 교차
5. fullend cross-validate          ← 5개 SSOT 간 전체 교차 체크
```

모든 단계가 통과하면:

```
✓ DDL          12 tables, 47 columns
✓ OpenAPI      34 endpoints, 89 fields
✓ SSaC         34 service functions, 127 sequences
✓ STML         18 pages, 43 bindings
✓ Cross        0 mismatches

All 5 SSOT sources are consistent.
```

하나라도 실패하면:

```
✓ DDL          12 tables, 47 columns
✓ OpenAPI      34 endpoints, 89 fields
✗ SSaC         reservation-page.go:CancelReservation
               @model Reservation.SoftDelete — method not found in sqlc queries
✗ STML         reservation-page.html
               data-bind="memo" — field not in OpenAPI response, not in custom.ts
✗ Cross        2 mismatches

FAILED: Fix SSaC and STML errors before codegen.
```

### fullend gen

검증 통과 후 전체 코드를 산출한다.

```bash
fullend gen specs/ artifacts/
```

내부 실행 순서:

```
1. fullend validate specs/         ← 먼저 검증
2. sqlc generate                   ← DB 모델 Go struct 산출
3. openapi-generator generate      ← API 핸들러/클라이언트 산출
4. ssac gen                        ← 서비스 함수 + Model interface 산출
5. stml gen                        ← React 컴포넌트 산출
6. terraform fmt                   ← HCL 포맷팅
```

### fullend status

현재 프로젝트의 SSOT 현황을 요약한다.

```bash
fullend status specs/
```

```
SSOT Status:
  DDL        specs/db/schema.sql              12 tables
  OpenAPI    specs/api/openapi.yaml           34 endpoints
  SSaC       specs/backend/service/           34 functions
  STML       specs/frontend/                  18 pages
  Terraform  specs/terraform/                  3 modules
  
  Custom components: 7 (3 wrappers, 4 custom+JSDoc)
  Custom calculations: 2 (cart-page.custom.ts, dashboard-page.custom.ts)
  
  Last validated: 2026-03-08 14:32:00 ✓
```

## 교차 검증 규칙

Fullend의 고유 가치는 5번째 단계 `cross-validate`에 있다. 개별 도구가 잡지 못하는 계층 간 불일치를 잡는다.

### STML ↔ OpenAPI

| 검증 | 규칙 |
|---|---|
| data-fetch | operationId가 OpenAPI에 존재하는가 |
| data-action | operationId가 존재하고 HTTP method가 맞는가 |
| data-field | 해당 엔드포인트 request schema에 필드가 있는가 |
| data-bind | response schema에 필드가 있는가 (없으면 custom.ts 체크) |
| data-param-* | parameters에 해당 파라미터가 있는가 |
| data-each | response의 해당 필드가 배열인가 |

### SSaC ↔ OpenAPI

| 검증 | 규칙 |
|---|---|
| @param request | 해당 엔드포인트 request에 필드가 있는가 |
| @result + response @var | response schema에 필드가 있는가 |
| 함수명 | operationId와 매칭되는가 |

### SSaC ↔ DDL

| 검증 | 규칙 |
|---|---|
| @model Model.Method | sqlc 쿼리에 해당 메서드가 있는가 |
| @result Type | DDL 테이블에서 파생된 struct와 일치하는가 |
| @param 타입 | DDL 컬럼 타입과 일치하는가 |

### OpenAPI x- ↔ DDL

| 검증 | 규칙 |
|---|---|
| x-sort.allowed | 해당 컬럼이 테이블에 존재하는가, 인덱스가 있는가 |
| x-filter.allowed | 해당 컬럼이 테이블에 존재하는가 |
| x-include.allowed | FK 관계로 연결된 테이블인가 |

### STML ↔ SSaC (간접)

STML과 SSaC가 같은 OpenAPI operationId를 참조하므로, 양쪽 검증이 통과하면 프론트엔드가 호출하는 API와 백엔드가 처리하는 API의 일치가 보장된다. 별도 직접 검증이 필요 없다.

## 프로젝트 구조

```
specs/                              # SSOT (사용자의 결정)
├── api/
│   └── openapi.yaml               #   API 계약
├── db/
│   ├── schema.sql                 #   DDL
│   └── queries/                   #   sqlc 쿼리
├── backend/
│   └── service/                   #   SSaC 서비스 흐름
├── frontend/
│   ├── session-page.html          #   STML 페이지 선언
│   ├── cart-page.html
│   ├── cart-page.custom.ts        #   프론트 계산 로직
│   └── components/                #   커스텀 컴포넌트
│       ├── DatePicker.tsx         #     React 생태계 래퍼
│       └── KanbanBoard.tsx        #     JSDoc 명세 + 직접 구현
├── model/
│   └── interface.go               #   비-DB 모델 계약
└── terraform/
    └── main.tf                    #   인프라

artifacts/                          # 코드젠 산출물 (재생성 가능)
├── backend/
├── frontend/
└── terraform/
```

`specs/`가 진실이다. `artifacts/`는 언제든 재생성할 수 있다.

## 에이전트 연동

`CLAUDE.md` 또는 `AGENTS.md`에 다음을 추가한다:

```markdown
## SSOT 원칙

구현(artifacts/) 수정 전 반드시 해당 SSOT를 먼저 수정한다:

- 화면 변경 → specs/frontend/*.html 먼저
- API 변경 → specs/api/openapi.yaml 먼저
- 서비스 변경 → specs/backend/service/*.go 먼저
- DB 변경 → specs/db/schema.sql 먼저

수정 후 반드시 `fullend validate specs/`를 실행한다.
에러가 0이 될 때까지 수정을 반복한다.
SSOT와 구현이 불일치하면 SSOT가 진실이다.
```

에이전트는 SSOT를 수정하고 `fullend validate`를 돌리고 에러를 읽고 수정을 반복한다. 전체 시스템을 이해할 필요 없이, validate가 가리키는 곳만 고치면 정합성이 복원된다. 똑똑한 모델은 한 번에 맞추고, 멍청한 모델은 세 번 만에 맞추는 차이일 뿐 결과는 같다.

## 규모별 SSOT 크기

| 규모 | 예시 | SSOT 줄 수 | 구현 코드 | 컨텍스트 점유율 |
|---|---|---|---|---|
| 소형 | 미용실 예약 | ~1,500 | ~1만 줄 | ~8% |
| 중형 | Jira, Notion급 | ~12,500 | ~10만 줄 | ~55% |
| 대형 | Shopify급 | ~30,000 | ~30만 줄 | ~90% |

200K 토큰 컨텍스트 기준. 중형 SaaS까지 에이전트가 전체 설계를 한 번에 읽을 수 있다.

## 관련 프로젝트

- [SSaC](https://github.com/geul-org/ssac) — Service Sequences as Code. 서비스 흐름 선언 + 심볼릭 코드젠.
- [STML](https://github.com/geul-org/stml) — SSOT Template Markup Language. UI 선언 + OpenAPI 교차 검증 + React 코드젠.

## 라이선스

MIT
