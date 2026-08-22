package prompt

// 模板使用 {{key}} 占位，由 Render 替换。勿在模板正文里写未转义的单花括号 JSON 示例冲突——
// JSON 示例里的 { } 保持原样；占位符统一 {{name}}。

const titleOptionsTpl = `你是一位爆款文章标题专家,擅长创作吸引人的标题。

根据以下选题,生成 3-5 个爆款文章标题方案:
选题：{{topic}}

要求:
1. 每个方案包含主标题和副标题
2. 主标题要包含数字、情绪化词汇,吸引眼球
3. 副标题要补充说明,增强吸引力
4. 标题要简洁有力,不超过30字
5. 不同方案要有不同的切入角度
6. 符合新媒体爆款文章的风格

请直接返回 JSON 格式,不要有其他内容:
[
  {
    "mainTitle": "主标题1",
    "subTitle": "副标题1"
  },
  {
    "mainTitle": "主标题2",
    "subTitle": "副标题2"
  },
  {
    "mainTitle": "主标题3",
    "subTitle": "副标题3"
  }
]`

const outlineTpl = `你是一位专业的文章策划师,擅长设计文章结构。

根据以下标题,生成文章大纲:
主标题：{{mainTitle}}
副标题：{{subTitle}}
{{descriptionSection}}

要求:
1. 大纲要有清晰的逻辑结构
2. 包含开头引入、核心观点(3-5个)、结尾升华
3. 每个章节要有明确的标题和核心要点(2-3个)
4. 适合2000字左右的文章

请直接返回 JSON 格式,不要有其他内容:
{
  "sections": [
    {
      "section": 1,
      "title": "章节标题",
      "points": ["要点1", "要点2"]
    }
  ]
}`

const descriptionSectionTpl = `
用户补充要求：{{userDescription}}
请在大纲中充分体现用户的补充要求。`

const contentTpl = `你是一位资深的内容创作者,擅长撰写优质文章。

根据以下大纲,创作文章正文:
主标题：{{mainTitle}}
副标题：{{subTitle}}
大纲：
{{outline}}

要求:
1. 内容要充实,每个章节300-400字
2. 语言流畅,富有感染力
3. 适当使用金句,增强可读性
4. 添加过渡句,确保逻辑连贯
5. 使用 Markdown 格式,章节使用 ## 标题

请直接返回 Markdown 格式的正文内容,不要有其他内容。`

const imageRequirementsTpl = `你是一位专业的新媒体编辑,擅长为文章配图。

根据以下文章内容,分析配图需求,并在正文中插入图片占位符:
主标题：{{mainTitle}}
正文：
{{content}}

【重要】可用的配图方式（请严格只从以下方式中选择，禁止使用未列出的方式）：
{{availableMethods}}

各配图方式的使用要求：
{{methodUsageGuide}}

通用要求:
1. 识别需要配图的位置(封面、关键章节、段落之间等)
2. 建议配图数量: 3-5张
3. **在正文中插入占位符**：使用格式 {{IMAGE_PLACEHOLDER_N}}，其中 N 为配图序号（1, 2, 3...）
   - 封面图占位符 {{IMAGE_PLACEHOLDER_1}} 放在文章最开头（正文第一行之前）
   - 其他配图占位符可以放在任意合适位置（章节标题后、段落之间、列表项后等）
   - 占位符必须独占一行
4. **imageSource 字段必须且只能是上述可用配图方式之一，不要使用其他值**
5. placeholderId 必须与正文中插入的占位符完全一致
6. position=1 为封面图

请直接返回 JSON 格式,不要有其他内容:
{
  "contentWithPlaceholders": "正文与占位符...",
  "imageRequirements": [
    {
      "position": 1,
      "type": "cover",
      "sectionTitle": "",
      "imageSource": "NANO_BANANA",
      "keywords": "",
      "prompt": "cover prompt",
      "placeholderId": "{{IMAGE_PLACEHOLDER_1}}"
    }
  ]
}`

const modifyOutlineTpl = `你是一位专业的文章策划师,擅长根据用户反馈优化文章结构。

当前主标题：{{mainTitle}}
当前副标题：{{subTitle}}
当前大纲：
{{outline}}

用户修改建议：
{{modifySuggestion}}

要求:
1. 充分理解并落实用户的修改建议
2. 保持大纲结构清晰、逻辑连贯
3. 每个章节包含标题与 2-3 个要点

请直接返回 JSON 格式,不要有其他内容:
{
  "sections": [
    {
      "section": 1,
      "title": "章节标题",
      "points": ["要点1", "要点2"]
    }
  ]
}`
