-- 用户来源 IP：注册 IP 与最近使用 IP。
-- 注册 IP 在注册时写入；历史用户为空。
-- 最近使用 IP 在登录与 API Key 鉴权热路径上防抖写入，不回填历史数据。
ALTER TABLE users ADD COLUMN IF NOT EXISTS registration_ip VARCHAR(45);

ALTER TABLE users ADD COLUMN IF NOT EXISTS last_used_ip VARCHAR(45);

CREATE INDEX IF NOT EXISTS idx_users_registration_ip ON users (registration_ip);

CREATE INDEX IF NOT EXISTS idx_users_last_used_ip ON users (last_used_ip);

COMMENT ON COLUMN users.registration_ip IS 'Registration source IP; NULL for users created before the field existed';

COMMENT ON COLUMN users.last_used_ip IS 'Last login/API request source IP; only recorded after the field shipped';
