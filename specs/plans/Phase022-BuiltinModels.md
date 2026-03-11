# ✅ Phase 022: fullend 내장 모델 (Session, Cache, File)

## 배경

Phase 021에서 패키지 접두사 기반 모델 판단 규칙이 확립됨.
접두사가 있으면 해당 패키지 경로에서 Go interface를 파싱하여 교차 검증.
fullend/pkg든 사용자 커스텀이든 동일한 규칙.

Phase 022에서는 fullend가 기본 제공하는 내장 모델 패키지를 구현.
사용은 선택 — 사용자가 자기 패키지로 대체 가능.

External 모델은 코드젠 도구 성격이 다르므로 Phase 023으로 분리.

## 대상

| 종류 | 패키지 | 구현체 |
|---|---|---|
| Session | `pkg/session` | PostgreSQL, Memory |
| Cache | `pkg/cache` | PostgreSQL, Memory |
| File | `pkg/file` | S3, LocalFile |

## 설계

### 1. Session 모델 (pkg/session)

key-value + TTL 구조. 사용자 귀속 상태 (로그인, 장바구니 등).

**인터페이스:**
```go
type SessionModel interface {
    Set(ctx context.Context, key string, value any, ttl time.Duration) error
    Get(ctx context.Context, key string) (string, error)
    Delete(ctx context.Context, key string) error
}
```

**SSaC에서의 사용:**
```go
// @get Session session = session.Session.Get({key: request.Token})
// @post Session result = session.Session.Set({key: userID, value: userData, ttl: 3600})
// @delete Session result = session.Session.Delete({key: request.Token})
```

SSaC 파라미터명은 Go interface 파라미터명과 **정확히 일치**해야 함. `{token: ...}`은 ERROR — interface에 `token` 파라미터 없음.
`ctx context.Context`는 gin 프레임워크가 제공하므로 SSaC에서 명시 불필요. interface에 `ctx`가 없으면 ERROR.

**fullend.yaml 설정:**
```yaml
session:
  backend: postgres  # postgres | memory
```

### 2. Cache 모델 (pkg/cache)

Session과 동일한 key-value + TTL 구조. 목적만 다름 (데이터 효율화).

**인터페이스:**
```go
type CacheModel interface {
    Set(ctx context.Context, key string, value any, ttl time.Duration) error
    Get(ctx context.Context, key string) (string, error)
    Delete(ctx context.Context, key string) error
}
```

**SSaC에서의 사용:**
```go
// @get CachedGig gig = cache.Cache.Get({key: "gig:" + request.ID})
// @post CacheResult result = cache.Cache.Set({key: "gig:" + gig.ID, value: gig, TTL: 300})
// @delete CacheResult result = cache.Cache.Delete({key: "gig:" + request.ID})
```

**fullend.yaml 설정:**
```yaml
cache:
  backend: postgres  # postgres | memory
```

### 3. File 모델 (pkg/file)

key(경로) → 파일 바이너리. 2종 구현체 기본 제공.

**인터페이스:**
```go
type FileModel interface {
    Upload(ctx context.Context, key string, body io.Reader) error
    Download(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
}
```

**구현체:**
- `LocalFileModel` — 로컬 파일시스템 (`os.Create`, `os.Open`)
- `S3Model` — AWS S3 (`s3.PutObject`, `s3.GetObject`)

**SSaC에서의 사용:**
```go
// @post FileResult result = file.File.Upload({key: path, body: request.File})
// @get FileData data = file.File.Download({key: path})
// @delete FileResult result = file.File.Delete({key: path})
```

**fullend.yaml 설정:**
```yaml
file:
  backend: s3  # s3 | local
  s3:
    bucket: my-bucket
    region: ap-northeast-2
  local:
    root: ./uploads
```

S3가 기본. 비용이 거의 무시할 수준이고, 로컬은 서버 이전/스케일아웃 시 문제됨.

## 공통 원칙

- 모든 내장 모델은 패키지 접두사 규칙을 따름 (Phase 021 규칙과 동일)
- fullend가 특별 취급하지 않음 — 사용자 커스텀 패키지와 동일한 검증 경로
- 사용자가 대체 시 fullend.yaml 설정은 무시됨 (사용자 패키지가 자체 설정 관리)

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `pkg/session/session.go` | 신규 — SessionModel 인터페이스 + PostgreSQL/Memory 구현체 |
| `pkg/cache/cache.go` | 신규 — CacheModel 인터페이스 + PostgreSQL/Memory 구현체 |
| `pkg/file/file.go` | 신규 — FileModel 인터페이스 + S3/LocalFile 구현체 |
| `internal/projectconfig/config.go` | session/cache/file backend 설정 파싱 |

## 의존성

- Phase 021 완료 (패키지 접두사 모델 판단 규칙)

## 미결 사항

1. **Queue 모델** — PostgreSQL LISTEN/NOTIFY 기반. 필요 시 후속 Phase
2. **추가 백엔드** — Redis, Memcached 등은 커뮤니티 확장 또는 필요 시 후속 추가

## 검증 방법

```bash
go test ./pkg/session/...
go test ./pkg/cache/...
go test ./pkg/file/...
fullend validate specs/dummy-gigbridge  # 내장 모델 interface 교차 검증
```
