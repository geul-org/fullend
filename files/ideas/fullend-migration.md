# Fullend 레거시 마이그레이션 파이프라인

> 레거시 시스템을 Fullend SSOT 생태계로 이전하는 자동화 아키텍처. 레거시 코드의 내부 로직 해석을 최소화하고, 결과물(화면 + API 응답) 비교로 정확성을 검증한다.

## 철학

레거시 코드의 스파게티 로직을 직접 파싱하면 AI 에이전트에게 불필요한 인지 부하와 환각을 유발한다. 이 파이프라인은 "내부를 이해하자"가 아니라 **"결과가 같으면 된다"**는 결과론적 접근을 취한다.

다만 화면만으로는 부족하다. 화면이 동일해도 DB에 레코드가 안 들어갔을 수 있고, 금액 계산이 1원 차이날 수 있다. 따라서 **화면 비교 + API 응답 비교**를 병행한다.

## 파이프라인 아키텍처

5단계로 구성된다. 1~3단계는 SSOT 역산출, 4~5단계는 검증 루프.

### 1단계 — SSOT 역산출

레거시에서 추출 가능한 정보로 5개 SSOT 초안을 산출한다.

| SSOT | 추출 방법 | 정확도 |
|---|---|---|
| DDL | DB 스키마 덤프 (`pg_dump --schema-only`) | 100% (기계적) |
| OpenAPI | API 트래픽 캡처 (프록시 기록) 또는 기존 문서 | 90%+ (보정 필요) |
| STML | 화면 크롤링 → VLM이 HTML 구조 + data-* 역산출 | 70~80% (반복 보정) |
| SSaC | OpenAPI + DDL 기반으로 서비스 흐름 추론 (LLM) | 60~70% (검증 루프 의존) |
| Terraform | 기존 인프라 설정 export 또는 신규 설계 | 상황별 |

DDL은 덤프하면 끝이고, OpenAPI는 트래픽에서 역산출 가능하다. STML과 SSaC는 검증 루프에서 보정한다.

### 2단계 — STML에서 E2E 시나리오 자동 파생

STML의 `data-*` 속성에서 사용자 시나리오를 기계적으로 추출한다.

```html
<!-- STML: reservation-page.html -->
<section data-fetch="ListReservations" data-param-user-id="currentUser.id">
  <ul data-each="reservations">
    <li data-bind="title"></li>
    <li data-bind="status"></li>
  </ul>
  <p data-state="reservations.empty">예약이 없습니다</p>
</section>

<div data-action="CreateReservation">
  <input data-field="RoomID" type="number" />
  <input data-field="StartAt" type="datetime-local" />
  <button type="submit">예약</button>
</div>
```

파생되는 Playwright 시나리오:

```typescript
// 자동 생성: reservation-page.spec.ts

test('ListReservations 조회', async ({ page }) => {
  await page.goto('/reservations');
  await page.waitForResponse('**/api/reservations*');
  // 스크린샷 A (레거시) vs B (Fullend) 비교
  // API 응답 A vs B 비교
});

test('CreateReservation 생성', async ({ page }) => {
  await page.goto('/reservations');
  await page.fill('[data-field="RoomID"]', '101');
  await page.fill('[data-field="StartAt"]', '2026-03-10T14:00');
  await page.click('[type="submit"]');
  await page.waitForResponse('**/api/reservations');
  // 스크린샷 비교
  // API 응답 비교 (생성된 레코드 일치 여부)
});

test('빈 목록 상태 표시', async ({ page }) => {
  // data-state="reservations.empty" → 빈 상태 시나리오
  await page.goto('/reservations?user=new-user');
  await page.waitForSelector('[data-state="reservations.empty"]');
  // 스크린샷 비교
});
```

시나리오 파생 규칙:

| STML 속성 | 파생 시나리오 |
|---|---|
| `data-fetch` | 페이지 진입 → API 호출 → 응답 대기 → 렌더링 확인 |
| `data-action` + `data-field` | 필드 입력 → 제출 → API 호출 → 응답 확인 |
| `data-each` | 목록 렌더링 → 항목 수 일치 확인 |
| `data-state` | 조건부 상태 (빈 목록, 로딩, 에러) 시나리오 |
| `data-component` | 커스텀 컴포넌트 렌더링 확인 |

STML 페이지 하나당 시나리오 3~10개가 자동 생성된다. 18 페이지면 약 50~180개 시나리오.

### 3단계 — 병렬 실행 및 이중 비교

