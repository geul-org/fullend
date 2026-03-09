# Phase 029: fullend.yaml 복수 backends/frontends/databases 전환

## 목표

fullend.yaml의 `backend`/`frontend` 단수 형태를 `backends`/`frontends`/`databases` map 형태로 전환한다.
키가 식별자 역할을 하며, 복수 백엔드·프론트엔드·DB를 하나의 설정 파일에 선언할 수 있다.

## 변경 전후

### 변경 전
```yaml
backend:
  lang: go
  framework: gin
  module: github.com/geul-org/dummy-lesson
  middleware:
    - bearerAuth

frontend:
  lang: typescript
  framework: react
  bundler: vite
  name: dummy-lesson-web
```

### 변경 후
```yaml
backends:
  api:
    lang: go
    framework: gin
    module: github.com/geul-org/dummy-lesson
    middleware:
      - bearerAuth

frontends:
  web:
    lang: typescript
    framework: react
    bundler: vite
    name: dummy-lesson-web

databases:
  main:
    engine: postgresql
```

## 변경 항목

### A. projectconfig 구조체 (`artifacts/internal/projectconfig/`)

- `Backend` → `Backends map[string]Backend`
- `Frontend` → `Frontends map[string]Frontend`
- `Database` 타입 신규 + `Databases map[string]Database`
- `PrimaryBackend()` 헬퍼 — gen에서 단일 module path 필요 시 사용 (단일이면 그대로, 복수면 `api` 키 우선 → 알파벳순 첫 번째)
- `AllMiddleware()` 헬퍼 — 전체 backends에서 middleware 수집 (crosscheck용)
- Validate: backends 최소 1개 필수, 각 backend.module 필수

### B. validate 연동 (`artifacts/internal/orchestrator/validate.go`)

- Config summary 출력 변경: backends/frontends 키 목록 표시
- crosscheck에 middleware 전달 시 `AllMiddleware()` 사용

### C. gen 연동 (`artifacts/internal/orchestrator/gen.go`)

- `determineModulePath` — `PrimaryBackend().Module` 사용

### D. dummy fullend.yaml 업데이트

- `specs/dummy-lesson/fullend.yaml`
- `specs/dummy-study/fullend.yaml`

### E. manual-for-ai.md 업데이트

- fullend.yaml 예시를 복수 형태로 변경

### F. Phase028 계획서 업데이트

- fullend.yaml 포맷 예시를 복수 형태로 동기화

## 변경 파일 목록

| 파일 | 변경 |
|------|------|
| `artifacts/internal/projectconfig/projectconfig.go` | 복수 map 구조체 + 헬퍼 함수 |
| `artifacts/internal/orchestrator/validate.go` | Config summary + AllMiddleware |
| `artifacts/internal/orchestrator/gen.go` | PrimaryBackend 사용 |
| `specs/dummy-lesson/fullend.yaml` | 복수 형태 전환 |
| `specs/dummy-study/fullend.yaml` | 복수 형태 전환 |
| `artifacts/manual-for-ai.md` | 예시 업데이트 |
| `specs/plans/Phase028-ProjectConfig.md` | 예시 동기화 |

## 의존성

- Phase 028 완료

## 검증 방법

```bash
go build ./artifacts/cmd/fullend/
fullend validate specs/dummy-lesson
fullend validate specs/dummy-study
fullend gen specs/dummy-lesson /tmp/gen-lesson
head -1 /tmp/gen-lesson/backend/go.mod  # → module github.com/geul-org/dummy-lesson
```
