# 站点定制模块 (site-customization)

## 功能范围

站点外观与自定义入口的UI配置型扩展：

- **自定义菜单** (`custom_menu_items`)：用户侧边栏和管理员菜单的自定义链接
- **页脚友情链接** (`footer_friend_links`)：首页页脚展示的外部链接
- **自定义端点** (`custom_endpoints`)：管理员快速复制的API地址配置
- **站点名称** (`site_name`)：全站品牌名称（注：站点名称是基础功能，不受开关控制）
- **公告系统** (`announcements`)：管理员创建、目标定向、用户已读状态

## 开关行为

### 开启时
- 公共设置API返回自定义菜单、页脚链接、自定义端点
- 管理员可创建/编辑公告
- 前端显示所有自定义UI元素

### 关闭时
- 公共设置API **不返回**自定义菜单、页脚链接、自定义端点（数据层过滤）
- 管理员**无法创建**新公告（返回 `CUSTOM_EXTENSION_DISABLED`）
- 已发布的公告**继续显示**（历史内容保留，类似营销券的权益保留）
- 已读状态继续维护
- 站点名称**继续生效**（不受开关控制）

## 设计原则

站点定制是**UI配置型**而非业务逻辑型：

1. **前端显示层过滤**：通过 `useSiteCustomizationAdmission()` 控制UI元素显示
2. **后端数据层过滤**：关闭时公共API不返回自定义内容（避免数据泄露）
3. **历史内容保留**：已发布的公告继续可见（不能因开关让用户看不到重要通知）
4. **管理写入准入**：关闭时拒绝创建新公告

## 准入端口

### AnnouncementAdmission

管理员创建公告时的准入检查：

```go
type AnnouncementAdmission interface {
    // CheckCreate 检查是否允许创建新公告
    // 关闭时返回 CUSTOM_EXTENSION_DISABLED
    CheckCreate(ctx context.Context) error
}
```

### 公共设置过滤

在 `SettingsService.GetPublicSettings()` 中：
- 开关关闭时，将 `CustomMenuItems`、`FooterFriendLinks`、`CustomEndpoints` 设为 nil/空数组
- 站点名称保留（基础功能）

## 已完成

- [x] 准入接口定义
- [x] 公告创建准入检查
- [x] 公共设置过滤逻辑
- [x] 前端准入hook
- [x] 前端UI元素显示控制
- [x] 独立迁移文件
- [x] Wire 依赖装配
- [x] catalog.json 更新

## 当前状态

**managed: true** - 准入逻辑和宿主集成已完成。
