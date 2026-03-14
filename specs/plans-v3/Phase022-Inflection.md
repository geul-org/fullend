# Phase022: `jinzhu/inflection` 기반 단수화/복수화 정규화 ✅ 완료

## 목표

hand-rolled 단수화/복수화 로직을 `github.com/jinzhu/inflection` 라이브러리로 전량 교체하여 영어 불규칙 변환을 정확하게 처리한다.

## 발견 경위

`files/dummy-zenflow-report2.md` — zenflow 벤치마크 중 발견. Phase023, Phase024의 선행 조건.

## 현황

fullend 전체에 hand-rolled 단수화/복수화가 **8개 파일, 21곳** 산재:

### 단수화 (3곳)

| 위치 | 현재 로직 | 실패 케이스 |
|---|---|---|
| `validator/symbol.go:327-339` sqlFileToModel | `"ies"→"y"`, `"sses"→-"es"`, `"s"→-"s"` | "statuses"→"statuse", "people"→"peopl" |
| `gluegen/model_impl.go:902-922` singularize | `"sses"→-"es"`, `"xes"→-"es"`, `"ies"→"y"`, `"s"→-"s"` | 동일 |
| `crosscheck/ddl_coverage.go:85-97` tableToModel | `"ies"→"y"`, `"sses"→-"es"`, `"xes"→-"es"`, `"s"→-"s"` | 동일 |

### 복수화 — `toSnakeCase(x) + "s"` 패턴 (14곳)

| 파일 | 라인 | 컨텍스트 |
|---|---|---|
| `generator/go_interface.go` | :193 | `refTable := toSnakeCase(a.Source) + "s"` — source 테이블 조회 |
| `generator/go_interface.go` | :205 | `tableName := toSnakeCase(modelName) + "s"` — 모델 테이블 |
| `generator/go_interface.go` | :215 | `refTable := toSnakeCase(refModel) + "s"` — {Model}ID FK 조회 |
| `generator/go_interface.go` | :250 | `refTable := toSnakeCase(source) + "s"` — input source 테이블 |
| `generator/go_interface.go` | :262 | `tableName := toSnakeCase(modelName) + "s"` — input 모델 테이블 |
| `generator/go_interface.go` | :272 | `refTable := toSnakeCase(refModel) + "s"` — input {Model}ID FK |
| `generator/go_interface.go` | :296 | `tableName := toSnakeCase(modelName) + "s"` — key param 모델 테이블 |
| `generator/go_interface.go` | :306 | `refTable := toSnakeCase(refModel) + "s"` — key param {Model}ID FK |
| `validator/validator.go` | :643 | `tableName := toSnakeCase(modelName) + "s"` — 필드 타입 resolve |
| `validator/validator.go` | :653 | `refTable := toSnakeCase(refModel) + "s"` — {Model}ID FK fallback |
| `validator/validator.go` | :746 | `tableName := toSnakeCase(parts[0]) + "s"` — 모델명→테이블 유추 |
| `orchestrator/chain.go` | :204 | `tableName := strings.ToLower(modelName) + "s"` — chain DDL 매칭 |
| `orchestrator/chain.go` | :290 | `tableName := toSnakeCase(parts[0]) + "s"` — chain sequence 테이블 |
| `gluegen/hurl.go` | :314 | `resource := d.ID + "s"` — diagram ID 복수화 |

### 복수화 — hand-rolled 함수 (2곳)

| 파일 | 라인 | 현재 로직 |
|---|---|---|
| `crosscheck/ssac_ddl.go` | :140-146 | `modelToTable`: `snake + "s"` (ends in "s" skip) |
| `crosscheck/states.go` | :194-208 | `diagramIDToTable`: switch on last char (`y→ies`, `s/x→es`, `default→s`) |

### 복수화 — 후보 배열 (1곳)

| 파일 | 라인 | 현재 로직 |
|---|---|---|
| `crosscheck/openapi_ddl.go` | :317-321 | `resolveTableName`: 4개 후보 (`lower+"s"`, `lower`, `snake+"s"`, `snake`) |

## 해결: `github.com/jinzhu/inflection` 도입

