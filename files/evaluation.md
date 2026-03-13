# fullend 평가서

평가일: 2026-03-12
평가 대상: fullend v0 (Phase 027 완료 시점)
검증 프로젝트: GigBridge (프리랜싱 플랫폼)

---

## 1. 정량 지표

### 1-1. SSOT → 산출물 변환 효율

| 지표 | SSOT (입력) | Artifacts (출력) | 배율 |
|---|---|---|---|
| 파일 수 | 34 | 53 | 1.6x |
| 라인 수 | 1,098 | 3,859 | 3.5x |
| 바이트 | 32KB | 93KB (go.sum 제외) | 2.9x |
| 추정 토큰 | ~9,000 | ~26,000 | 2.9x |

결론: SSOT 1줄 작성 → 산출물 3.5줄 생성. 리뷰 대상이 코드 26k 토큰이 아니라 스펙 9k 토큰.

### 1-2. SSOT 구성 (34개 파일)

| SSOT 종류 | 파일 수 | 라인 수 | 역할 |
|---|---|---|---|
| fullend.yaml | 1 | 29 | 프로젝트 메타데이터 |
| OpenAPI | 1 | 525 | API 엔드포인트 12개 |
| DDL | 4 | 45 | 테이블 4개, 컬럼 25개 |
| SQL Queries | 4 | 33 | sqlc 쿼리 11개 |
| SSaC | 12 | 115 | 서비스 함수 12개 |
| States (Mermaid) | 2 | 18 | 상태 다이어그램 2개, 전이 7개 |
| OPA Rego | 1 | 68 | 인가 규칙 8개, 소유권 3개 |
| Gherkin Scenario | 1 | 99 | 시나리오 3개 |
| Func | 2 | 47 | 커스텀 함수 2개 |
| Model DTO | 1 | 7 | DTO 1개 |
| STML (Frontend) | 2 | 56 | 페이지 2개 |
| Terraform | 1 | 31 | 인프라 스켈레톤 |
| sqlc.yaml | 1 | 13 | sqlc 설정 |

### 1-3. 산출물 구성 (53개 파일)

| 카테고리 | 파일 수 | 내용 |
|---|---|---|
| Go 핸들러 | 12 | SSaC → gin 핸들러 코드젠 |
| Go 모델 | 8 | DDL + Queries → 모델 레이어 |
| Go 인프라 | 8 | 라우터, 미들웨어, 상태머신, authz, billing 등 |
| Go 설정 | 3 | go.mod, go.sum, main.go |
| Hurl 테스트 | 6 | smoke 12건 + scenario 30건 |
| React Frontend | 10 | STML → TSX 컴포넌트 |
| Frontend 설정 | 6 | package.json, vite.config, tsconfig 등 |

### 1-4. 검증 파이프라인 결과

| 단계 | 결과 |
|---|---|
| `fullend validate` | 10/10 SSOT 통과 (WARN 27건 — 전부 false positive) |
| `fullend gen` | 53개 파일 산출 |
| `go build` | 컴파일 성공 |
| hurl smoke test | 11/12 통과 (1건 = SMTP 외부 의존) |
| hurl scenario test | 29/30 통과 (1건 = SMTP 외부 의존) |
| 총 테스트 | **40/42 통과 (95.2%)**, 실패 2건은 fullend 외부 의존 |

### 1-5. Crosscheck 탐지 실적 (Phase 027)

`fullend validate`가 코드 생성 전에 자동 탐지한 실제 버그:

| 탐지 항목 | 내용 |
|---|---|
| 시나리오 토큰 role 불일치 3건 | AcceptProposal/SubmitWork/ApproveWork 전 토큰 재로그인 누락 |
| OpenAPI 응답 코드 누락 2건 | SubmitWork 403, ApproveWork 409 미정의 |

이 5건은 코드 생성 후 런타임에서야 발견되었을 버그. crosscheck가 specs 단계에서 차단.

---

## 2. 강점

### 2-1. SSOT 교차 검증이 실제로 작동한다
10개 SSOT 간 정합성을 기계적으로 검증. Phase 027에서 5건의 실제 버그를 코드 작성 전에 탐지. "validate 통과 = 정합성 보장"이라는 약속이 작동하는 것을 실증.

### 2-2. 선언적 스펙 → 기계적 산출
SSaC 1줄(`@auth "CreateGig" "gig" {UserID: currentUser.ID, Role: currentUser.Role}`)이 OPA 연동 + 미들웨어 + 에러 핸들링 코드로 변환. 사람이 작성할 코드량이 아니라 검증할 스펙량으로 문제가 축소.

