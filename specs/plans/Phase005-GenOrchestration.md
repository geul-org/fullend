# ✅ 완료 — Phase 5 — gen 오케스트레이션

## 목표
`fullend gen <specs-dir> <artifacts-dir>` 실행 시 validate 통과 후 전체 코드를 산출한다.

## 선행 조건
Phase 4 완료

## 변경 파일

| 파일 | 작업 |
|---|---|
| `artifacts/internal/orchestrator/gen.go` | 생성. 코드젠 순서 제어 |
| `artifacts/internal/orchestrator/exec.go` | 생성. 외부 도구 exec 래퍼 |
| `artifacts/cmd/fullend/main.go` | 수정. gen 서브커맨드 연결 |

## 실행 순서

```
1. fullend validate <specs-dir>     ← 먼저 검증, 실패 시 중단
2. sqlc generate                    ← exec 호출 (DB 모델 Go struct)
3. ssac generator.Generate()        ← 라이브러리 호출 (서비스 함수)
4. ssac generator.GenerateModelInterfaces() ← 라이브러리 호출 (Model interface)
5. stml generator.Generate()        ← 라이브러리 호출 (React TSX)
6. terraform fmt <specs>/terraform/ ← exec 호출 (HCL 포맷팅)
```

## exec 호출 규칙

- 외부 도구가 설치되지 않은 경우: 해당 단계 skip + WARNING 출력
- exec 실패 시: stderr 캡처하여 에러 메시지에 포함
- 타임아웃: 30초

## 출력 디렉토리 구조

```
<artifacts-dir>/
├── backend/
│   ├── service/          ← ssac gen 산출
│   └── model/            ← ssac GenerateModelInterfaces 산출
├── frontend/             ← stml gen 산출
└── db/                   ← sqlc generate 산출
```

## 의존성

| 패키지 | 호출 |
|---|---|
| `github.com/geul-org/ssac/generator` | `Generate()`, `GenerateModelInterfaces()` |
| `github.com/geul-org/stml/generator` | `Generate()` |
| `os/exec` | `sqlc generate`, `terraform fmt` |

## 검증 방법

- validate 실패 → gen 중단, 에러 출력
- validate 통과 → 각 단계 순차 실행, artifacts 디렉토리에 파일 생성 확인
- sqlc 미설치 → skip WARNING + 나머지 단계 계속
- terraform 미설치 → skip WARNING
