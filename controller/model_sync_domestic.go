package controller

import (
	"encoding/json"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

func getDomesticDefaultCatalog() ([]upstreamVendor, []upstreamModel) {
	vendors := []upstreamVendor{
		{Name: "DeepSeek", Description: "DeepSeek API，适合中文创作、通用对话、代码与推理场景。", Icon: "DeepSeek.Color", Status: 1},
		{Name: "阿里百炼", Description: "阿里云百炼 / DashScope，提供通义千问、向量、重排与多模态模型。", Icon: "Qwen.Color", Status: 1},
		{Name: "智谱", Description: "智谱 BigModel / Z.ai，提供 GLM 系列文本、视觉与推理模型。", Icon: "Zhipu.Color", Status: 1},
		{Name: "Moonshot", Description: "Moonshot / Kimi API，适合长上下文、代码与智能体场景。", Icon: "Moonshot", Status: 1},
		{Name: "火山方舟", Description: "火山引擎方舟，提供豆包、Seed 系列及多模态模型接入。", Icon: "Doubao.Color", Status: 1},
		{Name: "百度千帆", Description: "百度千帆 ModelBuilder，提供 ERNIE、DeepSeek 等模型服务。", Icon: "Wenxin.Color", Status: 1},
		{Name: "腾讯混元", Description: "腾讯混元大模型，适合中文创作、办公与企业应用场景。", Icon: "Hunyuan.Color", Status: 1},
		{Name: "MiniMax", Description: "MiniMax API，提供 M 系列文本、语音与图像模型。", Icon: "Minimax.Color", Status: 1},
		{Name: "SiliconFlow", Description: "硅基流动 SiliconCloud，聚合 Qwen、DeepSeek、GLM、BGE 等开源模型。", Icon: "SiliconCloud.Color", Status: 1},
		{Name: "快手可灵", Description: "快手可灵 Kling，提供文生视频、图生视频等创意生成模型。", Icon: "Kling.Color", Status: 1},
		{Name: "即梦", Description: "火山引擎即梦，提供图像与视频生成模型。", Icon: "Jimeng.Color", Status: 1},
		{Name: "Vidu", Description: "生数科技 Vidu，提供文生视频、图生视频、首尾帧与参考视频生成模型。", Icon: "Vidu", Status: 1},
	}

	models := []upstreamModel{
		domesticModel("deepseek-v4-flash", "DeepSeek", "DeepSeek V4 Flash，适合低成本、高速度的通用创作和问答。", "DeepSeek.Color", "国产,文本,推理", []string{"openai"}),
		domesticModel("deepseek-v4-pro", "DeepSeek", "DeepSeek V4 Pro，适合复杂推理、代码和长上下文任务。", "DeepSeek.Color", "国产,文本,推理,代码", []string{"openai"}),
		domesticModel("deepseek-chat", "DeepSeek", "DeepSeek 通用对话兼容模型名，适合存量配置平滑迁移。", "DeepSeek.Color", "国产,文本", []string{"openai"}),
		domesticModel("deepseek-reasoner", "DeepSeek", "DeepSeek 推理兼容模型名，适合数学、代码和复杂分析。", "DeepSeek.Color", "国产,文本,推理", []string{"openai"}),

		domesticModel("qwen3.6-max-preview", "阿里百炼", "Qwen 3.6 当前最强推理模型，适合复杂分析、多步骤规划和高质量生成。", "Qwen.Color", "国产,文本,推理", []string{"openai"}),
		domesticModel("qwen3.6-plus", "阿里百炼", "Qwen 3.6 主力通用模型，适合智能体、长文档和大型代码库场景。", "Qwen.Color", "国产,文本", []string{"openai"}),
		domesticModel("qwen3.6-plus-2026-04-02", "阿里百炼", "Qwen 3.6 Plus 快照模型，适合需要固定版本回归验证的场景。", "Qwen.Color", "国产,文本", []string{"openai"}),
		domesticModel("qwen3.6-flash", "阿里百炼", "Qwen 3.6 高速模型，适合低成本、高并发和多模态理解任务。", "Qwen.Color", "国产,文本", []string{"openai"}),
		domesticModel("qwen3.6-flash-2026-04-16", "阿里百炼", "Qwen 3.6 Flash 快照模型，适合固定版本验证与成本敏感场景。", "Qwen.Color", "国产,文本", []string{"openai"}),
		domesticModel("qwen3-max", "阿里百炼", "Qwen 3 旗舰通用模型，适合复杂创作、分析和工具调用。", "Qwen.Color", "国产,文本", []string{"openai"}),
		domesticModel("qwen3-max-preview", "阿里百炼", "Qwen 3 旗舰预览模型，适合新能力验证和高质量复杂任务。", "Qwen.Color", "国产,文本,推理", []string{"openai"}),
		domesticModel("qwen-flash", "阿里百炼", "Qwen 3 轻量高速模型，适合成本敏感的通用问答和工具调用。", "Qwen.Color", "国产,文本", []string{"openai"}),
		domesticModel("qwen-plus", "阿里百炼", "通义千问均衡模型，适合多数中文创作和业务问答。", "Qwen.Color", "国产,文本", []string{"openai"}),
		domesticModel("qwen-max", "阿里百炼", "通义千问旗舰兼容模型，适合复杂创作、分析和高质量生成。", "Qwen.Color", "国产,文本", []string{"openai"}),
		domesticModel("qwen-turbo", "阿里百炼", "通义千问高速兼容模型，适合存量低成本、高并发场景。", "Qwen.Color", "国产,文本", []string{"openai"}),
		domesticModel("qwq-plus", "阿里百炼", "QwQ 主力推理模型，适合数学、代码和长链路思考任务。", "Qwen.Color", "国产,文本,推理", []string{"openai"}),
		domesticModel("qwq-32b", "阿里百炼", "QwQ 32B 推理模型，适合数学、代码和复杂分析。", "Qwen.Color", "国产,文本,推理", []string{"openai"}),
		domesticModel("qwen3-235b-a22b", "阿里百炼", "Qwen3 大尺寸 MoE 模型，适合高质量综合能力场景。", "Qwen.Color", "国产,文本", []string{"openai"}),
		domesticModel("qwen3-235b-a22b-thinking-2507", "阿里百炼", "Qwen3 235B Thinking 快照模型，强调推理与复杂规划。", "Qwen.Color", "国产,文本,推理", []string{"openai"}),
		domesticModel("qwen3-235b-a22b-instruct-2507", "阿里百炼", "Qwen3 235B Instruct 快照模型，适合指令跟随和工具调用。", "Qwen.Color", "国产,文本", []string{"openai"}),
		domesticModel("qwen3-coder-plus", "阿里百炼", "Qwen3 Coder 主力代码模型，适合仓库级理解、生成和修改。", "Qwen.Color", "国产,文本,代码", []string{"openai"}),
		domesticModel("qwen3-coder-flash", "阿里百炼", "Qwen3 Coder 高速代码模型，适合更低延迟的代码补全与修复。", "Qwen.Color", "国产,文本,代码", []string{"openai"}),
		domesticModel("qwen3-coder-next", "阿里百炼", "Qwen3 Coder 新一代代码模型，适合复杂编码和工程任务。", "Qwen.Color", "国产,文本,代码", []string{"openai"}),
		domesticModel("qwen-image-2.0-pro", "阿里百炼", "Qwen Image 2.0 Pro，适合文字渲染、负向提示词和多张图片变体。", "Qwen.Color", "国产,图像,编辑", []string{"image-generation"}),
		domesticModel("qwen-image-2.0", "阿里百炼", "Qwen Image 2.0 快速图像生成 / 编辑模型。", "Qwen.Color", "国产,图像,编辑", []string{"image-generation"}),
		domesticModel("qwen-image-max", "阿里百炼", "Qwen Image Max 文生图模型，适合兼容旧版千问出图调用。", "Qwen.Color", "国产,图像", []string{"image-generation"}),
		domesticModel("qwen-image-plus", "阿里百炼", "Qwen Image Plus 文生图模型，适合兼容旧版千问出图调用。", "Qwen.Color", "国产,图像", []string{"image-generation"}),
		domesticModel("qwen-image", "阿里百炼", "通义千问图像生成模型，适合兼容旧版文生图调用。", "Qwen.Color", "国产,图像", []string{"image-generation"}),
		domesticModel("qwen-image-edit-max", "阿里百炼", "Qwen Image Max 编辑模型，适合高质量指令图像编辑。", "Qwen.Color", "国产,图像,编辑", []string{"image-generation"}),
		domesticModel("qwen-image-edit-plus", "阿里百炼", "Qwen Image Plus 编辑模型，适合多变体图像编辑。", "Qwen.Color", "国产,图像,编辑", []string{"image-generation"}),
		domesticModel("qwen-image-edit", "阿里百炼", "Qwen Image 基础编辑模型，适合兼容旧版图像编辑调用。", "Qwen.Color", "国产,图像,编辑", []string{"image-generation"}),
		domesticModel("wan2.7-image-pro", "阿里百炼", "万相 2.7 专业图像生成 / 编辑模型，适合高质量和多图参考创作。", "Qwen.Color", "国产,图像,编辑", []string{"image-generation"}),
		domesticModel("wan2.7-image", "阿里百炼", "万相 2.7 图像生成 / 编辑模型，兼顾速度和画质。", "Qwen.Color", "国产,图像,编辑", []string{"image-generation"}),
		domesticModel("z-image-turbo", "阿里百炼", "Z-Image Turbo 快速图像生成模型，适合低成本写实图片生成。", "Qwen.Color", "国产,图像", []string{"image-generation"}),
		domesticModel("happyhorse-1.0-t2v", "阿里百炼", "爱诗 1.0 文生视频模型，适合有声视频创作。", "Qwen.Color", "国产,视频,文生视频", []string{"openai-video"}),
		domesticModel("wan2.7-t2v", "阿里百炼", "万相 2.7 文生视频稳定别名，支持音频同步和多镜头叙事。", "Qwen.Color", "国产,视频,文生视频", []string{"openai-video"}),
		domesticModel("happyhorse-1.0-i2v", "阿里百炼", "爱诗 1.0 首帧生视频模型，适合图片转视频。", "Qwen.Color", "国产,视频,图生视频", []string{"openai-video"}),
		domesticModel("wan2.7-i2v", "阿里百炼", "万相 2.7 图生视频稳定别名，支持首帧、首尾帧和视频续写。", "Qwen.Color", "国产,视频,图生视频", []string{"openai-video"}),
		domesticModel("wan2.7-t2v-2026-04-25", "阿里百炼", "万相 2.7 文生视频模型，支持自定义音频文件。", "Qwen.Color", "国产,视频,文生视频", []string{"openai-video"}),
		domesticModel("wan2.7-i2v-2026-04-25", "阿里百炼", "万相 2.7 图生视频模型，支持首帧、首尾帧和视频续写。", "Qwen.Color", "国产,视频,图生视频", []string{"openai-video"}),
		domesticModel("happyhorse-1.0-r2v", "阿里百炼", "爱诗 1.0 参考生视频模型，适合角色一致性和有声视频生成。", "Qwen.Color", "国产,视频,参考生视频", []string{"openai-video"}),
		domesticModel("wan2.7-r2v", "阿里百炼", "万相 2.7 参考生视频模型，适合角色一致性和多参考素材。", "Qwen.Color", "国产,视频,参考生视频", []string{"openai-video"}),
		domesticModel("happyhorse-1.0-video-edit", "阿里百炼", "爱诗 1.0 视频编辑模型，适合风格转换和元素替换。", "Qwen.Color", "国产,视频,编辑", []string{"openai-video"}),
		domesticModel("wan2.7-videoedit", "阿里百炼", "万相 2.7 视频编辑模型，适合指令编辑、特效复刻和运镜复刻。", "Qwen.Color", "国产,视频,编辑", []string{"openai-video"}),
		domesticModel("wan2.6-t2v", "阿里百炼", "万相 2.6 文生视频模型，支持多镜头叙事、自动配音和自定义音频。", "Qwen.Color", "国产,视频,文生视频", []string{"openai-video"}),
		domesticModel("wan2.6-i2v", "阿里百炼", "万相 2.6 图生视频模型。", "Qwen.Color", "国产,视频,图生视频", []string{"openai-video"}),
		domesticModel("wan2.6-i2v-flash", "阿里百炼", "万相 2.6 Flash 图生视频模型，适合更快的视频生成。", "Qwen.Color", "国产,视频,图生视频", []string{"openai-video"}),
		domesticModel("wan2.6-r2v", "阿里百炼", "万相 2.6 参考生视频模型，适合多角色和叙事型视频生成。", "Qwen.Color", "国产,视频,参考生视频", []string{"openai-video"}),
		domesticModel("wan2.6-r2v-flash", "阿里百炼", "万相 2.6 Flash 参考生视频模型，适合更快的多参考视频生成。", "Qwen.Color", "国产,视频,参考生视频", []string{"openai-video"}),
		domesticModel("wan2.5-t2v-preview", "阿里百炼", "万相 2.5 Preview 文生视频模型，支持自动配音和自定义音频。", "Qwen.Color", "国产,视频,文生视频", []string{"openai-video"}),
		domesticModel("wan2.5-i2v-preview", "阿里百炼", "万相 2.5 Preview 图生视频模型，New API 阿里视频适配器已内置。", "Qwen.Color", "国产,视频,图生视频", []string{"openai-video"}),
		domesticModel("wan2.2-t2v-plus", "阿里百炼", "万相 2.2 Plus 文生视频模型。", "Qwen.Color", "国产,视频,文生视频", []string{"openai-video"}),
		domesticModel("wan2.2-i2v-plus", "阿里百炼", "万相 2.2 Plus 图生视频模型，New API 阿里视频适配器已内置。", "Qwen.Color", "国产,视频,图生视频", []string{"openai-video"}),
		domesticModel("wan2.2-i2v-flash", "阿里百炼", "万相 2.2 Flash 图生视频模型，New API 阿里视频适配器已内置。", "Qwen.Color", "国产,视频,图生视频", []string{"openai-video"}),
		domesticModel("wan2.2-kf2v-flash", "阿里百炼", "万相 2.2 首尾帧生视频 Flash 模型。", "Qwen.Color", "国产,视频,首尾帧", []string{"openai-video"}),
		domesticModel("wan2.2-animate-move", "阿里百炼", "万相 2.2 角色动画模型，适合动作迁移到静态人物。", "Qwen.Color", "国产,视频,角色动画", []string{"openai-video"}),
		domesticModel("wan2.2-animate-mix", "阿里百炼", "万相 2.2 角色替换模型，适合视频中替换人物。", "Qwen.Color", "国产,视频,角色动画", []string{"openai-video"}),
		domesticModel("wanx2.1-t2v-plus", "阿里百炼", "万相 2.1 Plus 文生视频模型（中国内地命名）。", "Qwen.Color", "国产,视频,文生视频", []string{"openai-video"}),
		domesticModel("wanx2.1-t2v-turbo", "阿里百炼", "万相 2.1 Turbo 文生视频模型（中国内地命名）。", "Qwen.Color", "国产,视频,文生视频", []string{"openai-video"}),
		domesticModel("wanx2.1-i2v-plus", "阿里百炼", "万相 2.1 Plus 图生视频模型（中国内地命名），New API 阿里视频适配器已内置。", "Qwen.Color", "国产,视频,图生视频", []string{"openai-video"}),
		domesticModel("wanx2.1-i2v-turbo", "阿里百炼", "万相 2.1 Turbo 图生视频模型（中国内地命名），New API 阿里视频适配器已内置。", "Qwen.Color", "国产,视频,图生视频", []string{"openai-video"}),
		domesticModel("wanx2.1-kf2v-plus", "阿里百炼", "万相 2.1 Plus 首尾帧生视频模型（中国内地命名）。", "Qwen.Color", "国产,视频,首尾帧", []string{"openai-video"}),
		domesticModel("wanx2.1-vace-plus", "阿里百炼", "万相 2.1 VACE 视频编辑模型（中国内地命名），支持多图参考、视频重绘、局部编辑和视频延展。", "Qwen.Color", "国产,视频,编辑", []string{"openai-video"}),
		domesticModel("text-embedding-v4", "阿里百炼", "DashScope 当前推荐文本向量模型，适合语义搜索、RAG 和聚类。", "Qwen.Color", "国产,向量", []string{"embeddings"}),
		domesticModel("text-embedding-v3", "阿里百炼", "DashScope 文本向量兼容模型，适合已有 v3 索引迁移。", "Qwen.Color", "国产,向量", []string{"embeddings"}),
		domesticModel("qwen3-vl-embedding", "阿里百炼", "Qwen3 多模态向量模型，适合图文混合检索。", "Qwen.Color", "国产,向量,多模态", []string{"embeddings"}),
		domesticModel("qwen2.5-vl-embedding", "阿里百炼", "Qwen2.5 多模态向量模型，适合图文混合检索兼容场景。", "Qwen.Color", "国产,向量,多模态", []string{"embeddings"}),
		domesticModel("tongyi-embedding-vision-plus-2026-03-06", "阿里百炼", "通义多模态向量快照模型，适合跨模态搜索。", "Qwen.Color", "国产,向量,多模态", []string{"embeddings"}),
		domesticModel("tongyi-embedding-vision-flash-2026-03-06", "阿里百炼", "通义多模态向量 Flash 快照模型，适合低成本跨模态搜索。", "Qwen.Color", "国产,向量,多模态", []string{"embeddings"}),
		domesticModel("tongyi-embedding-vision-plus", "阿里百炼", "通义多模态向量模型，适合跨模态搜索。", "Qwen.Color", "国产,向量,多模态", []string{"embeddings"}),
		domesticModel("tongyi-embedding-vision-flash", "阿里百炼", "通义多模态向量 Flash 模型，适合低成本跨模态搜索。", "Qwen.Color", "国产,向量,多模态", []string{"embeddings"}),
		domesticModel("qwen3-vl-rerank", "阿里百炼", "Qwen3 多模态重排模型，适合文本、图片和视频混合排序。", "Qwen.Color", "国产,重排,多模态", []string{"jina-rerank"}),
		domesticModel("qwen3-rerank", "阿里百炼", "DashScope 当前推荐文本重排模型，适合语义检索和 RAG。", "Qwen.Color", "国产,重排", []string{"jina-rerank"}),
		domesticModel("gte-rerank-v2", "阿里百炼", "DashScope 兼容文本重排模型，适合存量检索与 RAG 场景。", "Qwen.Color", "国产,重排", []string{"jina-rerank"}),

		domesticModel("glm-5", "智谱", "GLM 旗舰文本模型，适合通用创作、分析和智能体任务。", "Zhipu.Color", "国产,文本", []string{"openai"}),
		domesticModel("glm-4.7", "智谱", "GLM-4.7，适合推理、代码和复杂任务。", "Zhipu.Color", "国产,文本,推理,代码", []string{"openai"}),
		domesticModel("glm-4.7-flash", "智谱", "GLM-4.7 Flash，适合高速低成本调用。", "Zhipu.Color", "国产,文本", []string{"openai"}),
		domesticModel("glm-4.6", "智谱", "GLM-4.6，适合通用文本、代码和工具调用场景。", "Zhipu.Color", "国产,文本,代码", []string{"openai"}),
		domesticModel("glm-4-flash", "智谱", "GLM-4 Flash，适合入门和高并发轻量任务。", "Zhipu.Color", "国产,文本", []string{"openai"}),
		domesticModel("glm-4v-plus", "智谱", "GLM-4V Plus，多模态视觉理解模型。", "Zhipu.Color", "国产,视觉", []string{"openai"}),
		domesticModel("cogView-4-250304", "智谱", "CogView-4 文生图模型，适合中文文字渲染和高质量图片生成。", "Zhipu.Color", "国产,图像", []string{"image-generation"}),
		domesticModel("cogview-3-flash", "智谱", "CogView-3 Flash 免费图像生成模型，适合快速出图。", "Zhipu.Color", "国产,图像,免费", []string{"image-generation"}),

		domesticModel("kimi-k2.5", "Moonshot", "Kimi K2.5，适合长上下文、代码和智能体任务。", "Moonshot", "国产,文本,代码", []string{"openai"}),
		domesticModel("kimi-k2-thinking", "Moonshot", "Kimi K2 Thinking，适合推理和复杂规划。", "Moonshot", "国产,文本,推理", []string{"openai"}),
		domesticModel("kimi-k2-thinking-turbo", "Moonshot", "Kimi K2 Thinking Turbo，兼顾推理能力和响应速度。", "Moonshot", "国产,文本,推理", []string{"openai"}),
		domesticModel("kimi-k2-turbo-preview", "Moonshot", "Kimi K2 Turbo 预览模型。", "Moonshot", "国产,文本", []string{"openai"}),

		domesticModel("Doubao-pro-32k", "火山方舟", "豆包 Pro 32K，适合中文创作、问答和复杂业务文本。", "Doubao.Color", "国产,文本", []string{"openai"}),
		domesticModel("Doubao-lite-32k", "火山方舟", "豆包 Lite 32K，适合轻量文本和高并发场景。", "Doubao.Color", "国产,文本", []string{"openai"}),
		domesticModel("doubao-seed-1-6-thinking-250715", "火山方舟", "Doubao Seed 1.6 Thinking，适合推理和复杂分析。", "Doubao.Color", "国产,文本,推理", []string{"openai"}),
		domesticModel("doubao-seedream-4-0-250828", "火山方舟", "Seedream 4.0 图像生成模型。", "Doubao.Color", "国产,图像", []string{"image-generation"}),
		domesticModel("doubao-seedream-4-5-251128", "火山方舟", "Seedream 4.5 图像生成模型，适合高质量 4K 图像创作。", "Doubao.Color", "国产,图像,编辑", []string{"image-generation"}),
		domesticModel("doubao-seedance-1-0-pro-250528", "火山方舟", "Seedance 1.0 Pro 视频生成模型。", "Doubao.Color", "国产,视频", []string{"openai-video"}),
		domesticModel("doubao-seedance-1-0-pro-fast-250610", "火山方舟", "Seedance 1.0 Pro Fast 视频生成模型，适合更快的视频生成。", "Doubao.Color", "国产,视频", []string{"openai-video"}),
		domesticModel("doubao-seedance-1-0-lite-t2v-250428", "火山方舟", "Seedance 1.0 Lite T2V，适合低成本文生视频。", "Doubao.Color", "国产,视频,文生视频", []string{"openai-video"}),
		domesticModel("doubao-seedance-1-0-lite-i2v-250428", "火山方舟", "Seedance 1.0 Lite I2V，适合低成本图生视频。", "Doubao.Color", "国产,视频,图生视频", []string{"openai-video"}),
		domesticModel("doubao-seedance-1-5-pro-251215", "火山方舟", "Seedance 1.5 Pro 视频生成模型，支持有声视频生成。", "Doubao.Color", "国产,视频,有声视频", []string{"openai-video"}),
		domesticModel("doubao-seedance-2-0-260128", "火山方舟", "Seedance 2.0 视频生成模型，适合高质量、多镜头叙事和原声音画同步场景。", "Doubao.Color", "国产,视频,有声视频", []string{"openai-video"}),
		domesticModel("doubao-seedance-2-0-fast-260128", "火山方舟", "Seedance 2.0 Fast 视频生成模型，适合更快的高质量有声视频生成。", "Doubao.Color", "国产,视频,有声视频", []string{"openai-video"}),

		domesticModel("ernie-4.0-turbo-8k", "百度千帆", "ERNIE 4.0 Turbo，适合通用中文创作和问答。", "Wenxin.Color", "国产,文本", []string{"openai"}),
		domesticModel("ernie-4.0-turbo-128k", "百度千帆", "ERNIE 4.0 Turbo 128K，适合长文本处理。", "Wenxin.Color", "国产,文本,长上下文", []string{"openai"}),
		domesticModel("ernie-3.5-8k", "百度千帆", "ERNIE 3.5，适合通用文本任务。", "Wenxin.Color", "国产,文本", []string{"openai"}),
		domesticModel("ernie-speed-128k", "百度千帆", "ERNIE Speed 128K，适合长上下文轻量任务。", "Wenxin.Color", "国产,文本,长上下文", []string{"openai"}),
		domesticModel("deepseek-r1", "百度千帆", "千帆上的 DeepSeek R1 推理模型。", "Wenxin.Color", "国产,文本,推理", []string{"openai"}),
		domesticModel("deepseek-v3", "百度千帆", "千帆上的 DeepSeek V3 通用模型。", "Wenxin.Color", "国产,文本", []string{"openai"}),

		domesticModel("hunyuan-lite", "腾讯混元", "混元 Lite，适合轻量中文任务和测试。", "Hunyuan.Color", "国产,文本", []string{"openai"}),
		domesticModel("hunyuan-standard", "腾讯混元", "混元 Standard，适合通用中文创作和问答。", "Hunyuan.Color", "国产,文本", []string{"openai"}),
		domesticModel("hunyuan-standard-256K", "腾讯混元", "混元 Standard 256K，适合长上下文任务。", "Hunyuan.Color", "国产,文本,长上下文", []string{"openai"}),
		domesticModel("hunyuan-pro", "腾讯混元", "混元 Pro，适合复杂文本生成和分析。", "Hunyuan.Color", "国产,文本", []string{"openai"}),

		domesticModel("MiniMax-M2.7", "MiniMax", "MiniMax M2.7，适合文本创作、代码和工具调用。", "Minimax.Color", "国产,文本,代码", []string{"openai"}),
		domesticModel("MiniMax-M2.7-highspeed", "MiniMax", "MiniMax M2.7 Highspeed，高速文本模型。", "Minimax.Color", "国产,文本", []string{"openai"}),
		domesticModel("MiniMax-M2.5", "MiniMax", "MiniMax M2.5，适合通用对话和创作。", "Minimax.Color", "国产,文本", []string{"openai"}),
		domesticModel("image-01", "MiniMax", "MiniMax 图像生成模型。", "Minimax.Color", "国产,图像", []string{"image-generation"}),
		domesticModel("MiniMax-Hailuo-2.3", "MiniMax", "MiniMax Hailuo 2.3 视频生成模型，适合高质量文生视频和图生视频。", "Minimax.Color", "国产,视频", []string{"openai-video"}),
		domesticModel("MiniMax-Hailuo-2.3-Fast", "MiniMax", "MiniMax Hailuo 2.3 Fast 图生视频模型，适合效率优先场景。", "Minimax.Color", "国产,视频,图生视频", []string{"openai-video"}),
		domesticModel("MiniMax-Hailuo-02", "MiniMax", "MiniMax Hailuo 02 视频生成模型，支持更高分辨率和更长时长。", "Minimax.Color", "国产,视频", []string{"openai-video"}),

		domesticModel("Qwen/Qwen2-7B-Instruct", "SiliconFlow", "硅基流动 Qwen2 7B 指令模型，适合免费或低成本测试。", "SiliconCloud.Color", "国产,文本", []string{"openai"}),
		domesticModel("Qwen/Qwen2-72B-Instruct", "SiliconFlow", "硅基流动 Qwen2 72B 指令模型。", "SiliconCloud.Color", "国产,文本", []string{"openai"}),
		domesticModel("THUDM/glm-4-9b-chat", "SiliconFlow", "硅基流动 GLM-4 9B Chat 模型。", "SiliconCloud.Color", "国产,文本", []string{"openai"}),
		domesticModel("deepseek-ai/DeepSeek-V2-Chat", "SiliconFlow", "硅基流动 DeepSeek V2 Chat 模型。", "SiliconCloud.Color", "国产,文本", []string{"openai"}),
		domesticModel("BAAI/bge-m3", "SiliconFlow", "硅基流动 BGE-M3 向量模型。", "SiliconCloud.Color", "国产,向量", []string{"embeddings"}),
		domesticModel("BAAI/bge-reranker-v2-m3", "SiliconFlow", "硅基流动 BGE Reranker v2 M3 重排模型。", "SiliconCloud.Color", "国产,重排", []string{"jina-rerank"}),
		domesticModel("ByteDance/SDXL-Lightning", "SiliconFlow", "硅基流动 SDXL-Lightning 图像模型。", "SiliconCloud.Color", "国产,图像", []string{"image-generation"}),
		domesticModel("black-forest-labs/FLUX.1-schnell", "SiliconFlow", "硅基流动 FLUX.1 Schnell 图像生成模型，适合快速出图。", "SiliconCloud.Color", "图像,开源", []string{"image-generation"}),
		domesticModel("InstantX/InstantID", "SiliconFlow", "硅基流动 InstantID 图像模型，适合人物身份保持场景。", "SiliconCloud.Color", "图像,开源", []string{"image-generation"}),

		domesticModel("kling-v1", "快手可灵", "可灵 v1 视频生成模型，适合文生视频和图生视频。", "Kling.Color", "国产,视频", []string{"openai-video"}),
		domesticModel("kling-v1-6", "快手可灵", "可灵 v1.6 视频生成模型，适合更高质量的图生视频。", "Kling.Color", "国产,视频", []string{"openai-video"}),
		domesticModel("kling-v2-master", "快手可灵", "可灵 v2 Master 视频生成模型，适合高质量视频创作。", "Kling.Color", "国产,视频", []string{"openai-video"}),

		domesticModel("jimeng_high_aes_general_v21_L", "即梦", "即梦高清图像生成模型。", "Jimeng.Color", "国产,图像", []string{"image-generation"}),
		domesticModel("jimeng_vgfm_t2v_l20", "即梦", "即梦视频生成模型。", "Jimeng.Color", "国产,视频,文生视频", []string{"openai-video"}),

		domesticModel("viduq2", "Vidu", "Vidu Q2 视频生成模型，适合参考图和高质量视频生成。", "Vidu", "国产,视频", []string{"openai-video"}),
		domesticModel("viduq1", "Vidu", "Vidu Q1 视频生成模型，适合基础文生视频和图生视频。", "Vidu", "国产,视频", []string{"openai-video"}),
		domesticModel("vidu2.0", "Vidu", "Vidu 2.0 视频生成模型。", "Vidu", "国产,视频", []string{"openai-video"}),
		domesticModel("vidu1.5", "Vidu", "Vidu 1.5 视频生成模型。", "Vidu", "国产,视频", []string{"openai-video"}),
	}

	return vendors, models
}

func domesticModel(modelName string, vendorName string, description string, icon string, tags string, endpoints []string) upstreamModel {
	rawEndpoints, _ := json.Marshal(domesticEndpointMap(endpoints))
	return upstreamModel{
		Description: description,
		Endpoints:   rawEndpoints,
		Icon:        icon,
		ModelName:   modelName,
		NameRule:    0,
		Status:      1,
		Tags:        tags,
		VendorName:  vendorName,
	}
}

func domesticEndpointMap(endpoints []string) map[string]common.EndpointInfo {
	result := make(map[string]common.EndpointInfo, len(endpoints))
	for _, endpoint := range endpoints {
		endpointType := constant.EndpointType(endpoint)
		if info, ok := common.GetDefaultEndpointInfo(endpointType); ok {
			result[endpoint] = info
			continue
		}
		result[endpoint] = common.EndpointInfo{Method: httpMethodPost}
	}
	return result
}

const httpMethodPost = "POST"
