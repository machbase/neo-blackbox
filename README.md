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