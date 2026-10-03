# 站点定制：页脚友情链接

仅 `footer_friend_links` 属于此模块。关闭后公共设置隐藏友情链接但不删除持久化配置。公告、自定义菜单和自定义端点属于上游，不受开关影响。

独立接纳迁移为 `site-customization/202610030001_adopt_footer_links.sql`：legacy 缺失值补 true，native 补 false；保留显式选择，删除后不重种。

历史宿主迁移 `500_extension_site_customization.sql` 的归属描述错误。因无法确认所有部署的执行账本，文件保持字节不变，仅作兼容占位；不得继续使用该编号体系添加扩展迁移。