mason(`~/.clari/repos/mason/artifacts/backend/clerk.go`)이 이미 사용 중. GORM 등 Go 생태계 표준 라이브러리.

```go
import "github.com/jinzhu/inflection"

inflection.Plural("status")      // "statuses"
inflection.Plural("category")    // "categories"
inflection.Plural("person")      // "people"
inflection.Singular("statuses")  // "status"
inflection.Singular("categories") // "category"
inflection.Singular("people")    // "person"
```

## 변경 상세

### 1. `internal/ssac/validator/symbol.go:327-339` — `sqlFileToModel` 단수화

```go
// 변경 전 (hand-rolled)
func sqlFileToModel(filename string) string {
    name := strings.TrimSuffix(filename, ".sql")
    if strings.HasSuffix(name, "ies") { name = name[:len(name)-3] + "y" }
    else if strings.HasSuffix(name, "sses") || strings.HasSuffix(name, "xes") { name = name[:len(name)-2] }
    else if strings.HasSuffix(name, "s") { name = name[:len(name)-1] }
    return strings.ToUpper(name[:1]) + name[1:]
}

// 변경 후
func sqlFileToModel(filename string) string {
    name := strings.TrimSuffix(filename, ".sql")
    singular := inflection.Singular(name)
    return strings.ToUpper(singular[:1]) + singular[1:]
}
```

### 2. `internal/ssac/generator/go_interface.go` — 8곳 일괄 교체

:193, :205, :215, :250, :262, :272, :296, :306 — 모두 동일 패턴:
```go
// 변경 전
xxx := toSnakeCase(yyy) + "s"

// 변경 후
xxx := inflection.Plural(toSnakeCase(yyy))
```

### 3. `internal/ssac/validator/validator.go` — 3곳 일괄 교체

:643, :653, :746 — 동일 패턴:
```go
// 변경 전
tableName := toSnakeCase(xxx) + "s"

// 변경 후
tableName := inflection.Plural(toSnakeCase(xxx))
```

### 4. `internal/orchestrator/chain.go` — 2곳

:204:
```go
// 변경 전
tableName := strings.ToLower(modelName) + "s"

// 변경 후
tableName := inflection.Plural(strings.ToLower(modelName))
```

:290:
```go
// 변경 전
tableName := toSnakeCase(parts[0]) + "s"

// 변경 후
tableName := inflection.Plural(toSnakeCase(parts[0]))
```

### 5. `internal/crosscheck/ssac_ddl.go:140-146` — `modelToTable` 교체

```go
// 변경 전
func modelToTable(model string) string {
    snake := pascalToSnake(model)
    if strings.HasSuffix(snake, "s") { return snake }
    return snake + "s"
}

// 변경 후
func modelToTable(model string) string {
    return inflection.Plural(pascalToSnake(model))
}
```

### 6. `internal/crosscheck/states.go:194-208` — `diagramIDToTable` 교체

```go
// 변경 전
func diagramIDToTable(id string) string {
    if len(id) == 0 { return id }
    last := id[len(id)-1]
    switch {
    case last == 'y': return id[:len(id)-1] + "ies"
    case last == 's' || last == 'x': return id + "es"
    default: return id + "s"
    }
}

// 변경 후
func diagramIDToTable(id string) string {
    return inflection.Plural(id)
}
```

### 7. `internal/crosscheck/openapi_ddl.go:317-321` — `resolveTableName` 후보 교체

```go
// 변경 전
candidates := []string{
    strings.ToLower(resource) + "s",
    strings.ToLower(resource),
    pascalToSnake(resource) + "s",
    pascalToSnake(resource),
}

// 변경 후
snake := pascalToSnake(resource)
candidates := []string{
    inflection.Plural(strings.ToLower(resource)),
    strings.ToLower(resource),
    inflection.Plural(snake),
    snake,
}
```

### 8. `internal/gluegen/model_impl.go:902-922` — `singularize` 교체

