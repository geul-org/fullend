# Phase 031: fullend chain — Feature Chain 추출 ✅ 완료

## 목표

`fullend chain <operationId>` 명령으로 하나의 API 기능에 엮인 모든 SSOT의 파일:라인을 한 번에 출력한다.

## 배경

하나의 기능을 수정하려면 10개 SSOT를 관통하는 코드 경로를 파악해야 한다. 현재는 Grep을 수십 번 돌려야 하고, 레이어 간 연결(Go↔SQL↔Rego↔프론트엔드)은 정적 분석으로 잡히지 않는다.

crosscheck이 이미 SSOT 간 심볼 참조를 검증하고 있으므로, 심볼 테이블을 재활용하면 feature chain 추출이 가능하다.

## CLI

```bash
fullend chain <operationId> <specs-dir>

# 예시
fullend chain CreateOrder specs/gigbridge/
```

## 출력 형식

```
OpenAPI    specs/openapi.yaml:45        POST /api/orders
SSaC       specs/ssac/create_order.ssac  @get @auth @state @post @response
DDL        specs/ddl/003_orders.sql:1    CREATE TABLE orders
Rego       specs/authz/order.rego:12     allow if { input.action == "create" }
StateDiag  specs/states/order.md:3       [*] --> pending
FuncSpec   specs/funcs/billing.go:8      @func CalculateTotal
Gherkin    specs/scenarios/order.feature:5  Scenario: 재고 있는 상품 주문
STML       specs/stml/order_form.stml:1  <Form endpoint="POST /api/orders">
```

operationId에 연결되지 않는 SSOT는 출력하지 않는다.

## 탐색 경로

operationId를 시작점으로 심볼 테이블의 엣지를 타고 탐색:

```
operationId (OpenAPI)
├── path + method → SSaC 파일 (파서가 operationId↔SSaC 매핑)
│   ├── @get Model.Method → DDL 테이블 (심볼 테이블 Models → DDLTables)
│   ├── @auth action resource → Rego 정책 파일 (PolicySymbols)
│   ├── @state diagramID transition → Mermaid stateDiagram (StateSymbols)
│   ├── @call pkg.Func → Func Spec (FuncSymbols)
│   └── @publish topic → 큐 구독자 SSaC
├── OpenAPI response schema → STML endpoint 참조
└── Gherkin scenario → endpoint 참조 (ScenarioSymbols)
```

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `cmd/fullend/main.go` | `chain` 서브커맨드 추가 |
| `internal/orchestrator/chain.go` | 신규 — chain 오케스트레이션 |
| `internal/crosscheck/chain.go` | 신규 — 심볼 테이블에서 feature chain 그래프 추출 |
| `internal/reporter/chain.go` | 신규 — chain 출력 포매터 (파일:라인 형식) |

## 의존성

- crosscheck의 기존 심볼 테이블 (`SymbolTable`, `OperationSymbol` 등)
- 각 파서가 라인 번호 정보를 제공해야 함 (현재 일부 파서는 라인 번호 미보존 → 확인 필요)

## 검증

1. `fullend chain CreateOrder specs/gigbridge/` → 관련 SSOT 전체 출력
2. 존재하지 않는 operationId → 에러 메시지
3. 연결된 SSOT가 일부만 있는 경우 (Rego 없음 등) → 있는 것만 출력
4. `go test ./...` 통과
