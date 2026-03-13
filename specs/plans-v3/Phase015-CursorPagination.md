# Phase015: cursor pagination 코드젠 구현

## 목표

`x-pagination: { style: cursor }` 선언 시 cursor 기반 페이지네이션이 실제로 동작하도록 코드젠을 완성한다. 현재 offset만 동작하고 cursor는 파싱만 하고 SQL/응답 생성이 빠져 있다.

## 설계 원칙

**cursor = 고정 정렬 피드 전용**. 현업(Twitter, Slack, Stripe, GitHub)에서 cursor는 정렬 고정이 표준이다. 사용자가 런타임에 정렬을 바꿀 수 있으면 offset을 쓴다.

따라서:
- cursor 기본 정렬 = `id DESC`
- cursor 컬럼으로 UNIQUE 컬럼을 지정 가능 (e.g., `order_id`)
- cursor 값 = 마지막 행의 cursor 컬럼 값 (인코딩 없이 raw 문자열)
- `x-filter`는 cursor와 공존 가능 (WHERE 조건 추가일 뿐)

### cursor + x-sort 교차 검증 규칙

| 조건 | 결과 |
|------|------|
| cursor + x-sort 없음 | OK — `id DESC` 기본 |
| cursor + x-sort default가 DDL UNIQUE | OK — 해당 컬럼 DESC |
| cursor + x-sort default가 DDL UNIQUE 아님 | ERROR — 중복값 시 cursor 깨짐 |
| cursor + x-sort allowed 2개 이상 | ERROR — 런타임 정렬 전환 불가 |

UNIQUE 판별: DDL에서 `UNIQUE` 제약 또는 `PRIMARY KEY`가 있는 컬럼.

## 외부 의존성

### SSaC 수정지시서 020 (발송 완료)

현재 `ssac/validator/symbol.go`의 `Index` struct에 `IsUnique bool` 필드 없음, `DDLTable`에 `PrimaryKey []string` 없음.

- crosscheck (`checkCursorSort`)에서 UNIQUE 판별에 필요
- **코드젠은 SSaC 수정 없이 선행 구현 가능** — crosscheck만 SSaC 반영 후 추가

### fullend 아키텍처: OpenAPI → model_impl 데이터 흐름

현재 `generateModelImpls`가 `OpenAPIDoc`을 받지 않음. cursor 컬럼의 Go 필드명(e.g., `OrderID`)을 코드젠 시점에 알아야 함.

해법: `collectModelIncludes`와 동일 패턴으로 cursor 정보 사전 추출:

```go
// gluegen.go에서
cursorSpecs := collectCursorSpecs(input.OpenAPIDoc, input.ServiceFuncs)
// → map[operationId]string: "ID", "OrderID" 등
// generateModelImpls에 전달
```

## 실행 계획 (2단계)

### Phase 15-A: 코드젠 (SSaC 수정 없이 선행)

| 파일 | 변경 |
|------|------|
| `internal/gluegen/gluegen.go` | `collectCursorSpecs` 추가, `generateModelImpls`에 cursor 정보 전달 |
| `internal/gluegen/queryopts.go` | ParseQueryOpts cursor sort 결정 + BuildSelectQuery cursor WHERE 분기 |
| `internal/gluegen/model_impl.go` | `isCursorReturn` 분기 — COUNT 스킵, LIMIT+1, hasNext, nextCursor |
| `internal/gluegen/queryopts_test.go` | 신규 — cursor SQL 생성 테스트 |

### Phase 15-B: crosscheck (SSaC 수정지시서 020 반영 후)

| 파일 | 변경 |
|------|------|
| `internal/crosscheck/openapi_ddl.go` | `checkCursorSort` — allowed 개수 + UNIQUE 검증 |
| `internal/crosscheck/openapi_ddl_test.go` | cursor + x-sort 조합 테스트 |

## 현재 상태

### 동작하는 부분 (수정 불필요)

| 계층 | 상태 | 설명 |
|------|------|------|
| `pkg/pagination/cursor.go` | O | `Cursor[T]{Items, NextCursor, HasNext}` 제네릭 타입 |
| SSaC validator | O | `x-pagination style: cursor` ↔ `Cursor[T]` 반환 타입 일치 검증 |
| SSaC interface 코드젠 | O | `(*pagination.Cursor[Gig], error)` 리턴 시그니처 생성 |
| SSaC handler 코드젠 | O | `PaginationConfig{Style: "cursor", ...}` + x-sort 전달 확인 |
| `QueryOpts` struct | O | `Cursor string` 필드 이미 존재 |
| `ParseQueryOpts` | O | `cfg.Pagination.Style == "cursor"` → `c.Query("cursor")` 파싱 |

