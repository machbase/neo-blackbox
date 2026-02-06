# Blackbox Backend API Specification

## POST /api/camera

카메라 생성

Request:
```json
{
    "table": "string",                    // required - 테이블 이름 (여러 카메라가 같은 테이블 공유 가능)
    "name": "string",                     // required - 카메라 ID (고유 식별자)
    "desc": "string",                     // 카메라 설명

    "rtsp_url": "string",                 // RTSP 스트림 URL
    "webrtc_url": "string",               // WebRTC 스트림 URL
    "media_url": "string",                // 미디어 서버 URL

    "model_id": 0,                        // int - AI 모델 ID (기본값: 0)
    "detect_objects": ["string"],         // []string - 감지할 객체 목록
                                          // 예: ["person", "car", "truck", "bus"]

    "save_objects": false,                // bool - {table}_log 테이블에 감지 데이터 저장 여부

    "ffmpeg_options": [                   // []ReqKV - FFmpeg 옵션 배열
        { "k": "string", "v": "string" }  // k: 옵션명, v: 옵션값 (optional)
    ],

    "output_dir": "string",               // ffmpeg 출력 디렉토리
                                          // 기본값: {data_dir}/{camera_id}/in

    "output_name": "string"               // ffmpeg 출력 파일명
                                          // 기본값: manifest.mpd
}
```

Response:
```json
{
    "camera_id": "string"                 // 생성된 카메라 ID
}
```

---

## GET /api/camera/:id

카메라 정보 조회

Response:
```json
{
    "Enabled": false,                     // bool - 카메라 활성화 상태
    "table": "string",                    // 테이블 이름
    "name": "string",                     // 카메라 ID
    "desc": "string",                     // 카메라 설명

    "rtsp_url": "string",                 // RTSP 스트림 URL
    "webrtc_url": "string",               // WebRTC 스트림 URL
    "media_url": "string",                // 미디어 서버 URL

    "model_id": 0,                        // int - AI 모델 ID
    "detect_objects": ["string"],         // []string - 감지할 객체 목록
    "save_objects": false,                // bool - 감지 데이터 저장 여부

    "ffmpeg_options": [                   // []ReqKV - FFmpeg 옵션 배열
        { "k": "string", "v": "string" }
    ],

    "output_dir": "string",               // ffmpeg 출력 디렉토리
    "output_name": "string",              // ffmpeg 출력 파일명

    "EventRule": [                        // []EventRule - 이벤트 규칙 배열
        {
            "rule_id": "string",          // 규칙 ID
            "name": "string",             // 규칙 이름
            "expression_text": "string",  // DSL 표현식 (예: "person > 5")
            "record_mode": "string",      // 기록 모드: "ALL_MATCHES" | "EDGE_ONLY"
            "enabled": false              // bool - 규칙 활성화 여부
        }
    ]
}
```

---

## POST /api/camera/:id

카메라 정보 수정 (name, table은 변경 불가)

Request:
```json
{
    "desc": "string",                     // 카메라 설명

    "rtsp_url": "string",                 // RTSP 스트림 URL
    "webrtc_url": "string",               // WebRTC 스트림 URL
    "media_url": "string",                // 미디어 서버 URL

    "model_id": 0,                        // int - AI 모델 ID
    "detect_objects": ["string"],         // []string - 감지할 객체 목록
    "save_objects": false,                // bool - 감지 데이터 저장 여부

    "ffmpeg_options": [                   // []ReqKV - FFmpeg 옵션 배열
        { "k": "string", "v": "string" }
    ],

    "output_dir": "string",               // ffmpeg 출력 디렉토리
    "output_name": "string"               // ffmpeg 출력 파일명
}
```

Response:
```json
{
    "camera_id": "string"                 // 수정된 카메라 ID
}
```

---

## DELETE /api/camera/:id

카메라 삭제

Response:
```json
{
    "name": "string"                      // 삭제된 카메라 이름
}
```

---

## POST /api/camera/:id/enable

카메라 활성화 (ffmpeg 프로세스 시작)

Response:
```json
{
    "name": "string",                     // 카메라 이름
    "pid": 0,                             // int - 프로세스 ID
    "status": "string"                    // 상태: "running" | "stopped"
}
```

---

## POST /api/camera/:id/disable

카메라 비활성화 (ffmpeg 프로세스 종료)

