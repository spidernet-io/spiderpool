#!/bin/bash

# 用法: ./get_pr_info.sh PR_NUMBER
# 示例: ./get_pr_info.sh 4898

PR_NUMBER=$1
REPO="spidernet-io/spiderpool"

if [ -z "$PR_NUMBER" ]; then
  echo "请提供 PR 编号"
  echo "用法: $0 PR_NUMBER"
  exit 1
fi

# 获取 PR 信息
PR_INFO=$(curl -s "https://api.github.com/repos/$REPO/pulls/$PR_NUMBER")

# 使用 jq 提取所需信息
echo "PR 编号: $(echo $PR_INFO | jq -r .number)"
echo "标题: $(echo $PR_INFO | jq -r .title)"
echo "状态: $(echo $PR_INFO | jq -r .state)"
echo "作者: $(echo $PR_INFO | jq -r .user.login)"
echo "创建时间: $(echo $PR_INFO | jq -r .created_at)"
echo "合并时间: $(echo $PR_INFO | jq -r .merged_at)"
echo "标签: $(echo $PR_INFO | jq -r '[.labels[].name] | join(", ")')"
echo "合并人: $(echo $PR_INFO | jq -r '.merged_by.login // "未合并"')"
echo "URL: $(echo $PR_INFO | jq -r .html_url)"
echo ""
echo "描述:"
echo "$(echo $PR_INFO | jq -r .body)"