## cursor 동작 원리

### 기본 (x-sort 없음 → id DESC)

```
# 첫 페이지
GET /items?limit=20
→ SELECT * FROM items ORDER BY id DESC LIMIT 21
→ {items: [...20개], next_cursor: "42", has_next: true}

# 다음 페이지
GET /items?limit=20&cursor=42
→ SELECT * FROM items WHERE id < 42 ORDER BY id DESC LIMIT 21
→ {items: [...20개], next_cursor: "22", has_next: true}
```

### 커스텀 cursor 컬럼 (x-sort default = UNIQUE 컬럼)

```yaml
x-pagination:
  style: cursor
  defaultLimit: 20
  maxLimit: 100
x-sort:
  allowed: [order_id]
  default: order_id
  direction: desc
```

```
GET /items?limit=20&cursor=ORD-0042
→ SELECT * FROM items WHERE order_id < 'ORD-0042' ORDER BY order_id DESC LIMIT 21
```

cursor 컬럼 = `x-sort.default` (없으면 `id`). 런타임 정렬 전환 없음.

### has_next 판단

`LIMIT + 1`개를 가져와서 `LIMIT + 1`번째가 있으면 `has_next = true`. COUNT 쿼리 불필요.

### next_cursor 계산

결과의 마지막 행에서 cursor 컬럼 Go 필드 값 추출. 코드젠 시 cursor 컬럼의 Go 필드명을 OpenAPI x-sort에서 도출.

## Phase 15-A 상세

### 1. `internal/gluegen/gluegen.go` — cursor 정보 추출

```go
// collectCursorSpecs extracts cursor column Go field name per operation.
// Returns map[operationId]string ("ID" default, or PascalCase of x-sort.default).
func collectCursorSpecs(doc *openapi3.T, funcs []ssacparser.ServiceFunc) map[string]string {
    result := make(map[string]string)
    for _, pi := range doc.Paths.Map() {
        for _, op := range pi.Operations() {
            pag := getExtMap(op, "x-pagination")
            if pag == nil || getStr(pag, "style", "") != "cursor" {
                continue
            }
            cursorField := "ID"  // default
            if sortExt := getExtMap(op, "x-sort"); sortExt != nil {
                if def := getStr(sortExt, "default", ""); def != "" {
                    cursorField = strcase.ToGoPascal(def)
                }
            }
            result[op.OperationID] = cursorField
        }
    }
    return result
}
```

`generateModelImpls` 시그니처에 `cursorSpecs map[string]string` 추가.

### 2. `internal/gluegen/queryopts.go` — BuildSelectQuery 수정

`ParseQueryOpts`에서 cursor 스타일일 때 sort 컬럼 결정:

```go
if cfg.Pagination.Style == "cursor" {
    opts.Cursor = c.Query("cursor")
    if cfg.Sort != nil && cfg.Sort.Default != "" {
        opts.SortCol = cfg.Sort.Default
        opts.SortDir = cfg.Sort.Direction
    } else {
        opts.SortCol = "id"
        opts.SortDir = "desc"
    }
}
```

`BuildSelectQuery` cursor 분기:

```go
// Cursor-based pagination
if opts.Cursor != "" {
    cursorCol := opts.SortCol
    if cursorCol == "" {
        cursorCol = "id"
    }
    op := "<"  // DESC: 이전 값보다 작은 것
    if opts.SortDir == "asc" {
        op = ">"
    }
    if baseWhere == "" && len(args) == 0 {
        sql += " WHERE "
    } else {
        sql += " AND "
    }
    sql += fmt.Sprintf("%s %s $%d", cursorCol, op, argIdx)
    args = append(args, opts.Cursor)
    argIdx++
}

// Sort
if opts.SortCol != "" {
    dir := "ASC"
    if opts.SortDir == "desc" {
        dir = "DESC"
    }
    sql += fmt.Sprintf(" ORDER BY %s %s", opts.SortCol, dir)
}

// LIMIT
if opts.Limit > 0 {
    sql += fmt.Sprintf(" LIMIT $%d", argIdx)
    args = append(args, opts.Limit)
    argIdx++
}

// OFFSET — cursor 모드가 아닐 때만
if opts.Offset > 0 && opts.Cursor == "" {
    sql += fmt.Sprintf(" OFFSET $%d", argIdx)
    args = append(args, opts.Offset)
    argIdx++
}
```

### 3. `internal/gluegen/model_impl.go` — Cursor[T] 반환 분기

```go
isCursorReturn := strings.Contains(m.ReturnSig, "pagination.Cursor[")
```

