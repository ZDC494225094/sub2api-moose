-- 操作审计日志补充接入端点信息（多节点部署时区分用户从哪个入口、由哪台服务器处理）：
--   request_host：用户请求使用的 Host（域名或 IP:端口）
--   server_node ：处理请求的节点名（环境变量 SERVER_NODE_NAME，未设置时为主机名）
--   server_ip   ：处理请求的本机接收地址（连接 local addr）
-- 带常量默认值的 ADD COLUMN 在 PostgreSQL 11+ 只改元数据，不重写表。

SET LOCAL lock_timeout = '5s';

ALTER TABLE audit_logs
    ADD COLUMN IF NOT EXISTS request_host VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS server_node VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS server_ip VARCHAR(64) NOT NULL DEFAULT '';
