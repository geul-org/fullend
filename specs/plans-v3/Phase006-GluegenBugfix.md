# Phase 006: Gluegen 버그 수정 — Handler DB, WithTx, TSX 디렉티브, List 중복 파싱 ✅ 완료

## 목표

`fullend gen`으로 생성된 코드가 `go build` 통과하도록 gluegen의 4가지 버그를 수정한다.

## 버그 목록

### BUG-A: Handler struct에 `DB *sql.DB` 누락

SSaC가 `handler.go`에 `DB *sql.DB` 필드를 포함하여 생성하지만, fullend의 `generateDomainHandler()`가 Handler struct를 자체적으로 재생성하면서 `DB` 필드를 포함하지 않는다.

결과: 핸들러 함수에서 `h.DB.BeginTx()` 호출 → 컴파일 에러.

**원인**: `domain.go:166` — Handler struct 생성 시 모델 필드만 순회하고, `DB *sql.DB`를 추가하지 않음.

**파급 범위 (3곳 수정 필요)**:
1. `generateDomainHandler()` — struct 필드 + import
2. `generateMainWithDomains()` — Handler 초기화 시 `DB: conn`
3. flat 모드의 `generateServerStruct()` — 동일 패턴 적용 (해당 시)

### BUG-B: `WithTx` 모델 구현 — 잘못된 코드 생성

`model_impl.go`의 `generateMethodFromIface`에서 `WithTx(tx *sql.Tx) GigModel` 메서드를 처리할 때, 일반 exec 패턴으로 빠져서 컴파일 불가 코드 생성.

현재 생성 결과:
```go
func (m *gigModelImpl) WithTx(tx *sql.Tx) GigModel {
    _, err := m.db.ExecContext(context.Background(), "-- TODO: WithTx", tx)
    return err  // ← GigModel이 아닌 error 반환
}
```

올바른 생성 결과:
```go
func (m *gigModelImpl) WithTx(tx *sql.Tx) GigModel {
    return &gigModelImpl{db: m.db, tx: tx}
}
```

**원인**: `model_impl.go` — `WithTx` 메서드에 대한 특수 처리 없음. 또한 `gigModelImpl` struct에 `tx *sql.Tx` 필드가 없고, 각 메서드가 `m.db` 대신 트랜잭션 컨텍스트를 사용하는 로직도 없음.

### BUG-C: TSX 파일에 `// fullend:gen` 디렉티브 미부착

Go 파일에는 `attach.go`로 디렉티브를 부착하지만, STML이 생성한 TSX 페이지 파일에는 디렉티브가 없다.

**원인**: `frontend.go` — TSX 파일 생성 후 디렉티브 부착 로직 없음. `attach.go`는 Go 파일(`.go`)만 처리.

### BUG-D: List 핸들러 — `ParseQueryOpts` 호출 후 중복 파싱

List 핸들러에서 `model.ParseQueryOpts(c, cfg)`와 SSaC가 생성한 수동 `c.Query("limit")` 파싱이 공존한다.

현재 생성 결과:
```go
opts := model.ParseQueryOpts(c, model.QueryOptsConfig{...})  // ← fullend가 삽입
if v := c.Query("limit"); v != "" {                           // ← SSaC가 생성 (중복)
    opts.Limit, _ = strconv.Atoi(v)
}
if v := c.Query("offset"); v != "" {                          // ← 중복
    opts.Offset, _ = strconv.Atoi(v)
}
allowedSort := map[string]bool{...}                           // ← 중복
if v := c.Query("sort"); allowedSort[v] { ... }               // ← 중복
```

**원인**: `gluegen.go:228-234`의 `transformSource()`가 SSaC 출력의 `QueryOpts{}`를 `model.ParseQueryOpts(c, cfg)`로 교체하지만, SSaC가 같이 생성한 수동 파싱 코드(`c.Query("limit")` 등)를 제거하지 않음.

