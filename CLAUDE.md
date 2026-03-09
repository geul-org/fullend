# fullend

## 프로젝트 루트
~/.clari/repos/fullend

## 프로젝트 개요
Full-stack SSOT orchestrator — 7개 SSOT(STML, OpenAPI, SSaC, SQL DDL, Mermaid stateDiagram, OPA Rego, Terraform)의 정합성을 한 번에 검증하고 코드를 산출하는 Go CLI.

## 디렉토리

```
fullend/
├── files/                  # 기초 자료 (기획서, 참고 문서)
├── specs/                  # 설계 원천 (SSOT). 구현보다 우선
│   └── plans/              # 구현 계획서
├── artifacts/              # 코드 산출물 (SSOT에서 파생)
│   ├── cmd/fullend/        # CLI 엔트리포인트
│   │   └── main.go
│   └── internal/
│       ├── orchestrator/   # validate, gen, status 오케스트레이션
│       ├── crosscheck/     # SSOT 간 교차 검증 (fullend 고유 로직)
│       ├── statemachine/   # Mermaid stateDiagram 파서
│       ├── policy/         # OPA Rego 정책 파서
│       └── reporter/       # 검증 결과 출력 포매터
└── go.mod                  # github.com/geul-org/fullend
```

## 형제 프로젝트 (로컬 clone)

| 프로젝트 | 경로 | 역할 |
|---|---|---|
| SSaC | ~/.clari/repos/ssac | 서비스 흐름 선언 → Go 코드젠 |
| STML | ~/.clari/repos/stml | UI 선언 → React TSX 코드젠 |

fullend는 ssac, stml을 Go 모듈로 import하여 사용한다.
개발 중 로컬 참조: `go.mod`에 `replace` 디렉티브 사용.

## SSOT 원칙

**구현(artifacts/) 수정 전 반드시 해당 SSOT를 먼저 작성/수정한다.**

| 변경 대상 | SSOT 먼저 | 포맷 |
|---|---|---|
| API | `<root>/api/openapi.yaml` | OpenAPI 3.x |
| DB | `<root>/db/*.sql` + `<root>/db/queries/*.sql` | SQL DDL + sqlc 쿼리 |
| 서비스 로직 | `<root>/service/*.go` | SSaC (Go comment DSL) |
| 모델 계약 | `<root>/model/*.go` | Go interface |
| UI 레이아웃 | `<root>/frontend/*.html` | STML (HTML5 + data-*) |
| 상태 전이 | `<root>/states/*.md` | Mermaid stateDiagram |
| 인가 정책 | `<root>/policy/*.rego` | OPA Rego |
| 인프라 | `<root>/terraform/` | HCL |

SSOT와 구현이 불일치하면 **SSOT가 진실**이다.

## 계획 작성 원칙

구현 전 `specs/plans/`에 계획 md를 작성한다.

- 파일명: `PhaseNNN-TITLE.md` (예: `Phase001-CLISkeleton.md`)
- 구현 코드를 쓰기 전에 계획을 먼저 작성하고 승인을 받는다
- 계획에는 다음을 포함한다:
  - 목표: 무엇을 만드는가
  - 변경 파일 목록: 어떤 파일을 생성/수정하는가
  - 의존성: 외부 패키지, 형제 프로젝트 API
  - 검증 방법: 어떻게 확인하는가
- 계획이 승인되면 구현하고, 완료 후 계획 상단에 `✅ 완료` 표시

## CLI 명령어

### fullend validate <specs-dir>
SSOT 개별 검증 + 교차 정합성 검증.

```
1. sqlc compile                    ← DDL + 쿼리 검증
2. openapi-generator validate      ← OpenAPI 스키마 검증
3. ssac validate                   ← 서비스 흐름 내부 + OpenAPI/DDL 교차
4. stml validate                   ← UI 선언 내부 + OpenAPI 교차
5. states validate                 ← Mermaid stateDiagram 파싱 검증
6. policy validate                 ← OPA Rego 정책 파싱 검증
7. fullend cross-validate          ← SSOT 간 전체 교차 (고유 로직)
```

### fullend gen <specs-dir> <artifacts-dir>
검증 통과 후 전체 코드 산출.

```
1. fullend validate specs/         ← 먼저 검증
2. sqlc generate                   ← DB 모델 Go struct
3. openapi-generator generate      ← API 핸들러/클라이언트
4. ssac gen                        ← 서비스 함수 + Model interface
5. stml gen                        ← React TSX 컴포넌트
6. glue-gen                        ← Server struct + main.go + frontend setup
7. hurl-gen                        ← OpenAPI → Hurl 스모크 테스트
8. state-gen                       ← Mermaid stateDiagram → Go 상태 머신 패키지
9. authz-gen                       ← OPA Rego → Go Authorizer 구현체
10. terraform fmt                  ← HCL 포맷팅
```

### fullend status <specs-dir>
SSOT 현황 요약 출력.

## 교차 검증 규칙 (fullend 고유 가치)

개별 도구(ssac, stml)가 잡지 못하는 계층 간 불일치를 잡는다.

### STML ↔ OpenAPI
- data-fetch/action → operationId 존재 + HTTP method 일치
- data-field → request schema 필드 존재
- data-bind → response schema 필드 존재 (없으면 custom.ts)
- data-param-* → parameters 존재
- data-each → response 필드가 배열인지

### SSaC ↔ OpenAPI
- @param request → 엔드포인트 request 필드 존재
- @result + response @var → response schema 필드 존재
- 함수명 → operationId 매칭