`isList && isCursorReturn` 분기 (기존 `isList && isPageReturn` 아래 추가):
- COUNT 쿼리 스킵
- `requestedLimit := opts.Limit` 저장 후 `opts.Limit++`
- BuildSelectQuery + rows scan (기존 패턴 재사용)
- `hasNext := len(items) > requestedLimit`
- `if hasNext { items = items[:requestedLimit] }`
- cursor 필드 추출: `cursorSpecs[operationId]`로 Go 필드명 결정 (기본 `ID`)
- `nextCursor = fmt.Sprintf("%v", items[len(items)-1].{CursorField})`
- `return &pagination.Cursor[T]{Items: items, NextCursor: nextCursor, HasNext: hasNext}, nil`

### 4. hurl — 변경 없음

offset/cursor 모두 첫 요청은 `limit=2`만 보냄.

## Phase 15-B 상세 (SSaC 수정지시서 020 반영 후)

### 1. `internal/crosscheck/openapi_ddl.go` — checkCursorSort

```go
func checkCursorSort(op *openapi3.Operation, st *ssacvalidator.SymbolTable, ctx string) []CrossError {
    pag := getExtMap(op, "x-pagination")
    if pag == nil || getStr(pag, "style", "") != "cursor" {
        return nil
    }

    sortExt := getExtMap(op, "x-sort")
    if sortExt == nil {
        return nil  // cursor + sort 없음 → id DESC 기본, OK
    }

    var errs []CrossError

    // Rule 1: allowed가 2개 이상이면 ERROR
    allowed := getStrSlice(sortExt, "allowed")
    if len(allowed) > 1 {
        errs = append(errs, CrossError{
            Rule:    "x-pagination ↔ x-sort",
            Context: ctx,
            Message: "cursor 모드에서 x-sort allowed가 2개 이상 — 런타임 정렬 전환은 cursor를 깨뜨립니다",
            Level:   "ERROR",
        })
        return errs
    }

    // Rule 2: default 컬럼이 DDL UNIQUE인지 확인
    defaultCol := getStr(sortExt, "default", "")
    if defaultCol == "" && len(allowed) == 1 {
        defaultCol = allowed[0]
    }
    if defaultCol != "" {
        tableName := inferTableFromCtx(op, st)
        if !isUniqueColumn(defaultCol, tableName, st) {
            errs = append(errs, CrossError{
                Rule:    "x-pagination ↔ x-sort ↔ DDL",
                Context: ctx,
                Message: fmt.Sprintf("cursor 모드의 x-sort default %q — DDL에서 UNIQUE가 아닙니다. 중복값 시 cursor가 깨집니다", defaultCol),
                Level:   "ERROR",
            })
        }
    }

    return errs
}

// isUniqueColumn checks if a column is PRIMARY KEY or has UNIQUE constraint.
// Requires SSaC 수정지시서 020: Index.IsUnique + DDLTable.PrimaryKey
func isUniqueColumn(col, tableName string, st *ssacvalidator.SymbolTable) bool {
    table, ok := st.DDLTables[tableName]
    if !ok {
        return false
    }
    for _, pk := range table.PrimaryKey {
        if pk == col {
            return true
        }
    }
    for _, idx := range table.Indexes {
        if idx.IsUnique && len(idx.Columns) == 1 && idx.Columns[0] == col {
            return true
        }
    }
    return false
}
```

## 테스트

### Phase 15-A 테스트

`internal/gluegen/queryopts_test.go` (신규):
- `TestBuildSelectQuery_CursorFirstPage` — cursor 빈값, `ORDER BY id DESC LIMIT $N`
- `TestBuildSelectQuery_CursorNextPage` — cursor=42, `WHERE id < 42 ORDER BY id DESC LIMIT $N`
- `TestBuildSelectQuery_CursorCustomCol` — cursor=ORD-42, `WHERE order_id < 'ORD-42' ORDER BY order_id DESC LIMIT $N`
- `TestBuildSelectQuery_CursorWithFilter` — cursor + filter 조합
- `TestBuildSelectQuery_OffsetUnchanged` — 기존 offset 동작 유지 확인

통합: gigbridge에 cursor 전용 엔드포인트 임시 추가 → validate → gen → build 확인 후 되돌림.

### Phase 15-B 테스트 (SSaC 반영 후)

`internal/crosscheck/openapi_ddl_test.go` (신규):
- cursor + x-sort default UNIQUE → 정상
- cursor + x-sort default 비-UNIQUE → ERROR
- cursor + x-sort allowed 2개 → ERROR
- cursor + x-sort 없음 → 정상
- cursor + x-filter → 정상
- offset + x-sort → 정상 (기존 동작)

## 검증

```
go test ./internal/gluegen/...
go test ./internal/crosscheck/...
go test ./...
```
