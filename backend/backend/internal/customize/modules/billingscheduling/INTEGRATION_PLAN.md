# 计费与调度增强模块 - 服务层集成计划

## 集成策略

由于计费调度功能深度耦合且涉及大量文件，采用**标记优先、按需切换**的策略：

### Phase 1: 标记准入点（当前阶段）
在关键服务中注入准入接口，但**默认不执行检查**（透传）。

### Phase 2: 选择性启用
在需要控制的功能点启用准入检查。

### Phase 3: 完整验证
验证开关不影响历史数据。

## 需要集成的服务

### 1. Rate Multiplier（分组倍率）
**服务文件：**
- `BillingCacheService` - 计费缓存服务
- `GroupService` - 分组服务
- `AccountService` - 账号服务

**集成点：**
- 读取用户分组倍率时检查准入
- 扩展关闭时倍率回退到 1.0

### 2. Time Pricing（分时定价）
**服务文件：**
- `custom_channel_time_pricing.go` - 分时定价逻辑

**集成点：**
- `MultiplierAt` 方法中检查准入
- 扩展关闭时返回 1.0（无分时差异）

### 3. Custom Usage（自定义用量）
**服务文件：**
- `gateway_usage_billing.go` - 用量计费
- `account_usage_service.go` - 用量服务

**集成点：**
- 自定义用量计算前检查准入
- 扩展关闭时使用标准计算

### 4. Composite Scheduling（组合调度）
**服务文件：**
- `scheduler_*.go` - 调度逻辑
- `route_*.go` - 路由服务

**集成点：**
- 复杂调度策略前检查准入
- 扩展关闭时使用简单调度

## 准入注入模式

```go
type BillingCacheService struct {
    // ... existing fields ...
    rateMultiplierAdmission interface{} // billingscheduling.RateMultiplierAdmission
}

func (s *BillingCacheService) SetRateMultiplierAdmission(admission interface{}) {
    s.rateMultiplierAdmission = admission
}

func (s *BillingCacheService) getRateMultiplier(ctx context.Context, groupID int64) float64 {
    // Check admission if set
    if adm, ok := s.rateMultiplierAdmission.(interface{ AllowRateMultiplier(context.Context) error }); ok {
        if err := adm.AllowRateMultiplier(ctx); err != nil {
            // Extension disabled, fallback to 1.0
            return 1.0
        }
    }
    // Extension enabled or admission not set, use custom multiplier
    return s.loadRateMultiplier(ctx, groupID)
}
```

## 历史数据保护

**关键约束：** 准入检查只影响**新操作**，不修改历史记录。

**保护措施：**
1. 准入检查在计算**前**执行，不在查询历史数据时执行
2. 已记录的 usage/billing 数据保持不变
3. 缓存回退不影响 DB 中的真实记录

## 实施优先级

**P0 (必须完成):**
- [x] 准入接口定义
- [x] Wire 装配
- [ ] 分时定价集成（custom_channel_time_pricing.go）

**P1 (渐进完成):**
- [ ] 倍率准入注入（BillingCacheService）
- [ ] 自定义用量准入
- [ ] 组合调度准入

**P2 (可选增强):**
- [ ] 完整的单元测试
- [ ] 集成测试验证

## 当前进度

已完成准入接口和 Wire 装配，现在开始集成到服务层。
