---
name: "avalonia-run-binding"
description: "Avalonia 12 中 &lt;Run&gt; 元素配合编译绑定导致 StackOverflowException 的诊断与修复。当 Avalonia 应用在选择或加载页面时崩溃出现堆栈溢出时使用此技能。"
---

# Avalonia Run 元素 + 编译绑定 = StackOverflow

> .NET 8 + Avalonia 12.0.3 + AtomUI 6.0 + CommunityToolkit.Mvvm 8.4.2 实战踩坑

---

## 一、问题描述

在 Avalonia 12 项目中启用编译绑定后（`AvaloniaUseCompiledBindingsByDefault=true` 或 `x:DataType`），当页面渲染时立即抛出：

```
System.StackOverflowException: Exception_WasThrown
```

**触发条件**：`TextBlock` 内使用 `<Run>` 元素且 `Run.Text` 上存在 `{Binding}` 表达式。

## 二、根因

Avalonia 的 `TextBlock` 使用 `Inlines` 集合渲染富文本。当 `<Run>` 的 `Text` 属性通过绑定更新时：

1. `Run.Text` 值变化 → 父级 `TextBlock` 重新测量 `Inlines`
2. 重新测量触发布局传递（Layout Pass）
3. 布局传递中编译绑定被重新求值
4. 绑定值变化 → 回到步骤 1 → **无限递归**

编译绑定（`x:DataType`）会加剧此问题，因为绑定在编译时解析为直接属性访问，执行更快，递归更紧密。

## 三、解决方案

**所有 `<Run>` 绑定替换为 `TextBlock.Text` + `StringFormat`。**

### ❌ 错误写法

```xml
<TextBlock Foreground="{DynamicResource TextSecondary}">
  <Run Text="📦 "/>
  <Run Text="{Binding ServerVersion, StringFormat='v{0}'}"/>
</TextBlock>
```

### ✅ 正确写法（单属性）

```xml
<TextBlock Foreground="{DynamicResource TextSecondary}"
           Text="{Binding ServerVersion, StringFormat='📦 v{0}'}"/>
```

### ✅ 正确写法（多属性拼接）

```xml
<!-- 用多个 TextBlock 并排替代 Run 拼接 -->
<StackPanel Orientation="Horizontal" Spacing="0">
  <TextBlock Text="{Binding DiskUsed, StringFormat='{}{0:F1} / '}"/>
  <TextBlock Text="{Binding DiskTotal, StringFormat='{}{0:F1} GB'}"/>
</StackPanel>
```

### ✅ 正确写法（数字拼接）

```xml
<!-- 原来：<Run Text="{Binding Count}"/><Run Text=" / "/><Run Text="{Binding Total}"/> -->
<TextBlock Text="{Binding Count, StringFormat='{}{0} / '}"/>
<TextBlock Text="{Binding Total}"/>
```

## 四、检测方法

1. 应用启动后操作某页面立即崩溃 → `StackOverflowException`
2. 检查崩溃页面的 XAML → 搜索 `<Run Text="{Binding`
3. 全部替换为 `TextBlock.Text` + `StringFormat`

> **注意**：`Span` 元素也有同样的问题，替换规则相同。

## 五、安全例外

以下 `<Run>` 的使用是安全的（无绑定）：

```xml
<!-- ✅ 纯静态文本，无 Binding -->
<TextBlock>
  <Run Text="确定要删除项目「"/>
  <Run x:Name="ProjectNameRun" Text=""/>
  <Run Text="」吗？"/>
</TextBlock>
```

---

## 六、快速参考

| 写法 | 安全性 |
|------|--------|
| `<Run Text="静态文本"/>` | ✅ 安全 |
| `<Run Text="{Binding ...}"/>` | ❌ 堆栈溢出 |
| `<Span Text="{Binding ...}"/>` | ❌ 堆栈溢出 |
| `<TextBlock Text="{Binding ...}"/>` | ✅ 安全 |
