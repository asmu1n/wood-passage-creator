const stepNames: Record<string, string> = {
  title_generator: '生成标题',
  outline_generator: '生成大纲',
  outline_modifier: 'AI 修改大纲',
  content_generator: '生成正文',
  image_generator: '规划与生成配图',
  content_merger: '图文合成',
  agent1_generate_titles: '生成标题',
  agent2_generate_outline: '生成大纲',
  agent3_generate_content: '生成正文',
  agent4_analyze_image_requirements: '分析配图需求',
  agent5_generate_images: '生成配图',
  agent6_merge_content: '图文合成',
  ai_modify_outline: 'AI 修改大纲',
}

export const getExecutionStepName = (name: string): string => stepNames[name] ?? name
