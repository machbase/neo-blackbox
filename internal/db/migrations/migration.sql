create tag table sensor3 (
    name varchar(128) primary key,
    time datetime basetime,
    value double summarized
) with rollup;


create tag table {camera} (
    name varchar(128) primary key,
    time datetime basetime,     -- 시작 시간
    value double summarized,    -- 몇 초 (LENGTH)
    chunk_path varchar(128)     -- 동영상 파일 경로 (청크 경로)
) with rollup;


CREATE TAG TABLE {camera}_event (
    name                VARCHAR(128) PRIMARY KEY,  -- camera_id.rule_id
    time                DATETIME BASETIME,         -- tick_time
    value               double,       -- 2/1/0/-1
    expression_text      VARCHAR(200),
    used_counts_snapshot JSON -- JSON
) METADATA (
    camera_id  VARCHAR(64),
    rule_id    VARCHAR(64),
    rule_name  VARCHAR(128)
);

CREATE TAG TABLE {camera}_log (
    name     VARCHAR(128) PRIMARY KEY,   -- camera_id.ident
    time     DATETIME BASETIME,          -- tick_time
    value    DOUBLE,                     -- count (차트/집계용)

    model_id VARCHAR(64) -- 현재 0
) METADATA (
    camera_id VARCHAR(64),
    ident     VARCHAR(64)
);

-- stream_config 추가 컬럼: media_url, event_rule, detect_objects
-- ALTER TABLE stream_config ADD COLUMN media_url VARCHAR(512);
-- ALTER TABLE stream_config ADD COLUMN event_rule VARCHAR(1024);
-- ALTER TABLE stream_config ADD COLUMN detect_objects JSON;

-- MVS (Machine Vision System) 설정은 파일로 관리: {mvs_dir}/{camera_id}.mvs