# UseCase SSOT 신설 검토

## 배경

현재 5개 SSOT(OpenAPI, SQL DDL, SSaC, STML, Terraform)로 빌드 가능한 풀스택을 생성할 수 있다. 그러나 런타임 비즈니스 로직의 정확성을 기계적으로 검증하려면, 현재 SSOT에 선언할 곳이 없는 정보가 있다.

## 현재 SSOT가 커버하지 못하는 것

| 정보 | 현재 위치 | 문제 |
|---|---|---|
| 인가 정책 (누가 무엇을 할 수 있나) | SSaC `authorize`에 action/resource만 있음 | "본인 강의만 수정 가능"이 선언 안 됨 |
| 상태 전이 (리소스 생명주기) | 없음 | Course가 draft→published→archived로 가는 흐름이 암묵적 |
| 크로스 엔드포인트 흐름 | 없음 | "등록→로그인→강의생성→공개→수강신청" 순서가 어디에도 없음 |
| 크로스 엔드포인트 불변식 | 없음 | "삭제된 강의는 목록에 안 나온다"가 선언 안 됨 |
| 선행 조건 | SSaC guard로 부분 커버 | "공개된 강의만 수강 가능"은 EnrollCourse에 없고 암묵적 |

## 기존 SSOT 확장으로 해결 가능한가

**SSaC 확장?** — SSaC는 단일 함수 스코프다. 함수 간 관계를 넣으면 성격이 달라진다.

**OpenAPI 확장?** — x- 확장을 더 넣을 수 있지만, 인가 정책이나 상태 전이를 YAML에 넣으면 openapi.yaml이 비대해지고 본래 역할(API 계약)을 벗어난다.

**결론: 기존 SSOT에 끼워넣으면 각 SSOT의 단일 책임이 깨진다. 신설이 맞다.**

## UseCase SSOT가 담아야 할 것

```
1. 인가 정책     — 누가, 무엇을, 어떤 조건에서
2. 상태 전이     — 리소스 생명주기
3. 시나리오 흐름  — 엔드포인트 간 의존 순서 + 데이터 흐름
4. 불변식        — 항상 참이어야 하는 조건
```

## 포맷 후보

SSaC처럼 Go 주석 DSL로 갈 수도 있고, YAML이나 별도 DSL로 갈 수도 있다. 예시로 가늠:

```yaml
# specs/usecase/course-lifecycle.yaml
usecase: CourseLifecycle

policy:
  CreateCourse:  { role: instructor }
  UpdateCourse:  { role: instructor, owner: course.InstructorID }
  DeleteCourse:  { role: instructor, owner: course.InstructorID }
  PublishCourse: { role: instructor, owner: course.InstructorID }
  EnrollCourse:  { role: student, precondition: course.Published }

states:
  Course:
    draft:      { on: PublishCourse → published }
    published:  { on: DeleteCourse → deleted }

scenarios:
  instructor-creates-and-publishes:
    - Register { role: instructor }
    - Login → token
    - CreateCourse → course
    - CreateLesson { CourseID: course.ID } → lesson
    - PublishCourse { CourseID: course.ID }
    - assert: ListCourses contains course.ID

  student-enrolls:
    - Register { role: student }
    - Login → token
    - EnrollCourse { CourseID: course.ID } → enrollment
    - assert: ListMyEnrollments contains enrollment.ID
    - assert: ListMyPayments count > 0

  negative:
    - EnrollCourse without auth → 401
    - EnrollCourse { CourseID: 999999 } → 404
    - EnrollCourse twice → 409
    - UpdateCourse by non-owner → 403
    - EnrollCourse on unpublished → 403

invariants:
  - after DeleteCourse: ListCourses excludes course.ID
  - after EnrollCourse: Enrollment.count += 1
```

## 교차 검증에 미치는 영향

6번째 SSOT가 되면 fullend crosscheck에 규칙이 추가된다:

| 규칙 | 검증 |
|---|---|
| UseCase policy.role ↔ SSaC authorize | authorize가 있는데 policy가 없거나 그 역 |
| UseCase states ↔ DDL | 상태 컬럼(published, status)이 DDL에 존재하는가 |
| UseCase scenarios 엔드포인트 ↔ OpenAPI | 시나리오에 쓴 operationId가 존재하는가 |
| UseCase precondition ↔ SSaC guard | "course.Published" 조건이 SSaC에 guard로 있는가 |
| UseCase invariants ↔ SSaC sequence | "삭제 후 목록 제외"가 실제 구현과 일치하는가 |

## 판단

**신설해야 한다.** 이유:

1. 10000파일 규모에서 인가 정책이 암묵적이면 보안 구멍이 생긴다. 선언해야 검증 가능하다.
2. 상태 전이가 선언 없으면 "공개 전 수강 가능" 같은 버그를 기계적으로 잡을 수 없다.
3. 크로스 엔드포인트 시나리오가 있어야 Hurl 2단계도 자동 생성이 가능해진다.
4. 바이브 코더 + Claude Code 시나리오에서, AI가 이 파일도 같이 생성하면 비즈니스 로직 검증까지 자동화된다.

## Hurl 테스트와의 관계

```
1단계: OpenAPI → 스모크 테스트           ← 완전 자동 (Phase019)
2단계: UseCase SSOT → 비즈니스 시나리오   ← UseCase 파서 구현 후 자동 생성
3단계: 크로스 엔드포인트 불변식 검증       ← UseCase invariants에서 파생
```

Phase019(Hurl 1단계)와는 독립적이다. UseCase SSOT 설계는 별도 Phase로 가져간다.
