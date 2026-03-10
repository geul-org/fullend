# Phase 017: DDL nullable 컬럼 검증

## 배경

모든 모델 필드는 Go 기본 타입으로 생성된다. DDL에 NOT NULL이 없는 컬럼이 있으면 DB에서 NULL이 들어올 수 있고, Go의 `sql.Scan`이 실패한다.

nullable을 허용하면 포인터 타입(`*int64`)이 필요하고, 이는 ssac-gen 역참조, func spec Response struct 불일치 등 연쇄 복잡도를 유발한다.

## 규칙

fullend validate DDL 검증 단계에서: **모든 컬럼은 NOT NULL이어야 한다.** NOT NULL이 없는 컬럼은 ERROR.

PRIMARY KEY는 암시적 NOT NULL이므로 통과.

에러 메시지:
```
[ERROR] DDL: 테이블 "gigs" 컬럼 "freelancer_id" — NOT NULL이 없습니다. NOT NULL DEFAULT 값을 지정하세요
```

## 변경 파일

### `internal/orchestrator/validate.go`

DDL 파일 파싱 후 각 컬럼에 NOT NULL 또는 PRIMARY KEY가 있는지 검사.

### 테스트

- `TestCheckDDLNullableColumn_Error` — NOT NULL 없는 컬럼 감지
- `TestCheckDDLNullableColumn_OK` — NOT NULL, PRIMARY KEY 컬럼은 통과

## 의존성

없음. DDL 파일만 사용.

## 검증 방법

```bash
go test ./internal/orchestrator/ -run TestCheckDDLNullable -v
```

nullable 컬럼 있는 DDL로 `fullend validate` → ERROR 출력 확인.

## 상태: 구현 중