`ParseQueryOpts`가 이미 limit/offset/sort/filter를 모두 처리하므로 수동 파싱 코드는 불필요할 뿐 아니라, `MaxLimit` 검증을 우회하는 문제도 있음.

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `internal/gluegen/domain.go` | 수정 — `generateDomainHandler()`: DB 필드 + import, `generateMainWithDomains()`: DB 초기화 |
| `internal/gluegen/model_impl.go` | 수정 — `WithTx` 특수 처리 + impl struct에 `tx` 필드 + `conn()` 헬퍼 |
| `internal/gluegen/attach.go` | 수정 — TSX 파일 디렉티브 부착 함수 추가 |
| `internal/gluegen/gluegen.go` | 수정 — TSX 디렉티브 부착 호출 + `transformSource`에서 중복 파싱 제거 |

## 상세 설계

### BUG-A: Handler struct에 DB 필드 추가

#### 1. `generateDomainHandler()` — struct 필드 + import

```go
func generateDomainHandler(serviceDir, domain string, serviceFuncs []ssacparser.ServiceFunc, modulePath string) error {
    domainDir := filepath.Join(serviceDir, domain)
    if err := os.MkdirAll(domainDir, 0755); err != nil {
        return err
    }

    models := collectModelsForDomain(serviceFuncs, domain)
    funcs := collectFuncsForDomain(serviceFuncs, domain)
    needsDB := domainNeedsDB(serviceFuncs, domain)

    var b strings.Builder
    b.WriteString(fmt.Sprintf("package %s\n\n", domain))

    // import 블록: DB 필요 시 database/sql 추가
    if needsDB {
        b.WriteString("import (\n")
        b.WriteString("\t\"database/sql\"\n\n")
        b.WriteString(fmt.Sprintf("\t\"%s/internal/model\"\n", modulePath))
        b.WriteString(")\n\n")
    } else {
        b.WriteString(fmt.Sprintf("import \"%s/internal/model\"\n\n", modulePath))
    }

    b.WriteString("// Handler handles requests for the " + domain + " domain.\n")
    b.WriteString("type Handler struct {\n")

    if needsDB {
        b.WriteString("\tDB *sql.DB\n")
    }

    for _, m := range models {
        fieldName := ucFirst(lcFirst(m) + "Model")
        b.WriteString(fmt.Sprintf("\t%s model.%sModel\n", fieldName, m))
    }
    // ... funcs, JWTSecret 등 기존 로직 ...

    b.WriteString("}\n")

    path := filepath.Join(domainDir, "handler.go")
    return os.WriteFile(path, []byte(b.String()), 0644)
}
```

#### 2. `domainNeedsDB()` 헬퍼

```go
// domainNeedsDB checks if any service function in the domain has write sequences.
func domainNeedsDB(serviceFuncs []ssacparser.ServiceFunc, domain string) bool {
    for _, fn := range serviceFuncs {
        if fn.Domain != domain {
            continue
        }
        for _, seq := range fn.Sequences {
            switch seq.Type {
            case "post", "put", "delete":
                return true
            }
        }
    }
    return false
}
```

#### 3. `generateMainWithDomains()` — Handler 초기화

`domain.go:401-411`에서 도메인별 Handler 초기화 시 `DB: conn` 추가:

```go
for _, domain := range domains {
    domainModels := collectModelsForDomain(serviceFuncs, domain)
    fieldName := ucFirst(domain)

    var handlerLines []string

    // DB 필드 초기화 (쓰기 시퀀스가 있는 도메인)
    if domainNeedsDB(serviceFuncs, domain) {
        handlerLines = append(handlerLines, "\t\t\tDB: conn,")
    }

    for _, m := range domainModels {
        mFieldName := ucFirst(lcFirst(m) + "Model")
        handlerLines = append(handlerLines, fmt.Sprintf("\t\t\t%s: model.New%sModel(conn),", mFieldName, m))
    }
    if domainNeedsJWTSecret(serviceFuncs, domain) {
        handlerLines = append(handlerLines, "\t\t\tJWTSecret: *jwtSecret,")
    }
    // ...
}
```

### BUG-B: WithTx 구현 생성

#### 1. impl struct에 tx 필드 추가

`generateModelFile`의 struct 정의 부분:

