# neo-blackbox

CCTV 영상 녹화, AI 객체 감지, 이벤트 규칙 평가를 하나로 묶은 백엔드 서버입니다.
Machbase(시계열 DB)에 영상 청크와 감지 데이터를 저장하고, REST API로 조회할 수 있습니다.

## 주요 기능

- **카메라 관리** - RTSP/WebRTC 카메라 등록, 활성화/비활성화, 상태 조회
- **영상 녹화** - FFmpeg를 이용한 RTSP 스트림 녹화 및 청크 단위 DB 저장
- **AI 감지** - 외부 AI 서버(MVS)와 연동하여 객체 감지 결과 수집
- **이벤트 규칙** - DSL 기반 규칙 평가 (예: `person > 5 AND car >= 2`)
- **센서 데이터** - 센서 데이터 저장 및 조회
- **웹 UI** - API 테스트용 웹 페이지 내장

## 프로젝트 구조

```
neo-blackbox/
├── main.go                  # 진입점
├── magefile.go              # 빌드/배포 스크립트 (mage)
├── config.yaml              # 설정 파일
├── web/
│   └── index.html           # API 테스트 웹 UI
├── tools/                   # 외부 바이너리 (ffmpeg, ffprobe, mediamtx, ai)
└── internal/
    ├── config/              # 설정 로드
    ├── db/                  # Machbase DB 연동
    ├── dsl/                 # 이벤트 규칙 DSL 파서
    ├── ffmpeg/              # FFmpeg 프로세스 관리
    ├── logger/              # 로그 설정
    ├── mediamtx/            # MediaMTX 미디어 서버 연동
    ├── server/              # HTTP API 핸들러
    └── watcher/             # 파일 감시 및 DB 저장
```

## 시작하기

### 필요 사항

- Go 1.25+
- [Machbase](https://machbase.com) (시계열 DB)
- FFmpeg / FFprobe
- [Mage](https://magefile.org) (빌드 도구)

### 설정

`config.yaml`을 환경에 맞게 수정합니다.

```yaml
server:
  addr: 127.0.0.1:8000          # 서버 주소
  camera_dir: "./tmp/cameras"    # 카메라 설정 파일 저장 경로
  mvs_dir: "./tmp/mvs"          # MVS 파일 저장 경로
  data_dir: "./tmp/data"        # 영상 데이터 저장 경로

machbase:
  host: 127.0.0.1
  port: 5654

ffmpeg:
  binary: "/path/to/ffmpeg"

log:
  level: "info"                  # debug, info, warn, error
  format: "json"                 # json, text
  output: "file"                 # stdout, file, both
  file:
    filename: "./logs/blackbox.log"
```

### 빌드 및 실행

```bash
# 빌드
mage build

# 실행
mage run

# 개발 모드 (go run)
mage dev

# 테스트
mage test
```

### 배포

```bash
# 패키징 (dist/ 폴더에 tar.gz 생성)
mage package

# 기본 서버에 배포
mage dp

# G4U 서버에 배포
mage dpG4u
```

## API

서버 실행 후 `http://{addr}/` 에서 웹 UI로 API를 테스트할 수 있습니다.

전체 API 명세는 [API_SPEC.md](API_SPEC.md)를 참고하세요.

### 주요 API 목록

| 메서드 | 경로 | 설명 |
|--------|------|------|
| `POST` | `/api/camera` | 카메라 생성 |
| `GET` | `/api/cameras` | 카메라 목록 조회 |
| `GET` | `/api/camera/:id` | 카메라 정보 조회 |
| `POST` | `/api/camera/:id/enable` | 카메라 활성화 |
| `POST` | `/api/camera/:id/disable` | 카메라 비활성화 |
| `GET` | `/api/event_rule/:camera_id` | 이벤트 규칙 조회 |
| `POST` | `/api/event_rule` | 이벤트 규칙 추가 |
| `GET` | `/api/camera_events` | 이벤트 로그 조회 |
| `POST` | `/api/ai/result` | AI 감지 결과 업로드 |
| `GET` | `/api/v_get_chunk` | 영상 청크 다운로드 |

## DB 테이블

카메라 생성 시 3개의 Machbase TAG 테이블이 자동으로 만들어집니다.

| 테이블 | 용도 |
|--------|------|
| `{table}` | 영상 청크 저장 |
| `{table}_event` | 이벤트 규칙 평가 결과 |
| `{table}_log` | AI 객체 감지 카운트 |

## 이벤트 규칙 (DSL)

카메라별로 이벤트 규칙을 등록하면, AI 감지 결과를 기반으로 매 초 규칙을 평가합니다.

```
person > 5               # person이 5개 초과
person > 5 AND car >= 2  # person 5개 초과이고 car 2개 이상
(person + car) > 10      # person과 car 합계가 10 초과
```

기록 모드:
- **ALL_MATCHES** - 조건이 참일 때마다 기록
- **EDGE_ONLY** - 상태가 변할 때만 기록 (false->true: TRIGGER, true->false: RESOLVE)
