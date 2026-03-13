# Phase 007: 민감 필드 보호 + rows.Err() 누락 수정 ✅ 완료

## 목표

BUG006(password_hash JSON 노출)과 BUG009(rows.Err() 미확인)를 수정한다.

## 버그 목록

### BUG006: User 구조체에 password_hash가 json 태그로 노출

**심각도**: HIGH — 정보 유출

DDL 컬럼을 그대로 `json:"column_name"` 태그로 매핑하여 `password_hash` 등 민감 필드가 API 응답에 노출된다.

**설계 결정**: 2단계 방어

1. **DDL `@sensitive` 어노테이션** — `-- @sensitive` 주석이 붙은 컬럼은 `json:"-"` 태그 생성 (확정 차단)
2. **crosscheck WARNING** — 컬럼명에 `password`, `secret`, `hash`, `token` 패턴이 있는데 `@sensitive` 미선언 시 경고

DDL 예시:
```sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,  -- @sensitive
    name VARCHAR(255) NOT NULL
);
```

### BUG009: model List 메서드에서 rows.Err() 미확인

**심각도**: MEDIUM

`for rows.Next()` 루프 종료 후 `rows.Err()` 체크 없이 바로 `return items, nil`. DB 이터레이션 중 에러 발생 시 부분 결과를 정상 반환하게 된다.

해당 위치: `model_impl.go`의 `generateMethodFromIface()` 내 2곳
- `isList` 분기 (line 463): `for rows.Next()` → include 로딩 → 바로 return
- `isSliceReturn` 분기 (line 493): `for rows.Next()` → 바로 `return items, nil`

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `internal/gluegen/model_impl.go` | 수정 — `ddlColumn`에 `Sensitive bool` 추가, `parseDDLFiles()`에서 `@sensitive` 파싱, `generateTypesFile()`에서 `json:"-"` 생성, `generateMethodFromIface()`에서 `rows.Err()` 추가 |
| `internal/crosscheck/crosscheck.go` | 수정 — 민감 컬럼명 패턴 경고 규칙 추가 |

## 상세 설계

### BUG006-A: DDL 파서에 `@sensitive` 파싱 추가

`model_impl.go:19` — `ddlColumn` struct에 필드 추가:

```go
type ddlColumn struct {
    Name      string
    GoName    string
    GoType    string
    FKTable   string
    NotNull   bool
    Sensitive bool   // @sensitive 어노테이션 → json:"-"
}
```

`model_impl.go:628` — `parseDDLFiles()` 컬럼 파싱 루프에서 `-- @sensitive` 감지:

```go
for _, line := range lines {
    colMatch := colRe.FindStringSubmatch(line)
    if colMatch == nil {
        continue
    }
    // ... 기존 로직 ...

    sensitive := strings.Contains(line, "@sensitive")

    table.Columns = append(table.Columns, ddlColumn{
        Name:      colName,
        GoName:    snakeToGo(colName),
        GoType:    sqlTypeToGo(sqlType),
        FKTable:   fkTable,
        NotNull:   notNull,
        Sensitive: sensitive,
    })
}
```

### BUG006-B: types.go 생성 시 `json:"-"` 적용

`model_impl.go:216` — `generateTypesFile()` 컬럼 출력부:

```go
for _, col := range t.Columns {
    jsonTag := col.Name
    if col.Sensitive {
        jsonTag = "-"
    }
    b.WriteString(fmt.Sprintf("\t%-12s %s `json:\"%s\"`\n", col.GoName, col.GoType, jsonTag))
}
```

### BUG006-C: crosscheck에 민감 컬럼명 패턴 경고

`crosscheck.go`에 DDL 기반 검사 추가. `parseDDLFiles()`를 crosscheck에서도 호출하거나, 기존 DDL 파싱 결과를 받아서 경고:

```go
// checkSensitiveColumns warns when column names match sensitive patterns but lack @sensitive annotation.
func checkSensitiveColumns(tables map[string]*ddlTable) []Result {
    var results []Result
    patterns := []string{"password", "secret", "hash", "token"}

    for _, t := range tables {
        for _, col := range t.Columns {
            if col.Sensitive {
                continue // 이미 @sensitive 선언됨
            }
            lower := strings.ToLower(col.Name)
            for _, p := range patterns {
                if strings.Contains(lower, p) {
                    results = append(results, Result{
                        Level:   "WARNING",
                        Source:  fmt.Sprintf("db/%s.sql", t.TableName),
                        Message: fmt.Sprintf("column %q matches sensitive pattern %q but has no @sensitive annotation — consider adding -- @sensitive to exclude from JSON", col.Name, p),
                    })
                    break
                }
            }
        }
    }
    return results
}
```

### BUG009: rows.Err() 체크 추가

`model_impl.go` — `generateMethodFromIface()` 내 2곳에 `rows.Err()` 체크 삽입.

#### 1. `isList` 분기 (line 463 이후, include 로딩 전)

현재:
```go
b.WriteString("\t}\n")                           // end for rows.Next()
// Include loading ...
```

수정:
```go
b.WriteString("\t}\n")                           // end for rows.Next()
b.WriteString("\tif err := rows.Err(); err != nil {\n")
b.WriteString("\t\treturn nil, err\n")
b.WriteString("\t}\n")
// Include loading ...
```

#### 2. `isSliceReturn` 분기 (line 493)

현재:
```go
b.WriteString("\t}\n")              // end for rows.Next()
b.WriteString("\treturn items, nil\n")
```

수정:
```go
b.WriteString("\t}\n")              // end for rows.Next()
b.WriteString("\tif err := rows.Err(); err != nil {\n")
b.WriteString("\t\treturn nil, err\n")
b.WriteString("\t}\n")
b.WriteString("\treturn items, nil\n")
```

## 의존성

- BUG006: DDL 파일에 `-- @sensitive` 주석 추가 필요 (사용자 프로젝트 DDL)
- BUG009: 독립적, 즉시 수정 가능

## 검증

```bash
go test ./internal/gluegen/... ./internal/crosscheck/...
fullend gen specs/gigbridge/ artifacts/gigbridge/
```

1. **BUG006**: `artifacts/gigbridge/backend/internal/model/types.go`에서 `password_hash` 컬럼이 `json:"-"` 태그인지 확인
2. **BUG006**: `@sensitive` 없이 `password_hash` 컬럼이 있으면 crosscheck WARNING 출력 확인
3. **BUG009**: 생성된 model 파일의 List/ListBy 메서드에서 `rows.Err()` 체크 존재 확인
4. **전체**: `go build ./...` 통과
5. **전체**: `go test ./...` 통과
