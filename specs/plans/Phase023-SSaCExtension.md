# ✅ Phase 023: SSaC .ssac 확장자 대응 + go.mod 동기화

## 배경

SSaC 수정지시서 017에서:
1. 서비스 파일 확장자 `.go` → `.ssac` 변경
2. 패키지 모델 파라미터 매칭 검증 추가 (`context.Context` 자동 제외)

fullend orchestrator의 SSOT 탐지(`detect.go`)가 `service/*.go` 패턴으로 SSaC 파일을 찾고 있어 `.ssac`으로 수정 필요.

`ssacparser.ParseDir()`은 SSaC 라이브러리가 이미 `.ssac`으로 변경 완료 — fullend 측은 탐지 패턴만 수정.

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `internal/orchestrator/detect.go` | SSaC 탐지 패턴 `service/*.go` → `service/*.ssac`, 도메인 폴더 `service/*/*.go` → `service/*/*.ssac` |
| `go.mod` / `go.sum` | SSaC 모듈 동기화 (`go mod tidy`) |

## 의존성

- SSaC 수정지시서 017 완료 (`.ssac` 확장자 + 파라미터 매칭)

## 검증 방법

```bash
go test ./internal/orchestrator/...
fullend validate specs/dummy-gigbridge
fullend validate specs/dummy-lesson
fullend validate specs/dummy-study
```
