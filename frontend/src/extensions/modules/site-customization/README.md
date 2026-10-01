# 站点定制模块 (site-customization)

## 功能范围

站点外观与自定义入口的UI配置型扩展：

- **自定义菜单**：用户侧边栏和管理员菜单的自定义链接
- **页脚友情链接**：首页页脚展示的外部链接
- **自定义端点**：管理员快速复制的API地址配置
- **公告系统**：管理员创建、目标定向、用户已读状态

## 前端集成

### 使用准入 Hook

```typescript
import { useSiteCustomizationAdmission } from '@/extensions/modules/site-customization'

// 组件中使用
const isEnabled = useSiteCustomizationAdmission()

// 控制UI显示
<div v-if="isEnabled">
  <CustomMenu />
  <FooterLinks />
</div>
```

### 开关行为

- **开启时**：显示自定义菜单、页脚链接、公告等UI元素
- **关闭时**：隐藏所有自定义UI内容（后端已过滤数据）
- **历史公告继续显示**：已发布的公告不受开关影响（权益保留）

## 当前状态

**managed: false** - 前端准入集成尚未完成。
