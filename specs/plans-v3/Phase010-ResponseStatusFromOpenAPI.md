# Phase010: @response HTTP 상태 코드 — OpenAPI 기준 치환

## 목표

SSaC가 생성한 `__RESPONSE_STATUS__` 마커를 fullend gluegen 후처리에서 OpenAPI 성공 응답 코드로 치환한다.

## 배경

SSaC 수정지시서 009 적용 완료. `@response` 템플릿이 `http.StatusOK` 대신 `__RESPONSE_STATUS__` 마커를 출력한다. SSaC는 성공 응답의 HTTP 코드를 일절 모르며, OpenAPI가 SSOT이다.

변경된 SSaC 템플릿 (수정지시서 009 실행결과 확인):
- `response`: `c.JSON(__RESPONSE_STATUS__, gin.H{...})`
- `response_direct`: `c.JSON(__RESPONSE_STATUS__, {{.Target}})`
- Guard 에러 코드(`@empty` 404 등)는 SSaC 소관, 변경 없음

Guard 에러 코드(`@empty` 404, `@state` 409, `@auth` 403 등)는 SSaC가 결정하므로 변경 없음.

## OpenAPI 검증 규칙 (validate 단계)

| 상황 | 처리 |
|---|---|
| 2xx 코드 명시 (`200`, `201`, `204` 등) | 통과 |
| `default`만 있고 2xx 없음 | **ERROR** — "성공 응답 코드를 명시하세요 (200, 201, 204 등). default는 성공 코드로 인정하지 않습니다" |

이 검증은 `internal/orchestrator/validate.go`의 `validateOpenAPI` 또는 `internal/crosscheck/`에서 수행한다. SSaC에 `@response`가 있는 operationId에 대해서만 검증하면 충분하다.

## 치환 규칙 (gen 단계)

| OpenAPI 성공 응답 | 치환 결과 |
|---|---|
| `200` | `http.StatusOK` |
| `201` | `http.StatusCreated` |
| `204` | `c.JSON(__RESPONSE_STATUS__, ...)` 줄 전체를 `c.Status(http.StatusNoContent)` 로 교체 |

validate 단계에서 2xx 미명시를 이미 ERROR로 튕기므로 gen 단계에서 fallback은 불필요하다.

### 성공 응답 코드 결정 로직

operationId로 OpenAPI operation을 찾고, `responses`에서 2xx 코드를 추출한다:

```go
func resolveSuccessStatus(doc *openapi3.T, operationID string) (string, error) {
    if doc == nil || doc.Paths == nil {
        return "", fmt.Errorf("OpenAPI doc not available")
    }
    for _, pi := range doc.Paths.Map() {
        for _, op := range pi.Operations() {
            if op.OperationID != operationID {
                continue
            }
            for code := range op.Responses.Map() {
                if len(code) == 3 && code[0] == '2' {
                    return httpStatusConst(code), nil
                }
            }
            return "", fmt.Errorf("operationId %q has no 2xx response code", operationID)
        }
    }
    return "", fmt.Errorf("operationId %q not found in OpenAPI", operationID)
}
```

OpenAPI operation당 2xx 성공 코드는 하나만 존재한다고 가정.

### operationId 매핑

SSaC가 생성하는 서비스 파일은 파일명이 `snake_case(operationId).go`이고, 함수명이 `operationId`이다. `transformSource` 단계에서 이미 파일별로 처리하므로, 파일명에서 operationId를 역추출할 수 있다.

또는 SSaC ServiceFunc 목록에서 파일명 → operationId 매핑을 구성한다.

## 구현 위치

`internal/gluegen/gluegen.go`의 `transformSource` 함수 내부 또는 직후.

`transformSource`가 이미 문자열 치환 후처리를 수행하므로, 같은 흐름에 `__RESPONSE_STATUS__` 치환을 추가한다.

```go
// transformSource 내부, 기존 치환 로직 이후:
if strings.Contains(src, "__RESPONSE_STATUS__") {
    statusCode := resolveSuccessStatus(doc, operationID)
    if statusCode == "http.StatusNoContent" {
        // c.JSON(__RESPONSE_STATUS__, gin.H{...}) → c.Status(http.StatusNoContent)
        // c.JSON(__RESPONSE_STATUS__, target) → c.Status(http.StatusNoContent)
        re := regexp.MustCompile(`c\.JSON\(__RESPONSE_STATUS__,\s*[^)]+\)`)
        src = re.ReplaceAllString(src, "c.Status(http.StatusNoContent)")
    } else {
        src = strings.ReplaceAll(src, "__RESPONSE_STATUS__", statusCode)
    }
}
```

## 변경 파일

| 파일 | 변경 |
|---|---|
| `internal/crosscheck/ssac_openapi.go` | `@response`가 있는 operationId에 2xx 응답 미명시 시 ERROR 추가 |
| `internal/gluegen/gluegen.go` | `transformSource` 시그니처에 `doc *openapi3.T`, `operationID string` 추가 |
| `internal/gluegen/gluegen.go` | `__RESPONSE_STATUS__` → OpenAPI 성공 코드 치환 로직 추가 |
| `internal/gluegen/gluegen.go` | `transformServiceFiles` 에서 파일명 → operationID 매핑 후 `transformSource`에 전달 |
| `internal/gluegen/domain.go` | domain 모드 `transformSource` 호출부 동일 변경 |

## 의존성

- SSaC 수정지시서 009 완료 (`__RESPONSE_STATUS__` 마커 출력 확인)

## 검증

1. `go test ./...` — 빌드/테스트 통과
2. `fullend validate specs/gigbridge` — 2xx 미명시 operation 있으면 ERROR 확인
3. `fullend gen specs/gigbridge artifacts/gigbridge` — 생성된 서비스 파일에서:
   - 200 응답 엔드포인트 → `c.JSON(http.StatusOK, ...)` 확인
   - 201 응답 엔드포인트 → `c.JSON(http.StatusCreated, ...)` 확인
   - 204 응답 엔드포인트 → `c.Status(http.StatusNoContent)` 확인
   - `__RESPONSE_STATUS__` 잔존 없음 확인
4. 생성된 코드 `go build` 통과
5. `default`만 있는 테스트용 OpenAPI로 validate 시 ERROR 출력 확인

## 상태: 대기 — SSaC 009 완료, 실행 가능
