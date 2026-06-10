import sys
sys.stdout.reconfigure(encoding='utf-8')

with open(r'E:\Project2026\zap-update\AGENTS.md', 'r', encoding='utf-8') as f:
    content = f.read()

old = """## 十一、交互与自检

- 写代码前如有不清楚的地方，主动提问，不要猜测或假设
- 将问题写在 `.trae/documents/Interactive.md` 中，等待回答后再继续
- 每次修改完成后，全面检查是否有 bug，确认任务是否完全完成

---

## 十二、PowerShell 编码警告"""

new = """## 十一、代码审查与提交

**每次涉及代码改动，必须执行以下流程：**

1. 使用 `$open-code-review` 技能对本次改动进行代码审查
2. 修复审查中发现的问题
3. 审查通过后，提交代码（`git add` + `git commit`）

不允许跳过审查直接提交。

---

## 十二、交互与自检

- 写代码前如有不清楚的地方，主动提问，不要猜测或假设
- 将问题写在 `.trae/documents/Interactive.md` 中，等待回答后再继续
- 每次修改完成后，全面检查是否有 bug，确认任务是否完全完成

---

## 十三、PowerShell 编码警告"""

content = content.replace(old, new)
content = content.replace('## 十三、文档参考', '## 十四、文档参考')

with open(r'E:\Project2026\zap-update\AGENTS.md', 'w', encoding='utf-8') as f:
    f.write(content)

print('AGENTS.md updated with code review section')