```go
// 변경 전
func singularize(name string) string {
    lower := strings.ToLower(name)
    var singular string
    switch {
    case strings.HasSuffix(lower, "sses"): singular = lower[:len(lower)-2]
    case strings.HasSuffix(lower, "xes"):  singular = lower[:len(lower)-2]
    case strings.HasSuffix(lower, "ies"):  singular = lower[:len(lower)-3] + "y"
    case strings.HasSuffix(lower, "s"):    singular = lower[:len(lower)-1]
    default: singular = lower
    }
    if len(singular) == 0 { return name }
    return strcase.ToGoPascal(singular)
}

// 변경 후
func singularize(name string) string {
    singular := inflection.Singular(strings.ToLower(name))
    if len(singular) == 0 { return name }
    return strcase.ToGoPascal(singular)
}
```
호출부 (:746, :810, :1014) — 변경 불필요 (시그니처 동일).

### 9. `internal/crosscheck/ddl_coverage.go:85-97` — `tableToModel` 교체

```go
// 변경 전
func tableToModel(table string) string {
    name := table
    if strings.HasSuffix(name, "ies") { name = name[:len(name)-3] + "y" }
    else if strings.HasSuffix(name, "sses") || strings.HasSuffix(name, "xes") { name = name[:len(name)-2] }
    else if strings.HasSuffix(name, "s") { name = name[:len(name)-1] }
    return snakeToPascal(name)
}

// 변경 후
func tableToModel(table string) string {
    return snakeToPascal(inflection.Singular(table))
}
```

### 10. `internal/gluegen/hurl.go:314` — diagram ID 복수화

```go
// 변경 전
resource := d.ID + "s"

// 변경 후
resource := inflection.Plural(d.ID)
```

## 변경 파일

- `go.mod` (`go get github.com/jinzhu/inflection`)
- `internal/ssac/validator/symbol.go` (:327-339 sqlFileToModel)
- `internal/ssac/generator/go_interface.go` (:193, :205, :215, :250, :262, :272, :296, :306)
- `internal/ssac/validator/validator.go` (:643, :653, :746)
- `internal/orchestrator/chain.go` (:204, :290)
- `internal/crosscheck/ssac_ddl.go` (:140-146 modelToTable)
- `internal/crosscheck/states.go` (:194-208 diagramIDToTable)
- `internal/crosscheck/openapi_ddl.go` (:317-321 resolveTableName)
- `internal/gluegen/model_impl.go` (:902-922 singularize)
- `internal/crosscheck/ddl_coverage.go` (:85-97 tableToModel)
- `internal/gluegen/hurl.go` (:314)

## 검증 방법

1. `go test ./internal/ssac/validator/...` — sqlFileToModel 관련 기존 테스트 통과
2. `go test ./internal/ssac/generator/...` — go_interface 관련 기존 테스트 통과
3. `go test ./internal/crosscheck/...` — modelToTable, diagramIDToTable, resolveTableName 관련 기존 테스트 통과
4. `go test ./internal/gluegen/...` — hurl, model_impl 관련 기존 테스트 통과
5. `go test ./internal/orchestrator/...` — chain 관련 기존 테스트 통과
6. 동작 호환성 — 기존 정규 복수형(workflow→workflows)은 결과 동일

## whyso 이력

| 파일 | 이력 |
|---|---|
| `internal/ssac/validator/symbol.go` | 없음 (최초 생성 후 미수정) |
| `internal/ssac/generator/go_interface.go` | 없음 (최초 생성 후 미수정) |
| `internal/ssac/validator/validator.go` | 없음 (최초 생성 후 미수정) |
| `internal/orchestrator/chain.go` | 없음 (최초 생성 후 미수정) |
| `internal/crosscheck/ssac_ddl.go` | 2026-03-10 생성, 수정 이력 없음 |
| `internal/crosscheck/states.go` | 2026-03-10 생성, 수정 이력 없음 |
| `internal/crosscheck/openapi_ddl.go` | 2026-03-10 생성, 수정 이력 없음 |
| `internal/gluegen/model_impl.go` | 없음 (최초 생성 후 미수정) |
| `internal/crosscheck/ddl_coverage.go` | 2026-03-10 생성, 수정 이력 없음 |
| `internal/gluegen/hurl.go` | 2026-03-10 생성, 수정 이력 없음 |
