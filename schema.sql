-- users 表：用户信息、余额、QPS限制
CREATE TABLE users (
    id          BIGSERIAL PRIMARY KEY,
    username    VARCHAR(64) UNIQUE NOT NULL,
    password    VARCHAR(255) NOT NULL,
    balance     DECIMAL(10,2) DEFAULT 0,     -- 余额（元）
    qps_limit   INT DEFAULT 10,             -- QPS限制
    created_at  TIMESTAMP DEFAULT NOW(),
    updated_at  TIMESTAMP DEFAULT NOW()
);

-- orders 表：音频处理订单
CREATE TABLE orders (
    id              BIGSERIAL PRIMARY KEY,
    order_no        UUID NOT NULL,                   -- 订单号
    user_id         BIGINT NOT NULL REFERENCES users(id),
    token_id        BIGINT REFERENCES user_tokens(id),
    status          VARCHAR(20) DEFAULT 'pending',  -- pending/processing/success/failed/canceled/expired
    task_id         VARCHAR(64),                     -- ASR 任务 ID
    callback_url    TEXT,                            -- 回调地址（可选）

    -- 原始音频信息
    original_url    TEXT,                            -- 原始音频地址
    source          VARCHAR(20),                     -- 来源: local/remote/stream

    -- 音频元数据
    audio_duration  BIGINT,                          -- 音频时长（秒）
    audio_format   VARCHAR(20),                     -- 音频格式 mp3/wav/m4a等
    sample_rate     BIGINT,                          -- 采样率 Hz
    channels        INT,                             -- 声道数
    bit_rate        BIGINT,                          -- 比特率 bps
    codec           VARCHAR(50),                     -- 编解码器

    -- S3 存储
    s3_key          TEXT,                            -- S3 对象 key
    s3_url          TEXT,                            -- S3 外链（预签名）
    s3_expires_at   TIMESTAMP,                       -- S3 链接过期时间

    -- 费用明细
    storage_cost    DECIMAL(10,2) DEFAULT 0,        -- S3 存储费用
    asr_cost        DECIMAL(10,2) DEFAULT 0,        -- ASR 转写费用
    subtitle_cost   DECIMAL(10,2) DEFAULT 0,        -- 字幕费用
    meeting_cost    DECIMAL(10,2) DEFAULT 0,        -- 会议纪要费用
    total_cost      DECIMAL(10,2) DEFAULT 0,        -- 总费用

    -- 可选功能
    need_subtitle   BOOLEAN DEFAULT FALSE,          -- 是否需要字幕
    need_meeting_note BOOLEAN DEFAULT FALSE,        -- 是否需要会议纪要

    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);

-- 自动更新 updated_at 的触发器函数
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- users 表的触发器
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- orders 表的触发器
CREATE TRIGGER update_orders_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- transcription_results 表：转写结果
CREATE TABLE transcription_results (
    id              BIGSERIAL PRIMARY KEY,
    order_no        UUID NOT NULL REFERENCES orders(order_no),
    result_s3_key   TEXT,                             -- S3 存储 key（转写结果）
    result_text     TEXT,                             -- 转写文本（小于阈值时直接存储）
    subtitle_s3_key TEXT,                             -- 字幕 S3 key
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);

CREATE TRIGGER update_transcription_results_updated_at
    BEFORE UPDATE ON transcription_results
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- meeting_summaries 表：会议纪要
CREATE TABLE meeting_summaries (
    id              BIGSERIAL PRIMARY KEY,
    order_no        UUID NOT NULL REFERENCES orders(order_no),
    summary_s3_key  TEXT,                             -- 会议纪要 S3 key
    summary_text    TEXT,                             -- 会议纪要文本
    cost            DECIMAL(10,2) DEFAULT 0,           -- 生成费用
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);

CREATE TRIGGER update_meeting_summaries_updated_at
    BEFORE UPDATE ON meeting_summaries
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- user_tokens 表：用户 token 管理（支持多设备登录）
CREATE TABLE user_tokens (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    token       UUID NOT NULL,
    created_at  TIMESTAMP DEFAULT NOW(),
    expires_at  TIMESTAMP DEFAULT (NOW() + INTERVAL '7 days'),
    updated_at  TIMESTAMP DEFAULT NOW()
);

CREATE TRIGGER update_user_tokens_updated_at
    BEFORE UPDATE ON user_tokens
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- user_recharges 表：用户充值记录
CREATE TABLE user_recharges (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    token_id    BIGINT REFERENCES user_tokens(id),
    amount      DECIMAL(10,2) NOT NULL,
    created_at  TIMESTAMP DEFAULT NOW(),
    updated_at  TIMESTAMP DEFAULT NOW()
);

CREATE TRIGGER update_user_recharges_updated_at
    BEFORE UPDATE ON user_recharges
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- recharge_orders 表：充值订单（支付订单）
CREATE TABLE recharge_orders (
    id              BIGSERIAL PRIMARY KEY,
    recharge_no     UUID NOT NULL,
    user_id         BIGINT NOT NULL REFERENCES users(id),
    token_id        BIGINT REFERENCES user_tokens(id),
    amount          DECIMAL(10,2) NOT NULL,
    pay_channel     VARCHAR(20) NOT NULL,
    status          VARCHAR(20) DEFAULT 'pending',
    trade_no        VARCHAR(64),
    pay_url         TEXT,
    expires_at      TIMESTAMP,
    paid_at         TIMESTAMP,
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);

CREATE TRIGGER update_recharge_orders_updated_at
    BEFORE UPDATE ON recharge_orders
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- 索引
CREATE UNIQUE INDEX idx_orders_order_no ON orders(order_no);
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_token_id ON orders(token_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_created_at ON orders(created_at);
CREATE INDEX idx_transcription_results_order_no ON transcription_results(order_no);
CREATE UNIQUE INDEX idx_user_tokens_token ON user_tokens(token);
CREATE INDEX idx_user_tokens_user_id ON user_tokens(user_id);
CREATE INDEX idx_user_recharges_user_id ON user_recharges(user_id);
CREATE INDEX idx_user_recharges_token_id ON user_recharges(token_id);
CREATE INDEX idx_user_recharges_created_at ON user_recharges(created_at);
CREATE UNIQUE INDEX idx_recharge_orders_recharge_no ON recharge_orders(recharge_no);
CREATE INDEX idx_recharge_orders_user_id ON recharge_orders(user_id);
CREATE INDEX idx_recharge_orders_status ON recharge_orders(status);
CREATE INDEX idx_recharge_orders_trade_no ON recharge_orders(trade_no);
CREATE INDEX idx_meeting_summaries_order_no ON meeting_summaries(order_no);