### 2-3. E2E 테스트 자동 생성
Gherkin 시나리오 99줄 → hurl 테스트 42건 자동 산출. 수동으로 42개 hurl 파일을 작성하고 유지보수하는 비용이 제거.

### 2-4. 상태 머신 검증
Mermaid stateDiagram으로 선언한 전이 규칙이 SSaC `@state` 디렉티브와 교차 검증되고, 런타임 상태 체크 코드로 산출. 잘못된 상태 전이는 validate에서 차단.

---

## 3. 약점

### 3-1. False positive 27건 (신뢰도 저하)
validate 실행 시 27건의 WARN이 출력되지만 전부 false positive. DDL→OpenAPI 25건은 component schemas 미인식, SSaC 2건은 Page[T] 패턴 미인식. 사용자가 WARN을 무시하는 습관이 들면 진짜 문제도 놓치게 됨.

### 3-2. 의미적 검증 부재
구조적 정합성("이 필드가 존재하는가")은 검증하지만, 의미적 정합성("이 OPA 코멘트가 실제 rule과 일치하는가", "이 필드가 API에 노출되면 안 되는 민감 정보인가")은 탐지 불가.

### 3-3. 검증 프로젝트 1개
GigBridge 하나에서만 검증됨. fullend가 이 프로젝트에 오버피팅되었을 가능성을 배제할 수 없음.

### 3-4. 외부 사용자 접근성 제로
README, 튜토리얼, 설치 가이드 없음. SSaC/STML이라는 자체 DSL을 외부 사용자가 학습해야 하는 진입 장벽.

### 3-5. 시나리오 커버리지 갭 미탐지
12개 operation 중 2개(RejectProposal, RaiseDispute)가 시나리오에서 미테스트인데, validate가 이를 알려주지 않음.

---

## 4. 경쟁 환경 비교

| 도구 | 접근 방식 | fullend 대비 |
|---|---|---|
| OpenAPI Generator | 스키마 → 클라이언트/서버 스텁 | API 1개 SSOT만. 교차 검증 없음 |
| Prisma | 스키마 → DB + 타입 | DB 1개 SSOT만. API/auth/state 무관 |
| Hasura/PostgREST | DB → 자동 API | 선언적이지만 비즈니스 로직 불가 |
| Supabase | BaaS | 빠르지만 커스텀 상태 머신/인가 정책 한계 |
| Amplication | 코드젠 플랫폼 | 교차 검증 없음. SSOT가 1-2개 |

fullend의 차별점: **10개 SSOT 간 교차 검증**. 위 도구들은 각자의 영역에서 코드를 생성하지만, SSOT 간 정합성은 사람의 몫. fullend는 이 정합성 검증을 자동화.

---

## 5. 포지셔닝

### 도구로서의 fullend
니치. 자체 DSL(SSaC, STML) 학습 비용, 1개 프로젝트 검증, 문서 부재. "cool side project" 수준의 반응 예상.

### 방법론으로서의 fullend
"AI 시대에 코드 리뷰 대신 스펙 리뷰" — 이 주장의 reference implementation. 바이브 코딩의 한계가 드러나는 시점에 타이밍이 맞을 가능성.

### 생산성 도구로서의 fullend
가장 현실적. 본인이 fullend + 클로드 코드로 서비스를 빠르게 찍어내는 데 사용. 유명세는 부산물.

---

## 6. 다음 단계 우선순위

| 순위 | 항목 | 목적 |
|---|---|---|
| 1 | 두 번째 프로젝트 (당근마켓) | 재현성 증명, 오버피팅 탈출 |
| 2 | false positive 제거 | validate 신뢰도 100% |
| 3 | 시나리오 커버리지 리포트 | validate 완성도 |
| 4 | 클로드 코드 + fullend 데모 | "프롬프트 10개로 hurl 통과" 시연 |
| 5 | README + 튜토리얼 | 외부 사용자 접근성 |

---

## 7. 종합 판정

**PoC 성공. MVP 직전.**

9k 토큰의 스펙으로 26k 토큰의 검증된 코드를 산출하고, 교차 검증이 실제 버그를 잡는다는 것을 1개 프로젝트에서 실증. "specs만 잘 쓰면 나머지는 fullend가 보장한다"는 약속이 80% 수준에서 작동.

두 번째 프로젝트 성공 + false positive 제거 시 MVP. 클로드 코드 연동 데모 성공 시 공개 가능 수준.