### SSaC ↔ DDL
- @model Model.Method → sqlc 쿼리 메서드 존재
- @result Type → DDL 테이블 파생 struct 일치
- @param 타입 → DDL 컬럼 타입 일치

### OpenAPI x- ↔ DDL
- x-sort.allowed → 컬럼 존재 + 인덱스 여부
- x-filter.allowed → 컬럼 존재
- x-include.allowed → 정방향 FK 관계 검증 (`column:table.column` 형식)

### States ↔ SSaC
- stateDiagram 전이 이벤트 → SSaC 함수 존재
- SSaC guard state → 해당 stateDiagram 존재
- 전이가 있는 operationId에 guard state 없음 → WARNING

### States ↔ DDL
- guard state 상태 필드 → DDL 컬럼 존재

### States ↔ OpenAPI
- 전이 이벤트명 → OpenAPI operationId 존재

### Policy ↔ SSaC
- SSaC authorize의 (action, resource) 쌍 → Rego에 매칭 allow 규칙 존재
- Rego allow 규칙의 (action, resource) 쌍 → SSaC에 매칭 authorize 시퀀스 존재
- Rego에서 input.resource_owner 참조 → 해당 resource의 @ownership 주석 존재

### Policy ↔ DDL
- @ownership 주석의 table.column → DDL에 해당 테이블·컬럼 존재
- @ownership via join의 join_table.fk → DDL에 해당 테이블·컬럼 존재

### Policy ↔ States
- stateDiagram 전이 이벤트에 authorize가 있으면 → Rego에 매칭 allow 규칙 존재

### STML ↔ SSaC (간접)
양쪽이 같은 OpenAPI operationId를 참조하므로, 개별 검증 통과 시 프론트/백엔드 일치 보장.

## SSaC 참조

### 11가지 시퀀스 타입 (닫힌 집합)
authorize, get, guard nil, guard exists, guard state, post, put, delete, password, call, response

### 예시
```go
// @sequence get
// @model User.FindByEmail
// @param Email request
// @result user User
//
// @sequence guard nil user
// @message "사용자를 찾을 수 없습니다"
//
// @sequence response json
// @var user
func Login(w http.ResponseWriter, r *http.Request) {}
```

### SSaC 패키지 (모듈 루트)
- `parser/` — Go AST 기반 @태그 파싱 → []ServiceFunc
- `validator/` — 내부 검증 + SymbolTable 교차 검증
- `generator/` — 템플릿 기반 Go 코드젠 + gofmt

## STML 참조

### 11가지 data-* 속성
| 핵심 (8) | 인프라 (3) |
|---|---|
| data-fetch, data-action, data-field, data-bind, data-param-*, data-each, data-state, data-component | data-paginate, data-sort, data-filter |

### STML 패키지 (모듈 루트)
- `parser/` — HTML5 DOM 파싱 → []PageSpec
- `validator/` — OpenAPI 교차 검증 (12개 규칙)
- `generator/` — React TSX 코드젠 (useQuery, useMutation, useForm)

## 구현 전략

fullend CLI는 ssac, stml을 라이브러리로 호출하고, 자체적으로는 다음만 구현:
1. **orchestrator** — CLI 파싱 + 실행 순서 제어 + 외부 도구 exec 호출
2. **crosscheck** — SSOT 간 교차 검증
3. **statemachine** — Mermaid stateDiagram 파서 + Go 상태 머신 코드젠
4. **policy** — OPA Rego 정책 파서 + Go Authorizer 코드젠
5. **reporter** — 검증 결과 통합 출력 (✓/✗ 형식)

### validate 단계 — Go 라이브러리 우선
- OpenAPI 검증: `kin-openapi` (github.com/getkin/kin-openapi, MIT)
- DDL 파싱: ssac `validator.LoadSymbolTable()` 재활용
- SSaC 검증: ssac `parser` + `validator` 직접 호출
- STML 검증: stml `parser` + `validator` 직접 호출

### gen 단계 — 외부 도구 exec 호출
- `sqlc generate` — DB 모델 Go struct
- ssac `generator.Generate()` — 서비스 함수 코드젠
- stml `generator.Generate()` — React TSX 코드젠
- `terraform fmt` — HCL 포맷팅

## 형제 프로젝트 수정 금지

ssac, stml 코드를 직접 수정하지 않는다.
수정이 필요하다고 판단되면:
1. 해당 프로젝트의 `files/` 에 수정요구서 md를 작성한다 (예: `~/.clari/repos/ssac/files/수정요구-xxx.md`)
2. fullend 쪽 작업을 즉시 중단한다
3. 사용자에게 보고하고 승인을 기다린다

## Coding Conventions

- Go 1.22+, gofmt 준수, 에러 즉시 처리 (early return)
- 파일명: snake_case, 변수/함수: camelCase, 타입: PascalCase
- 외부 도구 재발명 금지 — 오케스트레이션과 교차 검증만 구현
- 테스트: `go test ./...` 통과 필수

## Git 커밋/푸시 절차

1. **민감 정보 확인 필수** — 커밋 전 .env, 비밀키, 토큰, 자격증명 등 민감 정보가 포함되지 않았는지 반드시 확인한다
2. **Co-Authored-By 금지** — 커밋 메시지에 Co-Authored-By를 절대 넣지 않는다. GitHub 사용자 목록에 claude 이름이 표시되지 않도록 주의한다
