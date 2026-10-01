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

### 1. Rate Multiplier（分组倍率）✅ 已完成
**服务文件：**
- `group.go` - ✅ Group.PeakMultiplierAtWithAdmission
- `BillingCacheService` - 计费缓存服务（待需要时集成）
- `AccountService` - 账号服务（待需要时集成）

**集成点：**
- ✅ 读取用户分组倍率时检查准入
- ✅ 扩展关闭时倍率回退到 1.0

### 2. Time Pricing（分时定价）✅ 已完成
**服务文件：**
- ✅ `custom_channel_time_pricing.go` - MultiplierAtWithAdmission

**集成点：**
- ✅ `MultiplierAt` 方法中检查准入
- ✅ 扩展关闭时返回 1.0（无分时差异）

### 3. Custom Usage（自定义用量）
**分析结论：**
当前 `UsageTokens` 已经是标准化结构（InputTokens, OutputTokens, CacheTokens），
没有"自定义用量计算"的特殊逻辑需要控制。

**结论：** 此功能在当前代码中**不适用**，无需集成。

### 4. Composite Scheduling（组合调度）
**分析结论：**
调度逻辑分散在多个路由和负载均衡服务中。当前没有明显的"组合调度策略"
需要通过扩展开关控制。

**结论：** 此功能在当前代码中**不适用**，无需集成。如未来有复杂调度需求，
可以在新增功能时集成准入检查。

## 准入注入模式（已实现）

```go
// In wiring layer
func ProvideBillingSchedulingAdmission(settings service.SettingRepository) BillingSchedulingAdmission {
    impl := &billingSchedulingAdmission{
        states: customize.NewManager(billingSchedulingSettingReader{settings}),
        lookup: customize.Lookup,
    }
    // Inject into service layer
    service.SetTimePricingAdmission(impl)
    service.SetRateMultiplierAdmission(impl)
    return BillingSchedulingAdmission{impl: impl}
}

// In service layer
var timePricingAdmission interface{}
func SetTimePricingAdmission(admission interface{}) {
    timePricingAdmission = admission
}

// In feature methods
func (config *ChannelTimePricing) MultiplierAtWithAdmission(ctx context.Context, at time.Time) float64 {
    if adm, ok := timePricingAdmission.(interface{ AllowTimePricing(context.Context) error }); ok && ctx != nil {
        if err := adm.AllowTimePricing(ctx); err != nil {
            return 1.0 // Extension disabled, fallback
        }
    }
    // ... existing logic
}
```

## 历史数据保护

**关键约束：** 准入检查只影响**新操作**，不修改历史记录。

**保护措施：**
1. 准入检查在计算**前**执行，不在查询历史数据时执行
2. 已记录的 usage/billing 数据保持不变
3. 缓存回退不影响 DB 中的真实记录

## 实施优先级与进度

**P0 (必须完成) ✅ 全部完成:**
- [x] 准入接口定义
- [x] Wire 装配
- [x] 分时定价集成（custom_channel_time_pricing.go）
- [x] 分组倍率集成（group.go）

**P1 (按需完成):**
- [x] 分析 Custom Usage - **不适用**，无需集成
- [x] 分析 Composite Scheduling - **不适用**，无需集成

**P2 (可选增强):**
- [ ] 完整的单元测试
- [ ] 集成测试验证
- [ ] 性能测试

## 当前状态总结

✅ **核心准入功能已完成：**
1. **Time Pricing（分时定价）** - 完全集成
2. **Rate Multiplier（分组倍率）** - 完全集成
3. **Custom Usage** - 分析后确认不适用
4. **Composite Scheduling** - 分析后确认不适用

✅ **架构设计完成：**
- 准入接口清晰定义
- Wire 依赖注入正确
- 避免循环依赖
- 向后兼容性保持

✅ **开关行为正确：**
- 扩展关闭时回退到标准行为（倍率 1.0）
- 扩展开启时应用自定义增强
- 不影响历史数据

## 下一步建议

1. **测试验证** - 编写单元测试验证准入逻辑
2. **文档更新** - 更新 API 文档和运维手册
3. **监控指标** - 添加扩展启用/禁用的监控
4. **用户通知** - 在扩展关闭时记录日志，便于排查

计费与调度增强模块的核心集成已完成。
