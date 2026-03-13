# dummy-GigBridge 개발완료 보고서

## 프로젝트 개요

- **프로젝트명**: GigBridge (Freelance Escrow Matching Platform)
- **설명**: 클라이언트가 프로젝트(gig)를 등록하고, 프리랜서가 제안(proposal)을 제출하며, 에스크로 결제를 통해 작업 완료 시 10% 플랫폼 수수료 차감 후 정산하는 매칭 플랫폼
- **개발 일시**: 2026-03-12
- **소요 시간**: 약 22분 38초 (01:51:29 ~ 02:14:07)

## 생성 파일 목록

### SSOT (specs/gigbridge/)

| # | SSOT | 파일 경로 | 내용 |
|---|---|---|---|
| 1 | fullend.yaml | `fullend.yaml` | module: github.com/gigbridge/api, bearerAuth, JWT claims |
| 2 | SQL DDL | `db/{users,gigs,proposals,transactions}.sql` | 4 테이블, 23 컬럼, 3 인덱스, sentinel 레코드 |
| 3 | sqlc queries | `db/queries/{users,gigs,proposals,transactions}.sql` | 12 쿼리 (ModelPrefix 적용) |
| 4 | OpenAPI | `api/openapi.yaml` | 12 endpoints, x-pagination/sort/filter/include |
| 5 | SSaC | `service/{auth,gig,proposal}/**/*.ssac` | 12 서비스 함수 (파일당 1함수) |
| 6 | Model | `model/model.go` | 빈 패키지 (DDL 모델만 사용) |
| 7 | Mermaid States | `states/{gig,proposal}.md` | 2 다이어그램, 7 상태 전이 |
| 8 | OPA Rego | `policy/authz.rego` | 7 allow 룰, 3 @ownership (OPA v1 if 문법) |
| 9 | Gherkin | `scenario/gig_lifecycle.feature` | 1 @scenario + 2 @invariant |
| 10 | STML | `frontend/{gigs,gig-detail}.html` | 2 페이지, data-fetch/action/paginate/sort/filter |
| 11 | Terraform | `terraform/main.tf` | AWS RDS PostgreSQL 15 |
| Opt | Func Spec | `func/billing/{hold_escrow,release_funds}.go` | 2 커스텀 함수 |

### 코드 산출물 (artifacts/gigbridge/)

| 산출물 | 경로 | 내용 |
|---|---|---|
| Backend | `backend/` | Go/Gin 서버, 12 핸들러, authz, state machine |
| Frontend | `frontend/` | React/TypeScript, 2 페이지 |
| Smoke Test | `tests/smoke.hurl` | 12 요청 (전체 API 경로 커버) |
| Invariant Test | `tests/invariant-gig_lifecycle.hurl` | 3 시나리오 (happy path + 2 invariant) |

## 비즈니스 로직 요약

### API 엔드포인트 (12개)

| Method | Path | operationId | 인증 |
|---|---|---|---|
| POST | /auth/register | Register | - |
| POST | /auth/login | Login | - |
| GET | /gigs | ListGigs | - |
| POST | /gigs | CreateGig | Bearer |
| GET | /gigs/{id} | GetGig | - |
| PUT | /gigs/{id}/publish | PublishGig | Bearer + @auth |
| POST | /gigs/{id}/proposals | SubmitProposal | Bearer + @auth |
| POST | /proposals/{id}/accept | AcceptProposal | Bearer + @auth |
| POST | /proposals/{id}/reject | RejectProposal | Bearer + @auth |
| POST | /gigs/{id}/submit-work | SubmitWork | Bearer + @auth |
| POST | /gigs/{id}/approve | ApproveWork | Bearer + @auth |
| POST | /gigs/{id}/dispute | RaiseDispute | Bearer + @auth |

### 상태 머신

**Gig**: `[*] → draft → open → in_progress → under_review → completed/disputed`
**Proposal**: `[*] → pending → accepted/rejected`

### 권한 규칙

| 액션 | 역할 | 소유권 |
|---|---|---|
| PublishGig | client | gig owner (client_id) |
| SubmitProposal | freelancer | NOT gig owner |
| AcceptProposal | client | gig owner |
| RejectProposal | client | gig owner |
| SubmitWork | freelancer | gig assignee (freelancer_id) |
| ApproveWork | client | gig owner |
| RaiseDispute | client | gig owner |

## 검증 결과

### fullend validate

```
✓ Config       gigbridge, go/gin, typescript/react
✓ OpenAPI      12 endpoints
✓ DDL          4 tables, 23 columns
✓ SSaC         12 service functions
✓ Model        1 files
✓ STML         2 pages, 2 bindings
✓ States       2 diagrams, 7 transitions
✓ Policy       1 files, 7 rules, 3 ownership mappings
✓ Scenario     1 features, 3 scenarios
✓ Func         2 funcs
✓ Terraform    1 files
✓ Cross        27 warnings
```

- **ERROR**: 0
- **WARNING**: 27 (DDL→OpenAPI $ref 미해석 23건 + Scenario↔Policy 토큰 role 추적 4건)

### fullend gen

```
✓ sqlc, oapi-gen, ssac-gen, ssac-model, stml-gen, glue-gen
✓ hurl-gen, state-gen, authz-gen, scenario-gen, func-gen, terraform
```

12/12 codegen 단계 전체 통과.

### go build

빌드 성공 (0 errors).

### hurl --test

| 테스트 | 결과 | 요청 수 | 소요 시간 |
|---|---|---|---|
| smoke.hurl | **PASS** | 12/12 | 193ms |
| invariant-gig_lifecycle.hurl | **FAIL** | 25/26 | 593ms |

- **smoke.hurl**: DISABLE_AUTHZ=1, DISABLE_STATE_CHECK=1 환경에서 12개 전체 요청 성공
- **invariant.hurl**: 403 기대 케이스에서 200 반환 (authz 비활성화 상태이므로 예상된 실패)

## 발견된 버그

### BUG-1: authz-gen — OPA data.owners 미로딩 (CRITICAL)

`@ownership` 기반 소유권 데이터가 OPA 평가 전 DB에서 로딩되지 않아, DISABLE_AUTHZ 없이는 모든 @auth 검증이 실패합니다. 또한 OPA input 키 `resource_owner_id`와 Rego 참조 `input.resource_id` 간 이름 불일치도 존재합니다.

### BUG-2: hurl-gen — 중첩 객체 토큰 캡처 (MINOR)

Login 응답이 중첩 객체일 때 `$.token` 전체를 캡처하여 Bearer 토큰으로 사용 시 렌더링 에러 발생. SSaC `@response` 플랫화로 우회 완료.

### BUG-3: pkg/auth — JSON 태그 누락 (MINOR)

`IssueTokenResponse.AccessToken` 필드에 `json:"access_token"` 태그 없음. PascalCase 직렬화로 OpenAPI snake_case 규칙과 불일치.

## 결론

- **SSOT 10개 + Func Spec 전체 작성 완료**, `fullend validate` ERROR 0 통과
- **코드 생성 → 빌드 → smoke 테스트** 파이프라인 정상 통과
- **invariant 테스트**는 authz-gen 버그(BUG-1)로 인해 DISABLE_AUTHZ=1 환경에서만 부분 검증 가능
- BUG-1 수정 후 invariant 테스트 재실행 필요
