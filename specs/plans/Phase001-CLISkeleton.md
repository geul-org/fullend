# ✅ 완료 — Phase 1 — CLI Skeleton + Go 모듈 초기화

## 목표
fullend CLI의 뼈대를 만든다. `fullend validate`, `fullend gen`, `fullend status` 서브커맨드가 라우팅되고, ssac/stml 모듈을 import할 수 있는 상태까지.

## 변경 파일

| 파일 | 작업 |
|---|---|
| `go.mod` | 생성. module github.com/geul-org/fullend, Go 1.22+ |
| `go.sum` | 생성 (go mod tidy) |
| `artifacts/cmd/fullend/main.go` | 생성. 서브커맨드 라우팅 (validate/gen/status) |

## 의존성

| 패키지 | 용도 |
|---|---|
| `github.com/geul-org/ssac` | parser, validator, generator |
| `github.com/geul-org/stml` | parser, validator, generator |
| `github.com/getkin/kin-openapi` | OpenAPI 3.x 파싱/검증 |

go.mod에 replace 디렉티브:
```
replace github.com/geul-org/ssac => ../ssac
replace github.com/geul-org/stml => ../stml
```

## 서브커맨드 인터페이스

```
fullend validate <specs-dir>
fullend gen <specs-dir> <artifacts-dir>
fullend status <specs-dir>
```

표준 라이브러리만으로 CLI 파싱 (flag 패키지 + os.Args). 외부 CLI 프레임워크 사용하지 않는다.

## 검증 방법

- `go build ./artifacts/cmd/fullend/` 성공
- `fullend validate` → "not implemented" 메시지 출력
- `fullend gen` → "not implemented" 메시지 출력
- `fullend status` → "not implemented" 메시지 출력
- `fullend` (인수 없음) → usage 출력
