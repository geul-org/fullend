# ✅ Phase020: Smoke Gen — object/array 타입 더미값 생성

## 목표

`generateDummyValue`에서 OpenAPI `type: object` / `type: array` 필드에 대해 유효한 JSON 더미값을 생성한다.

## 근본 원인

### 데이터 흐름

```
OpenAPI: payload_template → type: object
DDL:    payload_template → JSONB NOT NULL DEFAULT '{}'
```

### 코드 경로

1. `generateHurlTests` → `writeStep` → `generateRequestBody(reqSchema, checkEnums)`
2. `generateRequestBody`가 `schema.Properties`를 순회, 각 field에 `generateDummyValue(name, prop, checkEnums)` 호출
3. `generateDummyValue("payload_template", prop, checkEnums)`:
   - `prop.Type.Slice()[0]` = `"object"`
   - switch문에 `"object"` 케이스 없음
   - **`default: return "test_string"`** (hurl_util.go:57)
4. `formatDummyValue("test_string")` → `"\"test_string\""` (JSON quoted string)
5. 최종 출력: `"payload_template": "test_string"`

### 서버 에러 경로

```
Hurl 전송: {"payload_template": "test_string", ...}
→ Go ShouldBindJSON → Action.Create(..., "test_string", ...)
→ SQL INSERT INTO actions (..., payload_template, ...) VALUES (..., 'test_string', ...)
→ PostgreSQL: JSONB 컬럼에 'test_string'은 유효하지 않은 JSON → ERROR
→ 500 "Action 생성 실패"
```

### 문제의 본질

`generateDummyValue`의 switch문에 `"object"`, `"array"` 케이스가 없어서 default(`"test_string"`)로 빠짐.

## 영향 범위

- `type: object`인 request body field가 있는 모든 endpoint (zenflow: `payload_template`)
- `type: array`도 동일 경로로 `"test_string"` 생성 (현재 zenflow에는 해당 케이스 없음)

## 수정 포인트

`hurl_util.go` 한 파일, 두 함수:

### 1. `generateDummyValue` — object/array 케이스 추가

```go
case "object":
    return map[string]interface{}{} // 빈 JSON 객체
case "array":
    return []interface{}{}          // 빈 JSON 배열
```

### 2. `formatDummyValue` — 새 타입 포맷팅

현재 `formatDummyValue`는 string/int/float64/bool만 처리. object(`map`)와 array(`slice`)를 JSON literal로 출력해야 함:

```go
case map[string]interface{}:
    return "{}"
case []interface{}:
    return "[]"
```

이렇게 하면 `"payload_template": {}` (unquoted JSON literal)로 출력됨.

## 변경 파일 목록

| 파일 | 변경 내용 |
|---|---|
| `internal/gluegen/hurl_util.go` | `generateDummyValue`: object/array 케이스 추가, `formatDummyValue`: map/slice 케이스 추가 |

## 검증 방법

1. `go build ./cmd/fullend/` — 빌드 통과
2. `go test ./...` — 테스트 통과
3. `fullend gen specs/zenflow artifacts/zenflow` — 재생성
4. `artifacts/zenflow/tests/smoke.hurl` 확인: `"payload_template": {}` (빈 객체)
5. zenflow 서버 빌드 → hurl 테스트에서 CreateAction 통과
