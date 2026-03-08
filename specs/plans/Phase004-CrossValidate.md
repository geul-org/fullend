# ✅ 완료 — Phase 4 — Cross-Validate

## 목표
fullend 고유 가치인 교차 검증을 구현한다. ssac/stml 개별 검증이 잡지 못하는 계층 간 불일치를 잡는다.

## 선행 조건
Phase 3 완료

## 변경 파일

| 파일 | 작업 |
|---|---|
| `artifacts/internal/crosscheck/crosscheck.go` | 생성. 교차 검증 엔트리포인트 |
| `artifacts/internal/crosscheck/openapi_ddl.go` | 생성. OpenAPI x-extensions ↔ DDL |
| `artifacts/internal/crosscheck/ssac_ddl.go` | 생성. SSaC 타입 ↔ DDL 타입 |
| `artifacts/internal/crosscheck/types.go` | 생성. CrossError 타입 |
| `artifacts/internal/orchestrator/validate.go` | 수정. 5번째 단계로 crosscheck 추가 |

## 교차 검증 규칙

### 규칙 1: OpenAPI x-sort.allowed ↔ DDL
- x-sort.allowed의 각 컬럼이 DDL 테이블에 존재하는가
- 존재하지 않으면 ERROR
- 인덱스 없으면 WARNING (성능)

### 규칙 2: OpenAPI x-filter.allowed ↔ DDL
- x-filter.allowed의 각 컬럼이 DDL 테이블에 존재하는가

### 규칙 3: OpenAPI x-include.allowed ↔ DDL FK
- x-include의 각 리소스가 FK 관계로 연결된 테이블인가

### 규칙 4: SSaC @result Type ↔ DDL
- @result의 Type이 DDL에서 파생된 모델명과 일치하는가
- sqlc 쿼리 cardinality(:one → *Type, :many → []Type)와 사용 패턴 일치

### 규칙 5: SSaC @param 타입 ↔ DDL 컬럼 타입
- @param이 DDL 컬럼 참조 시 타입 일치 여부

## 입력 데이터

교차 검증은 Phase 2에서 이미 로드한 데이터를 재활용:
- `kin-openapi`로 로드한 OpenAPI spec (x-extensions 포함)
- ssac `SymbolTable` (DDLTables, Models, Operations)
- ssac `[]ServiceFunc` (파싱된 서비스 함수)

새로 파싱하지 않는다. orchestrator가 중간 결과를 넘겨준다.

## 의존성

| 패키지 | 용도 |
|---|---|
| `github.com/getkin/kin-openapi/openapi3` | x-extensions 추출 |
| `github.com/geul-org/ssac/parser` | ServiceFunc 타입 |
| `github.com/geul-org/ssac/validator` | SymbolTable, DDLTable 타입 |

## 실행 조건

OpenAPI + DDL + SSaC가 모두 존재할 때만 실행. 하나라도 없으면 skip.

## 검증 방법

- x-sort에 DDL에 없는 컬럼 지정 → 에러 출력
- x-filter에 DDL에 없는 컬럼 지정 → 에러 출력
- x-include에 FK 없는 테이블 지정 → 에러 출력
- @result 타입이 DDL 모델과 불일치 → 에러 출력
- 모든 교차 검증 통과 → "0 mismatches"