```go
b.WriteString(fmt.Sprintf("type %s struct {\n", implName))
b.WriteString("\tdb *sql.DB\n")
b.WriteString("\ttx *sql.Tx\n")  // 추가
b.WriteString("}\n\n")
```

#### 2. conn() 헬퍼 생성

struct 정의 직후에 생성:

```go
// conn() returns tx if set, otherwise db.
b.WriteString(fmt.Sprintf("func (m *%s) conn() interface {\n", implName))
b.WriteString("\tExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)\n")
b.WriteString("\tQueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)\n")
b.WriteString("\tQueryRowContext(ctx context.Context, query string, args ...any) *sql.Row\n")
b.WriteString("} {\n")
b.WriteString("\tif m.tx != nil {\n")
b.WriteString("\t\treturn m.tx\n")
b.WriteString("\t}\n")
b.WriteString("\treturn m.db\n")
b.WriteString("}\n\n")
```

#### 3. 기존 메서드의 m.db → m.conn()

`generateMethodFromIface` 내 모든 `m.db.ExecContext`, `m.db.QueryRowContext`, `m.db.QueryContext` → `m.conn().ExecContext` 등으로 교체.

구현 방법: 문자열 생성 시 `m.db` 대신 `m.conn()` 사용:
- `m.db.QueryRowContext(` → `m.conn().QueryRowContext(`
- `m.db.ExecContext(` → `m.conn().ExecContext(`
- `m.db.QueryContext(` → `m.conn().QueryContext(`

#### 4. WithTx 메서드 특수 처리

`generateMethodFromIface`의 최상단에서 `WithTx` 감지:

```go
func generateMethodFromIface(b *strings.Builder, implName, modelName string, m ifaceMethod, ...) {
    // WithTx 특수 처리
    if m.Name == "WithTx" {
        b.WriteString(fmt.Sprintf("func (m *%s) WithTx(tx *sql.Tx) %sModel {\n", implName, modelName))
        b.WriteString(fmt.Sprintf("\treturn &%s{db: m.db, tx: tx}\n", implName))
        b.WriteString("}\n")
        return
    }
    // ... 기존 로직 ...
}
```

### BUG-C: TSX 디렉티브 부착

`attach.go`에 TSX 파일용 함수 추가:

```go
// attachTSXDirectives scans pages/*.tsx files and injects // fullend: directive.
func attachTSXDirectives(artifactsDir string) error {
    pagesDir := filepath.Join(artifactsDir, "frontend", "src", "pages")
    entries, err := os.ReadDir(pagesDir)
    if err != nil {
        return nil // pages 디렉토리 없으면 스킵
    }
    for _, entry := range entries {
        if !strings.HasSuffix(entry.Name(), ".tsx") {
            continue
        }
        path := filepath.Join(pagesDir, entry.Name())
        src, err := os.ReadFile(path)
        if err != nil {
            continue
        }
        content := string(src)

        // 이미 디렉티브가 있으면 스킵
        if strings.Contains(content, "fullend:") {
            continue
        }

        // SSOT 경로: STML 파일명에서 파생
        stmlName := strings.TrimSuffix(entry.Name(), ".tsx") + ".html"
        ssotPath := "frontend/" + stmlName
        hash := contract.Hash7(content) // 파일 내용 해싱

        d := &contract.Directive{Ownership: "gen", SSOT: ssotPath, Contract: hash}
        newContent := d.StringJS() + "\n" + content
        os.WriteFile(path, []byte(newContent), 0644)
    }
    return nil
}
```

`gluegen.go`의 `Generate()` — `generateFrontendSetup()` 직후:

```go
if err := generateFrontendSetup(input.ArtifactsDir, ...); err != nil {
    return fmt.Errorf("frontend setup: %w", err)
}
if err := attachTSXDirectives(input.ArtifactsDir); err != nil {
    return fmt.Errorf("tsx directives: %w", err)
}
```

### BUG-D: List 핸들러 중복 파싱 제거