Response:
```json
{
    "name": "string",                     // 카메라 이름
    "status": "string"                    // 상태: "stopped"
}
```

---

## GET /api/camera/:id/status

카메라 상태 조회

Response:
```json
{
    "name": "string",                     // 카메라 이름
    "status": "string",                   // 상태: "running" | "stopped"
    "pid": 0,                             // int - 프로세스 ID (running인 경우)
    "started_at": "string",               // 시작 시간 (RFC3339 형식)
    "uptime": "string"                    // 가동 시간 (예: "2h30m15s")
}
```

---

## GET /api/cameras

카메라 목록 조회

Response:
```json
{
    "cameras": [                          // 카메라 목록 배열
        {
            "id": "string",               // 카메라 ID
            "label": "string"             // 카메라 레이블 (현재 id와 동일)
        }
    ]
}
```

---

## GET /api/cameras/health

전체 카메라 상태 조회

Response:
```json
{
    "total": 0,                           // int - 전체 카메라 수
    "running": 0,                         // int - 실행 중인 카메라 수
    "stopped": 0,                         // int - 중지된 카메라 수
    "cameras": [                          // 카메라 상태 배열
        {
            "name": "string",             // 카메라 이름
            "status": "string",           // 상태: "running" | "stopped"
            "pid": 0,                     // int - 프로세스 ID (running인 경우)
            "started_at": "string",       // 시작 시간 (RFC3339 형식)
            "uptime": "string"            // 가동 시간
        }
    ]
}
```

---

## GET /api/event_rule/:camera_id

특정 카메라의 이벤트 규칙 목록 조회

Response:
```json
{
    "camera_id": "string",                // 카메라 ID
    "event_rules": [                      // 이벤트 규칙 배열 (규칙이 없으면 빈 배열 [])
        {
            "rule_id": "string",          // 규칙 ID
            "name": "string",             // 규칙 이름
            "expression_text": "string",  // DSL 표현식
                                          // 예: "person > 5 AND car >= 2"
            "record_mode": "string",      // 기록 모드
                                          // "ALL_MATCHES": 매 초마다 기록
                                          // "EDGE_ONLY": 상태 변화 시점만 기록
            "enabled": false              // bool - 규칙 활성화 여부
        }
    ]
}
```

---

## POST /api/event_rule

이벤트 규칙 추가

Request:
```json
{
    "camera_id": "string",                // required - 카메라 ID
    "rule": {
        "rule_id": "string",              // required - 규칙 ID (고유 식별자)
        "name": "string",                 // 규칙 이름
        "expression_text": "string",      // required - DSL 표현식
                                          // 지원: 산술(+-*/), 비교(><>=<=!===)
                                          // 논리(AND/OR/NOT), 괄호
        "record_mode": "string",          // required - "ALL_MATCHES" | "EDGE_ONLY"
        "enabled": false                  // bool - 규칙 활성화 여부
    }
}
```

Response:
```json
{
    "camera_id": "string",                // 카메라 ID
    "rule": {
        "rule_id": "string",              // 규칙 ID
        "name": "string",                 // 규칙 이름
        "expression_text": "string",      // DSL 표현식
        "record_mode": "string",          // 기록 모드
        "enabled": false                  // bool - 규칙 활성화 여부
    }
}
```

---

## POST /api/event_rule/:camera_id/:rule_id

이벤트 규칙 수정 (rule_id는 URL에서 지정, 변경 불가)

Request:
```json
{
    "name": "string",                     // 규칙 이름
    "expression_text": "string",          // DSL 표현식
    "record_mode": "string",              // "ALL_MATCHES" | "EDGE_ONLY"
    "enabled": false                      // bool - 규칙 활성화 여부
}
```

Response:
```json
{
    "camera_id": "string",                // 카메라 ID
    "rule_id": "string",                  // 규칙 ID
    "rule": {
        "rule_id": "string",              // 규칙 ID
        "name": "string",                 // 규칙 이름
        "expression_text": "string",      // DSL 표현식
        "record_mode": "string",          // 기록 모드
        "enabled": false                  // bool - 규칙 활성화 여부
    }
}
```

---

## DELETE /api/event_rule/:camera_id/:rule_id

이벤트 규칙 삭제

Response:
```json
{
    "camera_id": "string",                // 카메라 ID
    "rule_id": "string"                   // 삭제된 규칙 ID
}
```

---