레거시(소스 A)와 Fullend 산출물(소스 B)을 동시에 구동하고, 자동 생성된 시나리오를 양쪽에서 실행한다.

```
포트 8080: 레거시 (소스 A)
포트 8081: fullend gen 산출물 (소스 B)
```

비교 대상은 두 가지:

**화면 비교 (Visual Regression)**
- 각 시나리오 스텝마다 양쪽 스크린샷 캡처
- Pixelmatch로 픽셀 레벨 비교
- 유사도 99.5% 이상이면 통과

**API 응답 비교 (Behavioral Regression)**
- 같은 요청을 양쪽에 보내고 응답 JSON을 비교
- 동적 필드(id, created_at, token) 마스킹 후 구조 + 값 비교
- 응답 상태 코드 일치 확인

```
시나리오 실행
  ├─ 화면 비교     → 시각적 동일성 (프론트엔드 정합)
  └─ API 응답 비교  → 동작 동일성 (백엔드 정합)
```

둘 다 통과해야 해당 시나리오 성공.

### 4단계 — 자율 복구 루프

비교 실패 시 에이전트가 SSOT를 수정하고 재검증한다.

```
비교 실패
  → 화면 diff 이미지 + API 응답 diff → VLM/LLM에게 전달
  → 에이전트가 원인 분석
  → SSOT 수정 (STML, OpenAPI, SSaC, DDL 중 해당 계층)
  → fullend validate
  → fullend gen
  → 실패한 시나리오부터 재실행
  → 반복 (최대 N회)
```

N회 반복 후에도 실패하면 **인간 리뷰 큐**에 넣는다. 100% 무인이 아니라, 에이전트가 해결 못하는 케이스만 인간에게 에스컬레이션한다.

### 5단계 — 완료 판정

모든 시나리오가 양쪽 비교를 통과하면 마이그레이션 완료.

```
마이그레이션 리포트:
  총 시나리오: 142
  자동 통과: 128 (90.1%)
  자율 복구 후 통과: 11 (7.7%)
  인간 리뷰 필요: 3 (2.1%)
  
  SSOT 산출물:
    DDL: 12 tables
    OpenAPI: 34 endpoints
    SSaC: 34 functions
    STML: 18 pages
    
  이후 유지보수: fullend validate + fullend gen
```

## Diff 정책

False alarm을 방지하기 위한 규칙:

| 정책 | 문제 | 해결 |
|---|---|---|
| 동적 데이터 마스킹 | 시간, 난수 ID, 광고 등 매번 다른 요소 | 해당 DOM 요소를 스크린샷 전 숨김 처리. API 응답에서도 동적 필드 마스킹 |
| 렌더링 허용치 | 폰트 안티앨리어싱, 1~2px 오차 | 픽셀 유사도 임계치 99.5% 설정 |
| 상태 기반 대기 | A/B 응답 속도 차이로 로딩 스피너 캡처 | `networkidle` 대기 + 핵심 DOM 렌더링 완료 확인 |
| 세션/인증 동기화 | 로그인 상태 불일치 | 동일 테스트 계정으로 양쪽 사전 로그인 |

## 자동화 불가 영역 (인간 에스컬레이션)

다음은 Playwright로 시뮬레이션이 안 되므로 인간 리뷰 큐에 들어간다:

- OAuth 외부 리다이렉트 (Google, GitHub 로그인)
- 이메일/SMS 인증 플로우
- 외부 결제 게이트웨이 콜백
- CAPTCHA
- 파일 업로드 후 서버 사이드 처리 결과 (PDF 생성 등)

이 영역은 전체 시나리오의 5~10%로 예상되며, 핵심 비즈니스 플로우가 아닌 경우가 많다.

## BusyFlow 연동

이 파이프라인은 BusyFlow의 Phase 5 "기존 서비스 이전" 기능이다.

```
사용자: "이 사이트 분석해줘" (URL 입력)
BusyFlow:
  1. 화면 크롤링 → STML 역산출
  2. DB 스키마 덤프 → DDL
  3. API 트래픽 캡처 → OpenAPI
  4. STML에서 E2E 시나리오 자동 생성
  5. 병렬 실행 + 이중 비교
  6. 자율 복구 루프
  7. "분석 완료. 142개 시나리오 중 139개 자동 통과.
      3개 항목 확인 필요합니다. 이전할까요?"
```

사용자는 URL을 주고, 결과를 확인하고, "이전해"라고 말하면 끝이다.