`transformSource()`에서 `QueryOpts{}`를 `ParseQueryOpts(c, cfg)`로 교체할 때, SSaC가 생성한 수동 파싱 코드를 제거한다.

SSaC가 생성하는 수동 파싱 패턴:
```go
if v := c.Query("limit"); v != "" {
    opts.Limit, _ = strconv.Atoi(v)
}
if v := c.Query("offset"); v != "" {
    opts.Offset, _ = strconv.Atoi(v)
}
allowedSort := map[string]bool{...}
if v := c.Query("sort"); allowedSort[v] {
    opts.SortCol = v
}
if v := c.Query("direction"); v == "asc" || v == "desc" {
    opts.SortDir = v
}
```

`gluegen.go`의 `transformSource()`에서 `ParseQueryOpts` 삽입 직후, 이 패턴들을 정규식으로 제거:

```go
if strings.Contains(src, "QueryOpts{}") {
    funcName := extractFuncName(src)
    if cfg, ok := xConfigs[funcName]; ok {
        src = strings.ReplaceAll(src, "QueryOpts{}", "model.ParseQueryOpts(c, "+cfg+")")
        // SSaC가 생성한 수동 파싱 코드 제거 — ParseQueryOpts가 이미 처리
        src = removeManualQueryParsing(src)
    } else {
        src = strings.ReplaceAll(src, "QueryOpts{}", "model.QueryOpts{}")
    }
}
```

```go
// removeManualQueryParsing removes SSaC-generated c.Query("limit/offset/sort/direction") blocks
// that are redundant when ParseQueryOpts is used.
func removeManualQueryParsing(src string) string {
    patterns := []string{
        // limit 블록
        `\tif v := c\.Query\("limit"\); v != "" \{\n\t\topts\.Limit, _ = strconv\.Atoi\(v\)\n\t\}\n`,
        // offset 블록
        `\tif v := c\.Query\("offset"\); v != "" \{\n\t\topts\.Offset, _ = strconv\.Atoi\(v\)\n\t\}\n`,
        // allowedSort + sort + direction 블록
        `\tallowedSort := map\[string\]bool\{[^}]+\}\n\tif v := c\.Query\("sort"\); allowedSort\[v\] \{\n\t\topts\.SortCol = v\n\t\}\n\tif v := c\.Query\("direction"\); v == "asc" \|\| v == "desc" \{\n\t\topts\.SortDir = v\n\t\}\n`,
    }
    for _, p := range patterns {
        re := regexp.MustCompile(p)
        src = re.ReplaceAllString(src, "")
    }
    return src
}
```

## 의존성

- Phase 002 (`internal/gluegen/attach.go`)
- Phase 001 (`internal/contract` — hash, directive)
- SSaC `수정지시서003` (auto-transaction — `h.DB.BeginTx`, `WithTx`)
- SSaC `수정지시서004` (authz.Claims 제거 — 완료)
- SSaC `수정지시서005` (`@call auth.*` 기본 에러 코드 401 — 대기 중)

## 검증

```bash
fullend gen specs/gigbridge/ artifacts/gigbridge/
cd artifacts/gigbridge/backend && go build ./...
```

1. **BUG-A**: Handler struct에 `DB *sql.DB` 필드 존재
2. **BUG-A**: handler.go에 `"database/sql"` import 존재
3. **BUG-A**: main.go에서 `DB: conn` 초기화 확인
4. **BUG-B**: impl struct에 `tx *sql.Tx` 필드 존재
5. **BUG-B**: `conn()` 헬퍼가 tx/db 분기
6. **BUG-B**: `WithTx` 메서드가 새 impl 인스턴스 반환
7. **BUG-B**: 모든 메서드가 `m.conn()` 사용
8. **BUG-C**: TSX 페이지 파일 첫 줄에 `// fullend:gen ssot=... contract=...` 존재
9. **BUG-D**: List 핸들러에 `c.Query("limit")` 등 수동 파싱 없음
10. **BUG-D**: `ParseQueryOpts`만으로 pagination/sort/filter 처리
11. **전체**: `go build ./...` 통과
12. **전체**: `go test ./...` 통과