## POST /api/ai/result

AI 감지 결과 업로드 ({camera}_log 테이블에 저장)

Request:
```json
{
    "camera_id": "string",                // required - 카메라 ID
    "model_id": 0,                        // int - AI 모델 ID
    "timestamp": 0,                       // int64 - 밀리초 단위 타임스탬프
                                          // (0은 유효하지 않음)
    "detections": {                       // map[string]int - 객체별 감지 카운트
        "person": 0,                      // 예: person 객체 0개 감지
        "car": 0                          // 예: car 객체 0개 감지
    },
    "total_objects": 0                    // int - 전체 감지된 객체 수
}
```

Response:
```json
null                                      // 응답 본문 없음 (success: true만 확인)
```

---

## GET /api/get_time_range?tagname={tagname}

카메라의 비디오 데이터 시간 범위 조회

Response:
```json
{
    "camera": "string",                   // 카메라 이름
    "start": "string",                    // 시작 시간 (RFC3339 형식)
    "end": "string",                      // 종료 시간 (RFC3339 형식)
    "chunk_duration_seconds": 0.0,        // float64 - 청크 길이 (초)
    "fps": 0                              // int - 프레임 레이트
}
```

---

## GET /api/get_chunk_info?tagname={tagname}&time={time}

특정 시간의 비디오 청크 정보 조회

Response:
```json
{
    "camera": "string",                   // 카메라 이름
    "time": "string",                     // 청크 시간 (RFC3339 형식)
    "length": 0,                          // int - 청크 크기 (바이트)
    "sign": 0                             // int - 서명 값
}
```

---

## GET /api/v_get_chunk?tagname={tagname}&time={time}

특정 시간의 비디오 청크 다운로드 (바이너리)

Response:
```
binary                                    // 비디오 청크 바이너리 데이터
```

---

## GET /api/get_camera_rollup_info?tagname={tagname}&minutes={minutes}&start_time={start_time}&end_time={end_time}

카메라 비디오 데이터 롤업 정보 조회 (시간대별 집계)

Response:
```json
{
    "camera": "string",                   // 카메라 이름
    "minutes": 0,                         // int - 집계 단위 (분)
    "start_time_ns": 0,                   // int64 - 시작 시간 (나노초)
    "end_time_ns": 0,                     // int64 - 종료 시간 (나노초)
    "start": "string",                    // 시작 시간 (RFC3339 형식)
    "end": "string",                      // 종료 시간 (RFC3339 형식)
    "rows": [                             // 시간대별 데이터 배열
        {
            "time": "string",             // 시간 (RFC3339 형식)
            "sum_length": 0.0             // float64 - 해당 시간대 총 데이터 크기
        }
    ]
}
```

---

## GET /api/sensors?tagname={tagname}

카메라의 센서 목록 조회

Response:
```json
{
    "camera": "string",                   // 카메라 이름
    "sensors": [                          // 센서 목록 배열
        {
            "id": "string",               // 센서 ID (예: "sensor-1")
            "label": "string"             // 센서 레이블 (예: "Sensor 1")
        }
    ]
}
```

---

## GET /api/sensor_data?sensors={sensors}&start={start}&end={end}

센서 데이터 조회

- `sensors`: 쉼표로 구분된 센서 ID 목록 (예: "sensor-1,sensor-2")
- `start`: 시작 시간 (RFC3339 형식)
- `end`: 종료 시간 (RFC3339 형식)

Response:
```json
{
    "sensors": ["string"],                // []string - 조회된 센서 ID 목록
    "samples": [                          // 시간별 샘플 데이터 배열
        {
            "time": "string",             // 샘플 시간 (RFC3339 형식)
            "values": {                   // map[string]float64 - 센서별 값
                "sensor-1": 0.0           // 센서 ID를 키로 하는 측정값
            }
        }
    ]
}
```

---

## POST /api/mvs/camera

MVS 카메라 설정 파일 생성 (.mvs 파일)

Request:
```json
{
    "camera_id": "string",                // required - 카메라 ID
    "camera_url": "string",               // required - 카메라 URL
    "model_id": 0,                        // int - AI 모델 ID
    "detect_objects": ["string"]          // []string - 감지할 객체 목록
}
```

Response:
```json
{
    "camera_id": "string",                // 카메라 ID
    "mvs_path": "string"                  // 생성된 .mvs 파일 경로
}
```
