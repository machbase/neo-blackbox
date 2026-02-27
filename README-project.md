# neo-blackbox

CCTV 영상 녹화, AI 객체 감지, 이벤트 규칙 평가를 하나로 묶은 백엔드 서버입니다.
Machbase(시계열 DB)에 영상 청크와 감지 데이터를 저장하고, REST API로 조회할 수 있습니다.

## 주요 기능

- **카메라 관리** - RTSP/WebRTC 카메라 등록, 활성화/비활성화, 상태 조회
- **영상 녹화** - FFmpeg를 이용한 RTSP 스트림 녹화 및 청크 단위 DB 저장
- **미디어 서버** - MediaMTX를 통한 RTSP 스트림 관리
- **AI 감지** - blackbox-ai-manager와 연동하여 객체 감지 결과 수집
- **이벤트 규칙** - DSL 기반 규칙 평가 (예: `person > 5 AND car >= 2`)
- **센서 데이터** - 센서 데이터 저장 및 조회
- **웹 UI** - API 테스트용 웹 페이지 내장

## 프로젝트 구조

```
neo-blackbox/
├── cmd/
│   └── neo-blackbox/
│       └── main.go              # 진입점
├── magefile.go                  # 빌드/배포 스크립트 (mage)
├── web/
│   └── index.html               # API 테스트 웹 UI
├── tools/                       # 외부 바이너리 (플랫폼별 서브디렉토리)
│   ├── linux-amd64/             #   ffmpeg, ffprobe, mediamtx, mediamtx.yml
│   ├── linux-arm64/             #   blackbox-ai-manager, blackbox-ai-core
│   ├── darwin-amd64/            #   libonnxruntime.so, config.json 등
│   ├── darwin-arm64/
│   └── windows-amd64/
└── internal/
    ├── ai/                      # AI manager 프로세스 관리
    ├── config/                  # 설정 로드 (config.yaml 포함)
    ├── db/                      # Machbase DB 연동
    ├── dsl/                     # 이벤트 규칙 DSL 파서
    ├── ffmpeg/                  # FFmpeg 프로세스 관리
    ├── logger/                  # 로그 설정
    ├── mediamtx/                # MediaMTX 프로세스/클라이언트 관리
    ├── server/                  # HTTP API 핸들러
    └── watcher/                 # 파일 감시 및 DB 저장
```

## 시작하기

### 필요 사항

- Go 1.25+
- [Machbase](https://machbase.com) (시계열 DB)
- [Mage](https://magefile.org) (빌드 도구)

### 설정

`internal/config/config.yaml`을 환경에 맞게 수정합니다.

```yaml
server:
  addr: 0.0.0.0:8000
  camera_dir: "../bin/cameras"   # 카메라 설정 파일 저장 경로
  mvs_dir: "../ai/mvs"           # MVS 파일 저장 경로
  data_dir: "../bin/data"        # 영상 데이터 저장 경로

machbase:
  host: 127.0.0.1
  port: 5654
  timeout_seconds: 30

mediamtx:
  binary: "../tools/mediamtx"   # 비어있으면 외부 서버 사용
  host: 127.0.0.1
  port: 9997                     # MediaMTX HTTP API 포트

ffmpeg:
  binary: "../tools/ffmpeg"
  defaults:
    probe_binary: "../tools/ffprobe"

ai:
  binary: "../ai/blackbox-ai-manager"  # 비어있으면 AI 비활성화
  config_file: "../ai/config.json"

log:
  dir: "../logs"                 # 앱 로그 + ffmpeg 로그 디렉토리
  level: "info"                  # debug, info, warn, error
  format: "json"                 # json, text
  output: "both"                 # stdout, file, both
  file:
    filename: "blackbox.log"
    max_size: 100                # MB
    max_backups: 10
    max_age: 30                  # days
    compress: true
```

> **경로 참고**: 모든 상대경로는 **config 파일이 위치한 디렉토리(`config/`) 기준**입니다.
> 예: `../bin/cameras` → `bin/cameras/`, `../ai/mvs` → `ai/mvs/`, `../tools/mediamtx` → `tools/mediamtx`
> 개발 시에는 `internal/config/config.yaml`을 직접 수정하거나, 절대경로로 설정하세요.

### 빌드 및 실행

```bash
# 빌드 (tmp/neo-blackbox 생성)
mage build

# 실행 (internal/config/config.yaml 사용)
mage run

# 개발 모드 (go run)
mage dev

# 커스텀 config로 실행
mage runWithConfig path/to/config.yaml
mage devWithConfig path/to/config.yaml

# 테스트
mage test

# 코드 품질 검사 (fmt + vet + test)
mage check
```

### 배포

패키징 시 타겟 플랫폼을 `os-arch` 형식으로 지정합니다.

```bash
# 패키징 (dist/ 폴더에 아카이브 생성)
mage package linux-amd64
mage package linux-arm64
mage package windows-amd64   # .zip 생성

# 기본 서버에 배포 (패키징 + scp)
mage dp linux-amd64

# G4U 서버에 배포
mage dpG4u linux-amd64
```

배포 서버 정보는 `.env` 파일로 설정합니다:

```env
DEPLOY_USER=eleven
DEPLOY_HOST=192.168.0.87
DEPLOY_PATH=/blackbox/be/pkg
```

## 패키지 구조

`mage package` 실행 후 생성되는 구조:

```
neo-blackbox-linux-amd64/
├── bin/
│   ├── neo-blackbox             # 백엔드 바이너리
│   └── web/
│       └── index.html           # 웹 UI (바이너리 실행 위치 기준 탐색)
├── config/
│   └── config.yaml              # 설정 파일 (환경에 맞게 수정 필요)
├── tools/                       # 미디어 도구
│   ├── ffmpeg
│   ├── ffprobe
│   ├── mediamtx
│   └── mediamtx.yml
├── ai/                          # AI 엔진
│   ├── blackbox-ai-manager
│   ├── blackbox-ai-core
│   ├── config.json
│   ├── libonnxruntime.so        # ONNX Runtime 공유 라이브러리
│   ├── models/
│   │   └── *.onnx               # AI 모델 파일
│   └── mvs/                     # MVS 작업 디렉토리 (런타임 생성)
└── README.txt
```

압축 해제 후 실행:

```bash
tar -xzf neo-blackbox-linux-amd64.tar.gz
cd neo-blackbox-linux-amd64

# config/config.yaml 수정 후 실행
./bin/neo-blackbox -config config/config.yaml
```

> **주의**: `config.yaml`의 상대경로는 **config 파일 위치(`config/`) 기준**입니다.
> 패키지 루트(`neo-blackbox-linux-amd64/`)에서 실행하면 경로가 올바르게 해석됩니다.


