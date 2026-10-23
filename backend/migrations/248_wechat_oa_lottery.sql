-- 微信公众号抽奖支持：活动渠道字段 + 公众号 openid 绑定表
-- 幂等：重复执行不报错

ALTER TABLE lottery_activities
    ADD COLUMN IF NOT EXISTS wechat_oa_only BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS wechat_oa_bindings (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL,
    openid     VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT wechat_oa_bindings_user_id_key UNIQUE (user_id),
    CONSTRAINT wechat_oa_bindings_openid_key UNIQUE (openid)
);

CREATE INDEX IF NOT EXISTS idx_wechat_oa_bindings_user_id ON wechat_oa_bindings (user_id);
CREATE INDEX IF NOT EXISTS idx_wechat_oa_bindings_openid ON wechat_oa_bindings (openid);
